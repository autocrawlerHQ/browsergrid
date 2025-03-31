/**
 * Controller for managing the agent loop with enhanced functionality for streaming,
 * async operations, and human-in-the-loop interactions
 */

import {
    AIAgent,
    ComputerEnvironment,
    AgentConfig,
    Action,
    SafetyCheck,
    ActionType
} from '../types';
import { SafetyManager } from '../safety/safety-manager';
import { configureSafety } from '../safety/safety-manager';

/**
 * Options for the agent controller
 */
export interface AgentControllerOptions {
    /**
     * Maximum number of iterations
     */
    maxIterations?: number;

    /**
     * Maximum time (in ms) to wait for a safety check acknowledgement
     */
    safetyCheckTimeoutMs?: number;

    /**
     * Pending safety check promises
     */
    pendingSafetyPromises?: Map<string, {
        resolve: (checks: SafetyCheck[]) => void;
        reject: (error: Error) => void;
        checks: SafetyCheck[];
    }>;

    /**
     * Callback for when an action is executed
     */
    onAction?: (action: Action) => void;

    /**
     * Callback for when reasoning is available
     */
    onReasoning?: (reasoning: string) => void;

    /**
     * Callback for handling safety checks
     * Returns acknowledged checks to proceed with
     */
    onSafetyCheck?: (checks: SafetyCheck[]) => Promise<SafetyCheck[]>;

    /**
     * Callback for when the agent loop completes
     */
    onComplete?: () => void;

    /**
     * Callback for when an error occurs
     */
    onError?: (error: Error) => void;

    /**
     * Callback for when a screenshot is taken
     */
    onScreenshot?: (screenshot: Buffer) => void;

    /**
     * Callback for when a text response is generated
     */
    onTextResponse?: (text: string) => void;

    /**
     * Callback for processing safety checks
     */
    processSafetyChecks?: (checks: SafetyCheck[]) => Promise<SafetyCheck[]>;
    
    
}

/**
 * Controller for managing the agent loop
 */
export class AgentController {
    // todo: make these private
    public agent: AIAgent;
    public environment: ComputerEnvironment;


    private config: AgentConfig;
    private safetyManager: SafetyManager;
    private isRunning: boolean = false;
    private maxIterations: number;
    private safetyCheckTimeoutMs: number;
    public onActionCallback?: (action: Action) => void;
    public onReasoningCallback?: (reasoning: string) => void;
    public onSafetyCheckCallback?: (checks: SafetyCheck[]) => Promise<SafetyCheck[]>;
    public onCompleteCallback?: () => void;
    public onErrorCallback?: (error: Error) => void;
    public onScreenshotCallback?: (screenshot: Buffer) => void;
    public onTextResponseCallback?: (text: string) => void;
    private previousActions: Action[] = [];
    private abortController: AbortController;

    /**
     * Create a new agent controller
     */
    constructor(
        agent: AIAgent,
        environment: ComputerEnvironment,
        config: AgentConfig,
        options: AgentControllerOptions = {}
    ) {
        this.agent = agent;
        this.environment = environment;
        this.config = config;
        this.maxIterations = options.maxIterations || 20;
        this.safetyCheckTimeoutMs = options.safetyCheckTimeoutMs || 5 * 60 * 1000; // 5 minutes
        this.onActionCallback = options.onAction;
        this.onReasoningCallback = options.onReasoning;
        this.onSafetyCheckCallback = options.onSafetyCheck;
        this.onCompleteCallback = options.onComplete;
        this.onErrorCallback = options.onError;
        this.onScreenshotCallback = options.onScreenshot;
        this.onTextResponseCallback = options.onTextResponse;
        this.abortController = new AbortController();

        // Initialize safety manager
        this.safetyManager = configureSafety(new SafetyManager(), {
            allowlist: config.allowlist,
            blocklist: config.blocklist,
            requireConfirmation: config.requireHumanConfirmation
        });
    }

