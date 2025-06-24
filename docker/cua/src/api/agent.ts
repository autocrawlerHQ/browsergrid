import { Request, Response, Router } from 'express';
import { SessionManager } from './session-manager';
import { SessionStore } from './session-store';
import path from 'path';
import fs from 'fs';
import { SafetyCheck } from '../types';
import { Action } from '../types/actions';
import { AgentConfig, EnvironmentType } from '../types/agent';
import { BrowserEnvironmentOptions } from '../environments/browser';

// Type definitions for request bodies
/**
 * Request body for agent action endpoints (sync and stream)
 */
interface AgentActionRequestBody {
  /** User instruction to be executed by the agent */
  instruction: string;
  /** Existing session ID (if continuing a session) */
  sessionId?: string;
  /** Agent configuration (required for new sessions) */
  config?: AgentConfig;
  /** Environment-specific options */
  environmentOptions?: BrowserEnvironmentOptions
  /** Timeout in milliseconds before the operation is aborted */
  timeout?: number;
}

/**
 * Request body for safety confirmation endpoint
 */
interface SafetyConfirmRequestBody {
  /** Session ID to process safety checks for */
  sessionId: string;
  /** Array of safety check IDs that have been acknowledged by the user */
  acknowledgedChecks: string[];
}

// Setup paths
const DATA_DIR = path.join(process.cwd(), 'data');
const DB_PATH = path.join(DATA_DIR, 'sessions.db');
const SCREENSHOTS_DIR = path.join(DATA_DIR, 'screenshots');

// Make sure data directory exists
if (!fs.existsSync(DATA_DIR)) {
  fs.mkdirSync(DATA_DIR, { recursive: true });
}

// Initialize the session store and manager
const sessionStore = new SessionStore(DB_PATH, SCREENSHOTS_DIR);
let sessionManager: SessionManager;

// Initialize the store and create the session manager
const initialize = async () => {
  await sessionStore.initialize();
  sessionManager = new SessionManager(sessionStore);
};

// Initialize on module load
initialize().catch(error => {
  console.error('Failed to initialize session store:', error);
  process.exit(1);
});

/**
 * Create and configure the API router
 */
const router = Router();


router.post('/agent/sync', async (req: Request<{}, {}, AgentActionRequestBody>, res: Response): Promise<any> => {
  try {
    const {
      instruction,
      sessionId,
      config,
      environmentOptions,
      timeout = 60000  // Default 1 minute timeout
    } = req.body;

    // Validate request
    if (!instruction) {
      return res.status(400).json({ error: 'Instruction is required' });
    }

    // Get or create session
    let session;
    if (sessionId) {
      session = await sessionManager.getSession(sessionId);
      if (!session) {
        return res.status(404).json({ error: 'Session not found' });
      }
    } else {
      if (!config) {
        return res.status(400).json({ error: 'New sessions require config' });
      }
      session = await sessionManager.createSession(config, environmentOptions);
    }

    // Check if session is already running
    if (session.status === 'running') {
      return res.status(409).json({
        error: 'Session is already running',
        sessionId: session.id
      });
    }

    // Update session status
    await sessionManager.updateSession(session.id, {
      status: 'running',
      currentState: {
        ...session.currentState,
        currentInstruction: instruction
      }
    });

    // Add instruction to history
    session.history.userInstructions.push({
      text: instruction,
      timestamp: new Date()
    });

    // Get controller
    const controller = await sessionManager.getAgentController(session.id);

    // Set timeout
    const timeoutId = setTimeout(() => {
      controller.stop();
    }, timeout);

    try {
      // Execute agent task
      await controller.start(instruction);

      // Take final screenshot
      const screenshot = await controller.environment.takeScreenshot();
      const screenshotPath = await sessionManager.storeScreenshot(
        session.id,
        screenshot
      );

      // Get current URL if possible
      let currentUrl;
      if (controller.environment.getCurrentUrl) {
        currentUrl = await controller.environment.getCurrentUrl();
      }

      // Get previous actions
      const actions = controller.getPreviousActions();

      // Update session with completed status
      await sessionManager.updateSession(session.id, {
        status: 'completed',
        currentState: {
          ...session.currentState,
          screenshotPath,
          currentUrl,
          previousActions: actions
        }
      });

      // Add response to history
      session.history.agentResponses.push({
        actions,
        timestamp: new Date(),
        reasoning: session.metadata.latestReasoning
      });

      // Clear timeout
      clearTimeout(timeoutId);

      // Return success response
      return res.json({
        sessionId: session.id,
        completed: true,
        response: "Task completed successfully",
        actions,
        screenshot: screenshot.toString('base64'),
        currentUrl,
        pendingSafetyChecks: session.currentState.pendingSafetyChecks
      });
    } catch (error: any) {
      // Clear timeout
      clearTimeout(timeoutId);

      // Update session with error status
      await sessionManager.updateSession(session.id, {
        status: 'error',
        metadata: {
          ...session.metadata,
          error: error.message
        }
      });

      // Return error response
      return res.status(500).json({
        sessionId: session.id,
        error: error.message,
        completed: false
      });
    }
  } catch (error) {
    console.error('Error in sync endpoint:', error);
    return res.status(500).json({ error: 'Internal server error' });
  }
});


