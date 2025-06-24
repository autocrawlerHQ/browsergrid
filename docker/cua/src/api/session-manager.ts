import { 
  Session, 
  AgentConfig, 
  SafetyCheck, 
  Action, 
  EnvironmentType,
  AIResponse,
  ComputerEnvironment 
} from '../types';
import { BrowserEnvironment } from '../environments/browser';
import { AgentController } from '../controllers/controller';
import { AIAgentFactory } from '../agents/factory';
import { SessionStore } from './session-store';

export class SessionManager {
  private activeControllers: Map<string, AgentController> = new Map();
  
  constructor(private sessionStore: SessionStore) {}
  
  /**
   * Create a new session
   */
  async createSession(
    config: AgentConfig, 
    environmentOptions: any
  ): Promise<Session> {
    const sessionId = this.sessionStore.generateSessionId();
    
    const session: Session = {
      id: sessionId,
      createdAt: new Date(),
      lastActiveAt: new Date(),
      agentConfig: config,
      environmentType: config.environment,
      environmentOptions,
      status: 'idle',
      currentState: {
        pendingSafetyChecks: [],
        previousActions: [],
      },
      history: {
        userInstructions: [],
        agentResponses: [],
        safetyEvents: [],
      },
      metadata: {}
    };
    
    // Store in SQLite
    await this.sessionStore.createSession(session);
    
    return session;
  }
  
  /**
   * Get a session by ID
   */
  async getSession(sessionId: string): Promise<Session | null> {
    return this.sessionStore.getSession(sessionId);
  }
  
  /**
   * Update a session
   */
  async updateSession(sessionId: string, updates: Partial<Session>): Promise<Session> {
    await this.sessionStore.updateSession(sessionId, updates);
    const updatedSession = await this.sessionStore.getSession(sessionId);
    if (!updatedSession) {
      throw new Error(`Session not found after update: ${sessionId}`);
    }
    return updatedSession;
  }
  
  /**
   * Store screenshot for a session
   */
  async storeScreenshot(sessionId: string, screenshot: Buffer): Promise<string> {
    return this.sessionStore.storeScreenshot(sessionId, screenshot);
  }
  
  /**
   * Get a screenshot
   */
  async getScreenshot(path: string): Promise<Buffer> {
    return this.sessionStore.getScreenshot(path);
  }
  
  /**
   * Create or get an agent controller for a session
   */
  async getAgentController(sessionId: string): Promise<AgentController> {
    // Return existing controller if available
    if (this.activeControllers.has(sessionId)) {
      return this.activeControllers.get(sessionId)!;
    }
    
    // Get session
    const session = await this.getSession(sessionId);
    if (!session) {
      throw new Error(`Session not found: ${sessionId}`);
    }
    
    // Create environment
    const environment = this.createEnvironment(
      session.environmentType, 
      session.environmentOptions
    );
    
    // Create agent
    const agent = AIAgentFactory.createAgent(session.agentConfig.provider);
    
    // Initialize agent
    await agent.initialize(session.agentConfig);
    
    // Initialize environment
    await environment.initialize({
      width: session.agentConfig.displayWidth,
      height: session.agentConfig.displayHeight,
      ...session.environmentOptions
    });
    
    // Create controller
    const controller = new AgentController(
      agent,
      environment,
      session.agentConfig,
      {
        maxIterations: session.agentConfig.maxIterations || 10,
        onAction: async (action) => {
          // Update session state with new action
          await this.appendActionToSession(sessionId, action);
        },
        onReasoning: async (reasoning) => {
          // Store reasoning
          await this.updateSessionMetadata(sessionId, 'latestReasoning', reasoning);
        },
        onSafetyCheck: async (checks) => {
          // Store safety checks
          await this.updateSessionSafetyChecks(sessionId, checks);
          
          // If human confirmation not required, return all checks
          if (!session.agentConfig.requireHumanConfirmation) {
            return checks;
          }
          
          // Otherwise return empty array - confirmations will come separately
          return [];
        }
      }
    );
    
    // Store controller
    this.activeControllers.set(sessionId, controller);
    
    return controller;
  }
  
  /**
   * Process safety checks for a session
   */
  async processSafetyChecks(sessionId: string, checkedIds: string[]): Promise<boolean> {
    if (!this.activeControllers.has(sessionId)) {
      return false;
    }
    
    const controller = this.activeControllers.get(sessionId)!;
    if (typeof controller.processSafetyChecks === 'function') {
      // Get the session to get the actual safety check objects
      const session = await this.getSession(sessionId);
      if (!session) return false;
      
      // Find the checks that match the IDs
      const checks = session.currentState.pendingSafetyChecks.filter(
        check => checkedIds.includes(check.id)
      );
      
      // Process them in the controller
      controller.processSafetyChecks(checks);
      return true;
    }
    
    return false;
  }
  
  /**
   * Close a session
   */
  async closeSession(sessionId: string): Promise<void> {
    // Stop controller if running
    if (this.activeControllers.has(sessionId)) {
      const controller = this.activeControllers.get(sessionId)!;
      controller.stop();
      
      // Clean up resources
      try {
        await controller.cleanup();
      } catch (error) {
        console.error(`Error cleaning up environment for session ${sessionId}:`, error);
      }
      
      this.activeControllers.delete(sessionId);
    }
    
    // Mark session as closed in database
    await this.updateSession(sessionId, { status: 'completed' });
  }
  
  /**
   * Delete a session completely
   */
  async deleteSession(sessionId: string): Promise<void> {
    // Close session first to clean up resources
    await this.closeSession(sessionId);
    
    // Delete from database
    await this.sessionStore.deleteSession(sessionId);
  }
  
  /**
   * Update session with new action
   */
  private async appendActionToSession(sessionId: string, action: Action): Promise<void> {
    const session = await this.getSession(sessionId);
    if (!session) return;
    
    // Add to previous actions
    session.currentState.previousActions.push(action);
    
    // Update session
    await this.updateSession(sessionId, {
      currentState: session.currentState
    });
  }
  
  /**
   * Update session metadata
   */
  private async updateSessionMetadata(
    sessionId: string, 
    key: string, 
    value: any
  ): Promise<void> {
    const session = await this.getSession(sessionId);
    if (!session) return;
    
    // Update metadata
    session.metadata[key] = value;
    
    // Update session
    await this.updateSession(sessionId, {
      metadata: session.metadata
    });
  }
  
  /**
   * Update session safety checks
   */
  private async updateSessionSafetyChecks(
    sessionId: string, 
    checks: SafetyCheck[]
  ): Promise<void> {
    const session = await this.getSession(sessionId);
    if (!session) return;
    
    // Update pending safety checks
    session.currentState.pendingSafetyChecks = checks;
    
    // Add to safety events history
    session.history.safetyEvents.push({
      checks,
      acknowledged: false,
      timestamp: new Date()
    });
    
    // Update session
    await this.updateSession(sessionId, {
      currentState: session.currentState,
      history: session.history
    });
  }
  
  /**
   * Create environment based on type
   */
  private createEnvironment(
    type: EnvironmentType, 
    options: any
  ): ComputerEnvironment {
    switch (type) {
      case EnvironmentType.BROWSER:
        return new BrowserEnvironment();
      // Add other environment types as needed
      default:
        throw new Error(`Unsupported environment type: ${type}`);
    }
  }
  
}