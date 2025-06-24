import OpenAI from 'openai';
import { 
  AIAgent, 
  AIResponse, 
  AgentConfig, 
  Action, 
  ActionType,
  EnvironmentType,
  SafetyCheck,
  SafetyCheckCode,
  MouseButton
} from '../types';
import { Reasoning } from 'openai/resources/shared';
import { Tool } from 'openai/resources/responses/responses';

export class OpenAIAgent implements AIAgent {
  private config!: AgentConfig;
  private openaiClient: OpenAI;
  private lastReasoning?: string;
  
  constructor() {
    this.openaiClient = new OpenAI({
      apiKey: process.env.OPENAI_API_KEY || ''
    });
  }
  
  async initialize(config: AgentConfig): Promise<void> {
    this.config = config;
    this.openaiClient = new OpenAI({ apiKey: config.apiKey });
  }
  
  async sendInitialRequest(userInstruction: string, initialScreenshot?: Buffer): Promise<AIResponse> {
    const tools: Tool[] = [{
      type: "computer-preview",
      display_width: this.config.displayWidth,
      display_height: this.config.displayHeight,
      environment: this.mapEnvironmentType(this.config.environment)
    }];
    
    const input: any[] = [{
      role: "user",
      content: userInstruction
    }];
    
    if (initialScreenshot) {
      const screenshotBase64 = initialScreenshot.toString('base64');
      
      input.push({
        type: "input_image",
        image_url: `data:image/png;base64,${screenshotBase64}`
      });
    }
    
    const reasoning: Reasoning | undefined = this.config.enableThinking ? {
      generate_summary: "concise"
    } : undefined;
    
    try {
      const response = await this.openaiClient.responses.create({
        model: this.config.modelName,
        tools,
        input,
        reasoning,
        truncation: "auto"
      });
      
      return this.processOpenAIResponse(response);
    } catch (error) {
      console.error('Error making OpenAI API call:', error);
      throw new Error(`Failed to get response from OpenAI: ${error}`);
    }
  }
  
  async sendActionResult(
    screenshot: Buffer, 
    previousResponseId: string, 
    callId: string,
    acknowledgedSafetyChecks?: SafetyCheck[],
    currentUrl?: string
  ): Promise<AIResponse> {
    const screenshotBase64 = screenshot.toString('base64');
    
    const computerCallOutput: any = {
      call_id: callId,
      type: "computer_call_output",
      output: {
        type: "input_image",
        image_url: `data:image/png;base64,${screenshotBase64}`
      }
    };
    
    if (currentUrl) {
      computerCallOutput.current_url = currentUrl;
    }
    
    if (acknowledgedSafetyChecks && acknowledgedSafetyChecks.length > 0) {
      computerCallOutput.acknowledged_safety_checks = acknowledgedSafetyChecks;
    }
    
    try {
      const response = await this.openaiClient.responses.create({
        model: this.config.modelName,
        previous_response_id: previousResponseId,
        tools: [{
          type: "computer-preview",
          display_width: this.config.displayWidth,
          display_height: this.config.displayHeight,
          environment: this.mapEnvironmentType(this.config.environment)
        }],
        input: [computerCallOutput],
        truncation: "auto"
      });
      
      return this.processOpenAIResponse(response);
    } catch (error) {
      console.error('Error sending action result to OpenAI:', error);
      throw new Error(`Failed to get response from OpenAI: ${error}`);
    }
  }
  
  getReasoning(): string | undefined {
    return this.lastReasoning;
  }
  
  private processOpenAIResponse(response: any): AIResponse {
    let action: Action | undefined;
    let callId: string | undefined;
    let pendingSafetyChecks: SafetyCheck[] = [];
    
    const computerCall = response.output.find((item: any) => item.type === 'computer_call');
    
    if (computerCall) {
      callId = computerCall.call_id;
      
      const input = computerCall.action;
      
      switch (input.type) {
        case 'click':
          action = {
            type: ActionType.CLICK,
            x: input.x,
            y: input.y,
            button: (input.button as MouseButton) || 'left'
          };
          break;
        case 'double_click':
          action = {
            type: ActionType.DOUBLE_CLICK,
            x: input.x,
            y: input.y,
            button: (input.button as MouseButton) || 'left'
          };
          break;
        case 'right_click':
          action = {
            type: ActionType.CLICK,
            x: input.x,
            y: input.y,
            button: 'right'
          };
          break;
        case 'type':
          action = {
            type: ActionType.TYPE,
            text: input.text
          };
          break;
        case 'keypress':
          action = {
            type: ActionType.KEYPRESS,
            keys: Array.isArray(input.keys) ? input.keys : [input.keys]
          };
          break;
        case 'scroll':
          action = {
            type: ActionType.SCROLL,
            x: input.x || 0,
            y: input.y || 0,
            scroll_x: input.dx || 0,
            scroll_y: input.dy || 0
          };
          break;
        case 'wait':
          action = {
            type: ActionType.WAIT,
            duration_ms: input.duration_ms
          };
          break;
        case 'screenshot':
          action = {
            type: ActionType.SCREENSHOT
          };
          break;
        case 'mouse_down':
          action = {
            type: ActionType.MOUSE_DOWN,
            x: input.x,
            y: input.y,
            button: (input.button as MouseButton) || 'left'
          };
          break;
        case 'mouse_up':
          action = {
            type: ActionType.MOUSE_UP,
            x: input.x,
            y: input.y,
            button: (input.button as MouseButton) || 'left'
          };
          break;
        case 'mouse_move':
          action = {
            type: ActionType.MOUSE_MOVE,
            x: input.x,
            y: input.y
          };
          break;
      }
      
      pendingSafetyChecks = (computerCall.pending_safety_checks || []).map((check: any) => ({
        id: check.id,
        code: this.mapSafetyCheckCode(check.code),
        message: check.message
      }));
    }
    
    const reasoningItem = response.output.find((item: any) => item.type === 'reasoning');
    if (reasoningItem && reasoningItem.summary) {
      this.lastReasoning = reasoningItem.summary
        .filter((item: any) => item.type === 'summary_text')
        .map((item: any) => item.text)
        .join('\n');
    }
    
    let textResponse: string | undefined = undefined;
    const textOutputItems = response.output.filter((item: any) => 
      item.type === 'text' || 
      (item.type === 'assistant_response' && item.text)
    );
    
    if (textOutputItems.length > 0) {
      textResponse = textOutputItems
        .map((item: any) => item.type === 'text' ? item.text : item.text)
        .join('\n');
    }
    
    return {
      id: response.id,
      action,
      textResponse,
      pendingSafetyChecks,
      reasoning: this.lastReasoning,
      callId
    };
  }
  
  private mapEnvironmentType(environment: EnvironmentType): "mac" | "windows" | "ubuntu" | "browser" {
    switch (environment) {
      case EnvironmentType.BROWSER:
        return 'browser';
      case EnvironmentType.MACOS:
        return 'mac';
      case EnvironmentType.WINDOWS:
        return 'windows';
      case EnvironmentType.LINUX:
        return 'ubuntu';
      default:
        return 'browser';
    }
  }
  
  private mapSafetyCheckCode(code: string): SafetyCheckCode {
    switch (code) {
      case 'malicious_instructions':
        return SafetyCheckCode.MALICIOUS_INSTRUCTIONS;
      case 'irrelevant_domain':
        return SafetyCheckCode.IRRELEVANT_DOMAIN;
      case 'sensitive_domain':
        return SafetyCheckCode.SENSITIVE_DOMAIN;
      default:
        return SafetyCheckCode.MALICIOUS_INSTRUCTIONS;
    }
  }
}