router.post('/agent/stream', async (req: Request<{}, {}, AgentActionRequestBody>, res: Response): Promise<any> => {
  try {
    const {
      instruction,
      sessionId,
      config,
      environmentOptions,
      timeout = 300000  // Default 5 minute timeout for streaming
    } = req.body;

    // Set up SSE headers
    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');

    // Helper to send SSE events
    const sendEvent = (event: string, data: any) => {
      res.write(`event: ${event}\n`);
      res.write(`data: ${JSON.stringify(data)}\n\n`);
    };

    // Validate request
    if (!instruction) {
      sendEvent('error', { error: 'Instruction is required' });
      return res.end();
    }

    // Get or create session
    let session;
    if (sessionId) {
      session = await sessionManager.getSession(sessionId);
      if (!session) {
        sendEvent('error', { error: 'Session not found' });
        return res.end();
      }
    } else {
      if (!config) {
        sendEvent('error', { error: 'New sessions require config' });
        return res.end();
      }
      session = await sessionManager.createSession(config, environmentOptions);
      sendEvent('session_created', { sessionId: session.id });
    }

    // Check if session is already running
    if (session.status === 'running') {
      sendEvent('error', {
        error: 'Session is already running',
        sessionId: session.id
      });
      return res.end();
    }

    // Update session status
    await sessionManager.updateSession(session.id, {
      status: 'running',
      currentState: {
        ...session.currentState,
        currentInstruction: instruction
      }
    });

    // Add instruction to history
    session.history.userInstructions.push({
      text: instruction,
      timestamp: new Date()
    });

    // Get controller
    const controller = await sessionManager.getAgentController(session.id);

    // Override safety check handler for streaming
    controller.onSafetyCheckCallback = async (checks: SafetyCheck[]) => {
      if (checks.length === 0) return [];

      // Send checks to client
      sendEvent('safety_check', { checks });

      // Only wait for confirmation if required
      if (session.agentConfig.requireHumanConfirmation) {
        return new Promise<SafetyCheck[]>((resolve, reject) => {
          // Store the promise in controller or a context that can be accessed by the confirmation API
          (controller as any).pendingSafetyCheckPromise = { resolve, reject, checks };

          // Set timeout for safety check
          setTimeout(() => {
            if ((controller as any).pendingSafetyCheckPromise) {
              (controller as any).pendingSafetyCheckPromise.reject(
                new Error('Safety check confirmation timeout')
              );
              (controller as any).pendingSafetyCheckPromise = null;
            }
          }, 60000); // 1 minute timeout for safety checks
        });
      }

      // If confirmation not required, return all checks
      return checks;
    };

    // Override reasoning callback
    controller.onReasoningCallback = (reasoning: string) => {
      sendEvent('reasoning', { reasoning });
    };

    // Override action callback
    controller.onActionCallback = (action: Action) => {
      sendEvent('action', { action });
    };

    // Handle client disconnect
    req.on('close', () => {
      // Reject any pending safety checks
      if ((controller as any).pendingSafetyCheckPromise) {
        (controller as any).pendingSafetyCheckPromise.reject(
          new Error('Client disconnected')
        );
        (controller as any).pendingSafetyCheckPromise = null;
      }

      // Stop controller
      controller.stop();

      // Update session status
      sessionManager.updateSession(session.id, { status: 'idle' }).catch(err => {
        console.error('Error updating session after disconnect:', err);
      });
    });

    // Set timeout
    const timeoutId = setTimeout(() => {
      controller.stop();
      sendEvent('error', { error: 'Operation timed out' });
      res.end();
    }, timeout);

    try {
      // Execute agent task
      await controller.start(instruction);

      // Take final screenshot
      const screenshot = await controller.environment.takeScreenshot();
      sendEvent('screenshot', { screenshot: screenshot.toString('base64') });

      // Store screenshot
      const screenshotPath = await sessionManager.storeScreenshot(
        session.id,
        screenshot
      );

      // Get current URL if possible
      let currentUrl;
      if (controller.environment.getCurrentUrl) {
        currentUrl = await controller.environment.getCurrentUrl();
        sendEvent('url', { url: currentUrl });
      }

      // Get previous actions
      const actions = controller.getPreviousActions();

      // Update session with completed status
      await sessionManager.updateSession(session.id, {
        status: 'completed',
        currentState: {
          ...session.currentState,
          screenshotPath,
          currentUrl,
          previousActions: actions
        }
      });

      // Add response to history
      session.history.agentResponses.push({
        actions,
        timestamp: new Date(),
        reasoning: session.metadata.latestReasoning
      });

      // Send completed event
      sendEvent('completed', { completed: true });

      // Clear timeout
      clearTimeout(timeoutId);

      // End response
      res.end();
    } catch (error: any) {
      // Clear timeout
      clearTimeout(timeoutId);

      // Update session with error status
      await sessionManager.updateSession(session.id, {
        status: 'error',
        metadata: {
          ...session.metadata,
          error: error.message
        }
      });

      // Send error event
      sendEvent('error', { error: error.message });

      // End response
      res.end();
    }
  } catch (error: any) {
    console.error('Error in stream endpoint:', error);
    res.write(`event: error\n`);
    res.write(`data: ${JSON.stringify({ error: 'Internal server error' })}\n\n`);
    res.end();
  }
});

