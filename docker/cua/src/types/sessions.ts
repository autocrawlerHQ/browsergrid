import { Action,  } from "./actions";
import { AgentConfig, EnvironmentType } from "./agent";
import { SafetyCheck } from "./safety";

export interface Session {
    id: string;                      // Unique session identifier
    createdAt: Date;                 // Creation timestamp
    lastActiveAt: Date;              // Last activity timestamp
    agentConfig: AgentConfig;        // Agent configuration
    environmentType: EnvironmentType; // Type of environment
    environmentOptions: any;         // Environment-specific options
    status: 'idle' | 'running' | 'paused' | 'error' | 'completed'; // Session status
    
    // Current state
    currentState: {
      currentUrl?: string;           // Current URL if applicable
      screenshotPath?: string;       // Path to last screenshot
      pendingSafetyChecks: SafetyCheck[]; // Pending safety checks
      previousActions: Action[];     // History of actions
      currentInstruction?: string;   // Current user instruction being processed
    };
    
    // Conversation history
    history: {
      userInstructions: Array<{     // User instructions history
        text: string;
        timestamp: Date;
      }>;
      agentResponses: Array<{       // Agent responses history
        text?: string;
        actions: Action[];
        timestamp: Date;
        reasoning?: string;
      }>;
      safetyEvents: Array<{         // Safety-related events
        checks: SafetyCheck[];
        acknowledged: boolean;
        timestamp: Date;
      }>;
    };
    
    // Session metadata
    metadata: Record<string, any>;   // Additional metadata
    
    // Expiration
    expiresAt?: Date;                // When session will expire
  }