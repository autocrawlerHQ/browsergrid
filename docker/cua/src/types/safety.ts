import { Action } from './actions';
import { EnvironmentType } from './agent';


export enum SafetyCheckCode {
  MALICIOUS_INSTRUCTIONS = 'malicious_instructions',
  IRRELEVANT_DOMAIN = 'irrelevant_domain',
  SENSITIVE_DOMAIN = 'sensitive_domain',
}


export interface SafetyCheck {
  id: string;
  code: SafetyCheckCode;
  message: string;
}


export enum SafetyPolicyLevel {
  MINIMAL = 'minimal',    // Basic safeguards only
  STANDARD = 'standard',  // Default protection level
  STRICT = 'strict',      // Maximum protection
  CUSTOM = 'custom'       // Custom configuration
}


export interface SafetyContext {
  /**
   * Current URL (for browser environments)
   */
  currentUrl?: string;
  
  /**
   * Environment type
   */
  environmentType: EnvironmentType;
  
  /**
   * Previous actions in the current session
   */
  previousActions: Action[];
  
  /**
   * User instruction that started this session
   */
  userInstruction: string;
  
  /**
   * Screenshot data (if available)
   */
  screenshotData?: Buffer;
}


export interface SafetyPlugin {
  /**
   * Unique identifier for the plugin
   */
  id: string;
  
  /**
   * Human-readable name of the plugin
   */
  name: string;
  
  /**
   * Description of what the plugin does
   */
  description: string;
  
  /**
   * Check an action for safety concerns
   * @returns Array of safety checks if issues found, empty array if safe
   */
  checkAction(action: Action, context: SafetyContext): Promise<SafetyCheck[]>;
  
  /**
   * Initialize the plugin with configuration
   */
  initialize(config: any): Promise<void>;
}


export interface SafetyOptions {
  policyLevel?: SafetyPolicyLevel;
  allowlist?: string[];
  blocklist?: string[];
  customPlugins?: SafetyPlugin[];
  requireConfirmation?: boolean;
}