router.post('/agent/safety/confirm', async (req: Request<{}, {}, SafetyConfirmRequestBody>, res: Response): Promise<any> => {
  try {
    const { sessionId, acknowledgedChecks } = req.body;

    if (!sessionId || !acknowledgedChecks || !Array.isArray(acknowledgedChecks)) {
      return res.status(400).json({ error: 'Invalid request parameters' });
    }

    // Process safety checks
    const success = await sessionManager.processSafetyChecks(
      sessionId,
      acknowledgedChecks
    );

    if (!success) {
      return res.status(404).json({
        error: 'No active session or pending safety checks found'
      });
    }

    return res.json({ success: true });
  } catch (error) {
    console.error('Error confirming safety checks:', error);
    return res.status(500).json({ error: 'Internal server error' });
  }
});


router.get('/agent/sessions/:sessionId', async (req: Request, res: Response): Promise<any> => {
  try {
    const session = await sessionManager.getSession(req.params.sessionId);
    if (!session) {
      return res.status(404).json({ error: 'Session not found' });
    }
    return res.json(session);
  } catch (error) {
    console.error('Error getting session:', error);
    return res.status(500).json({ error: 'Internal server error' });
  }
});

router.delete('/agent/sessions/:sessionId', async (req: Request, res: Response): Promise<any> => {
  try {
    await sessionManager.deleteSession(req.params.sessionId);
    return res.json({ success: true });
  } catch (error) {
    console.error('Error deleting session:', error);
    return res.status(500).json({ error: 'Internal server error' });
  }
});

// route to serve screenshots
router.get('/agent/screenshots/:sessionId/:filename', async (req: Request, res: Response): Promise<any> => {
  try {
    const { sessionId, filename } = req.params;
    const path = `${sessionId}/${filename}`;

    const screenshot = await sessionManager.getScreenshot(path);
    res.contentType('image/png');
    return res.send(screenshot);
  } catch (error) {
    console.error('Error serving screenshot:', error);
    return res.status(404).send('Screenshot not found');
  }
});

export default router;