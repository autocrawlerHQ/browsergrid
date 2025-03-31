import { 
    SafetyPlugin, 
    SafetyContext, 
    SafetyCheck, 
    SafetyCheckCode,
    Action,
    ActionType
  } from '../../types';
  

  export class SensitiveInfoPlugin implements SafetyPlugin {
    id = 'sensitive_info';
    name = 'Sensitive Information Protector';
    description = 'Prevents typing sensitive information like passwords, credit cards, or SSNs';
    
    private patterns: RegExp[] = [
      // Credit card pattern
      /\b(?:\d{4}[ -]?){3}\d{4}\b/,
      // SSN pattern
      /\b\d{3}[ -]?\d{2}[ -]?\d{4}\b/,
      // Password indicators
      /\bpassword\b|\bpass\b|\bpwd\b/i,
      // API keys (common formats)
      /\b[a-zA-Z0-9]{32,}\b/,
      /\b[a-zA-Z0-9-_]{39}\b/, // AWS access key
      /\bsk-[a-zA-Z0-9]{32,}\b/, // OpenAI API key format
    ];
    
    async initialize(config: any): Promise<void> {
      if (config?.patterns) {
        this.patterns = config.patterns.map((pattern: string) => new RegExp(pattern));
      }
    }
    
    async checkAction(action: Action, context: SafetyContext): Promise<SafetyCheck[]> {
      // Only check text inputs
      if (action.type === ActionType.TYPE) {
        const typeAction = action as any;
        const text = typeAction.text;
        
        // Check for sensitive patterns
        for (const pattern of this.patterns) {
          if (pattern.test(text)) {
            return [{
              id: `sensitive_info_${Date.now()}`,
              code: SafetyCheckCode.SENSITIVE_DOMAIN,
              message: 'Potential sensitive information detected in text input. To protect your security, please manually enter this information or confirm that this data is safe to share.'
            }];
          }
        }
        
        // Special check for password fields if we have URL context
        if (context.currentUrl && /password|pwd|pass/i.test(context.currentUrl) && text.length > 3) {
          return [{
            id: `password_entry_${Date.now()}`,
            code: SafetyCheckCode.SENSITIVE_DOMAIN,
            message: 'It appears you are entering a password. For security reasons, the AI agent should not enter passwords. Please enter this information manually.'
          }];
        }
      }
      
      return [];
    }
  }