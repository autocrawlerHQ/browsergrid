import {
    SafetyPlugin,
    SafetyContext,
    SafetyCheck,
    SafetyCheckCode,
    Action,
    ActionType
} from '../../types';


export class PromptInjectionPlugin implements SafetyPlugin {
    id = 'prompt_injection';
    name = 'Prompt Injection Detector';
    description = 'Detects and blocks potential prompt injection attacks';

    private sensitivePatterns: RegExp[] = [
        /ignore previous instructions/i,
        /disregard (all|your) instructions/i,
        /forget your previous instructions/i,
        /new instructions/i,
        /you (are|will) (now|instead)/i,
        /you must/i,
        /follow these instructions instead/i,
        /please (ignore|disregard)/i,
        /don't (follow|listen to)/i,
    ];

    async initialize(config: any): Promise<void> {
        if (config?.sensitivePatterns) {
            this.sensitivePatterns = config.sensitivePatterns.map((pattern: string) => new RegExp(pattern, 'i'));
        }
    }

    async checkAction(action: Action, context: SafetyContext): Promise<SafetyCheck[]> {
        if (action.type === ActionType.TYPE) {
            const typeAction = action as any;
            const text = typeAction.text;

            for (const pattern of this.sensitivePatterns) {
                if (pattern.test(text)) {
                    return [{
                        id: `prompt_injection_${Date.now()}`,
                        code: SafetyCheckCode.MALICIOUS_INSTRUCTIONS,
                        message: 'Potential prompt injection detected in text input. Please review and confirm if this text should be allowed.'
                    }];
                }
            }
        }

        return [];
    }
}