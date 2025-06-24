import {
    SafetyPlugin,
    SafetyContext,
    SafetyCheck,
    SafetyCheckCode,
    Action,
    ActionType
} from '../../types';

/**
 * Suspicious action detector plugin
 */
export class SuspiciousActionPlugin implements SafetyPlugin {
    id = 'suspicious_action';
    name = 'Suspicious Action Detector';
    description = 'Detects potentially harmful actions like file deletion or system commands';

    private suspiciousCommands: string[] = [
        'rm -rf', 'del', 'format', 'mkfs',
        'sudo', 'su', 'chmod', 'chown',
        'wget', 'curl', 'lynx',
        'install', 'apt', 'yum', 'brew',
        'powershell', 'cmd', 'regedit',
        'mount', 'unmount',
        'iptables', 'firewall',
        'kill', 'pkill', 'taskkill'
    ];

    async initialize(config: any): Promise<void> {
        if (config?.suspiciousCommands) {
            this.suspiciousCommands = config.suspiciousCommands;
        }
    }

    async checkAction(action: Action, context: SafetyContext): Promise<SafetyCheck[]> {
        // Check keyboard input for suspicious commands
        if (action.type === ActionType.TYPE) {
            const typeAction = action as any;
            const text = typeAction.text.toLowerCase();

            for (const command of this.suspiciousCommands) {
                if (text.includes(command.toLowerCase())) {
                    return [{
                        id: `suspicious_command_${Date.now()}`,
                        code: SafetyCheckCode.MALICIOUS_INSTRUCTIONS,
                        message: `Suspicious command detected: "${command}". This command could potentially harm your system. Please review and confirm if you want to proceed.`
                    }];
                }
            }
        }

        return [];
    }
}