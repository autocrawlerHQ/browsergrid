import { Action } from './actions';
import { SafetyCheck } from './safety';


export enum AIProvider {
    ANTHROPIC = 'anthropic',
    OPENAI = 'openai',
}


export enum EnvironmentType {
    BROWSER = 'browser',
    MACOS = 'macos',
    WINDOWS = 'windows',
    LINUX = 'linux',
}



export interface AIResponse {
    id: string;
    action?: Action;
    textResponse?: string;
    pendingSafetyChecks: SafetyCheck[];
    reasoning?: string;
    callId?: string; // For tracking tool calls
}


export interface AgentConfig {
    provider: AIProvider;
    apiKey: string;
    modelName: string;
    displayWidth: number;
    displayHeight: number;
    environment: EnvironmentType;
    maxTokens?: number;
    enableThinking?: boolean;
    thinkingBudget?: number;
    allowlist?: string[];
    blocklist?: string[];
    requireHumanConfirmation?: boolean;
    maxIterations?: number;
    safetyCheckTimeoutMs?: number;
}


export interface AIAgent {
    /**
     * Initialize the agent with configuration
     */
    initialize(config: AgentConfig): Promise<void>;

    /**
     * Send a request to the AI model with user instructions
     */
    sendInitialRequest(userInstruction: string, initialScreenshot?: Buffer): Promise<AIResponse>;

    /**
     * Send screenshot result back to the AI model
     */
    sendActionResult(
        screenshot: Buffer,
        previousResponseId: string,
        callId: string,
        acknowledgedSafetyChecks?: SafetyCheck[],
        currentUrl?: string
    ): Promise<AIResponse>;

    /**
     * Get the reasoning behind the AI's decision (if supported)
     */
    getReasoning(): string | undefined;
}