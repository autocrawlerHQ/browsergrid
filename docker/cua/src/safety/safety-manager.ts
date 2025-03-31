import { 
    SafetyPlugin, 
    SafetyContext, 
    SafetyCheck, 
    SafetyCheckCode,
    SafetyPolicyLevel,
    SafetyOptions
  } from '../types';
  import { extractDomain } from './utils/domain';
  
  export class SafetyManager {
    private plugins: SafetyPlugin[] = [];
    private allowlist: string[] = [];
    private blocklist: string[] = [];
    private requireConfirmation: boolean = true;
    
    registerPlugin(plugin: SafetyPlugin, config?: any): void {
      this.plugins.push(plugin);
      if (config) {
        plugin.initialize(config);
      }
    }
    
    setAllowlist(domains: string[]): void {
      this.allowlist = domains;
    }
    
    setBlocklist(domains: string[]): void {
      this.blocklist = domains;
    }
    
    setRequireConfirmation(require: boolean): void {
      this.requireConfirmation = require;
    }
    
    async checkAction(action: any, context: SafetyContext): Promise<SafetyCheck[]> {
      if (context.currentUrl) {
        const domain = extractDomain(context.currentUrl);
        
        if (this.blocklist.some(blockedDomain => domain.includes(blockedDomain))) {
          return [{
            id: `domain_blocked_${Date.now()}`,
            code: SafetyCheckCode.IRRELEVANT_DOMAIN,
            message: `Domain ${domain} is blocked by policy. If you need to access this domain, update your blocklist settings.`
          }];
        }
        
        if (this.allowlist.length > 0 && !this.allowlist.some(allowedDomain => domain.includes(allowedDomain))) {
          return [{
            id: `domain_not_allowed_${Date.now()}`,
            code: SafetyCheckCode.IRRELEVANT_DOMAIN,
            message: `Domain ${domain} is not on the allowlist. If you need to access this domain, update your allowlist settings.`
          }];
        }
      }
      
      const allChecks: SafetyCheck[] = [];
      
      for (const plugin of this.plugins) {
        const checks = await plugin.checkAction(action, context);
        allChecks.push(...checks);
      }
      
      return allChecks;
    }
    
    configurePolicyLevel(level: SafetyPolicyLevel): void {
      switch (level) {
        case SafetyPolicyLevel.MINIMAL:
          this.requireConfirmation = false;
          this.blocklist = [];
          this.allowlist = [];
          break;
          
        case SafetyPolicyLevel.STANDARD:
          this.requireConfirmation = true;
          this.blocklist = [
            'login', 'signin', 'auth', 'account', 'password',
            'banking', 'bank', 'finance', 'payment', 'paypal',
            'wallet', 'crypto', 'admin', 'dashboard'
          ];
          this.allowlist = [];
          break;
          
        case SafetyPolicyLevel.STRICT:
          this.requireConfirmation = true;
          this.blocklist = [
            'login', 'signin', 'auth', 'account', 'password',
            'banking', 'bank', 'finance', 'payment', 'paypal',
            'wallet', 'crypto', 'admin', 'dashboard',
            'email', 'mail', 'webmail', 'gmail', 'yahoo',
            'facebook', 'twitter', 'linkedin', 'instagram',
            'github', 'gitlab', 'aws', 'azure', 'cloud',
            'drive', 'dropbox', 'box', 'storage'
          ];
          this.allowlist = [];
          break;
      }
    }
  }

  export function configureSafety(manager: SafetyManager, options: SafetyOptions): SafetyManager {
    manager.configurePolicyLevel(options.policyLevel || SafetyPolicyLevel.STANDARD);
    
    manager.setAllowlist(options.allowlist || []);
    manager.setBlocklist(options.blocklist || []);
    
    manager.setRequireConfirmation(options.requireConfirmation || true);
    
    if (options.customPlugins) {
      for (const plugin of options.customPlugins) {
        manager.registerPlugin(plugin);
      }
    }

    return manager;
  }