    /**
     * Start the agent loop with a user instruction
     */
    async start(userInstruction: string): Promise<void> {
        if (this.isRunning) {
            throw new Error('Agent loop is already running');
        }

        this.isRunning = true;
        this.previousActions = [];
        this.abortController = new AbortController();

        // Ensure onSafetyCheck callback uses our handler
        this.onSafetyCheckCallback = this.handleSafetyChecks.bind(this);

        try {
            // Initialize the agent and environment
            await this.agent.initialize(this.config);
            await this.environment.initialize({
                width: this.config.displayWidth,
                height: this.config.displayHeight
            });

            // Take initial screenshot
            const initialScreenshot = await this.environment.takeScreenshot();

            // Emit the screenshot if callback is provided
            if (this.onScreenshotCallback) {
                this.onScreenshotCallback(initialScreenshot);
            }

            // Send initial request to the AI model
            let response = await this.agent.sendInitialRequest(userInstruction, initialScreenshot);

            // If there's a text response, emit it
            if (response.textResponse && this.onTextResponseCallback) {
                this.onTextResponseCallback(response.textResponse);
            }

            // Main agent loop
            let iterations = 0;
            while (response.action && iterations < this.maxIterations && this.isRunning) {
                iterations++;

                // Check if the operation was aborted
                if (this.abortController.signal.aborted) {
                    throw new Error('Operation aborted by user');
                }

                // Check if there's an action to execute
                if (response.action) {
                    // Call the onAction callback if provided
                    if (this.onActionCallback) {
                        this.onActionCallback(response.action);
                    }

                    // Add action to previous actions
                    this.previousActions.push(response.action);

                    // Call the onReasoning callback if provided
                    if (this.onReasoningCallback && response.reasoning) {
                        this.onReasoningCallback(response.reasoning);
                    }

                    // Handle safety checks
                    let safetyContext = {
                        currentUrl: await this.environment.getCurrentUrl?.(),
                        environmentType: this.config.environment,
                        previousActions: this.previousActions,
                        userInstruction,
                    };

                    // Perform safety checks
                    const safetyChecks = await this.safetyManager.checkAction(response.action, safetyContext);

                    // Combine with any pending safety checks from the model
                    const allSafetyChecks = [...safetyChecks, ...response.pendingSafetyChecks];

                    // Process safety checks if any
                    let acknowledgedSafetyChecks: SafetyCheck[] = [];
                    if (allSafetyChecks.length > 0) {
                        try {
                            if (this.onSafetyCheckCallback) {
                                // Use the callback to get acknowledged checks
                                const timeoutPromise = new Promise<SafetyCheck[]>((_, reject) => {
                                    setTimeout(() => reject(new Error('Safety check acknowledgement timeout')), this.safetyCheckTimeoutMs);
                                });

                                // Race the callback against the timeout
                                acknowledgedSafetyChecks = await Promise.race([
                                    this.onSafetyCheckCallback(allSafetyChecks),
                                    timeoutPromise
                                ]);
                            } else if (this.config.requireHumanConfirmation) {
                                // Default behavior is to not acknowledge any checks if human confirmation is required
                                console.log('Safety checks require confirmation but no callback provided');
                                break;
                            } else {
                                // Default behavior is to acknowledge all checks if human confirmation is not required
                                acknowledgedSafetyChecks = allSafetyChecks;
                            }
                        } catch (error) {
                            // If there's an error with safety check acknowledgement, abort the agent loop
                            throw error;
                        }
                    }

                    // Execute the action in the environment
                    if (response.action.type !== ActionType.SCREENSHOT) {
                        await this.environment.executeAction(response.action);
                    }

                    // Take a screenshot after executing the action
                    const screenshot = await this.environment.takeScreenshot();

                    // Emit the screenshot if callback is provided
                    if (this.onScreenshotCallback) {
                        this.onScreenshotCallback(screenshot);
                    }

                    // Get the current URL if available
                    let currentUrl: string | undefined;
                    if (this.environment.getCurrentUrl) {
                        currentUrl = await this.environment.getCurrentUrl();
                    }

                    // Send the screenshot back to the AI model
                    if (response.callId) {
                        response = await this.agent.sendActionResult(
                            screenshot,
                            response.id,
                            response.callId,
                            acknowledgedSafetyChecks,
                            currentUrl
                        );

                        // If there's a text response, emit it
                        if (response.textResponse && this.onTextResponseCallback) {
                            this.onTextResponseCallback(response.textResponse);
                        }
                    } else {
                        // If there's no callId, break out of the loop
                        break;
                    }
                } else {
                    // No action to execute, break out of the loop
                    break;
                }
            }

            // Call the onComplete callback if provided
            if (this.onCompleteCallback) {
                this.onCompleteCallback();
            }
        } catch (error) {
            console.error('Error in agent loop:', error);

            // Call the onError callback if provided
            if (this.onErrorCallback && error instanceof Error) {
                this.onErrorCallback(error);
            }

            throw error;
        } finally {
            // Clean up resources
            this.isRunning = false;
        }
    }

    /**
     * Stop the agent loop
     */
    stop(): void {
        if (this.isRunning) {
            this.isRunning = false;
            this.abortController.abort();
        }
    }

    /**
     * Check if the agent loop is running
     */
    isAgentRunning(): boolean {
        return this.isRunning;
    }

    /**
     * Get the previous actions
     */
    getPreviousActions(): Action[] {
        return [...this.previousActions];
    }

    public pendingSafetyPromises: Map<string, {
        resolve: (checks: SafetyCheck[]) => void;
        reject: (error: Error) => void;
        checks: SafetyCheck[];
    }> = new Map();

    /**
     * Process acknowledged safety checks to continue execution
     */
    processSafetyChecks(acknowledgedChecks: SafetyCheck[]): void {
        // Get the check IDs
        const checkIds = acknowledgedChecks.map(check => check.id);

        // Find matching promises
        for (const [key, { resolve, checks }] of this.pendingSafetyPromises.entries()) {
            const promiseCheckIds = checks.map(check => check.id);

            // Check if all required checks are acknowledged
            const allAcknowledged = promiseCheckIds.every(id => checkIds.includes(id));

            if (allAcknowledged) {
                // Resolve the promise with the acknowledged checks
                resolve(acknowledgedChecks);
                this.pendingSafetyPromises.delete(key);
                return;
            }
        }
    }

    /**
     * Override the default safety check callback
     */
    async handleSafetyChecks(checks: SafetyCheck[]): Promise<SafetyCheck[]> {
        if (checks.length === 0) return [];

        // If we don't require human confirmation, return all checks
        if (!this.config.requireHumanConfirmation) {
            return checks;
        }

        // Create a promise that will be resolved when checks are acknowledged
        return new Promise<SafetyCheck[]>((resolve, reject) => {
            const checkIds = checks.map(check => check.id);
            const key = checkIds.join(',');

            // Store the promise
            this.pendingSafetyPromises.set(key, { resolve, reject, checks });

            // Set a timeout
            setTimeout(() => {
                if (this.pendingSafetyPromises.has(key)) {
                    this.pendingSafetyPromises.get(key)!.reject(
                        new Error('Safety check confirmation timeout')
                    );
                    this.pendingSafetyPromises.delete(key);
                }
            }, this.safetyCheckTimeoutMs);
        });
    }



    /**
     * Cleanup resources
     */
    async cleanup(): Promise<void> {
        this.stop();
        await this.environment.cleanup();
    }
}