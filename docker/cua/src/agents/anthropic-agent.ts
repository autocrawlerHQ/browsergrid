import Anthropic from '@anthropic-ai/sdk';
import { 
  AIAgent, 
  AIResponse, 
  AgentConfig, 
  Action, 
  ActionType,
  SafetyCheck,
  MouseButton
} from '../types';
import { BetaMessageParam, BetaToolUnion, BetaThinkingConfigParam } from '@anthropic-ai/sdk/resources/beta/messages/messages';

export class AnthropicAgent implements AIAgent {
  private config!: AgentConfig;
  private anthropicClient: Anthropic;
  private lastReasoning?: string;
  private conversationHistory: any[] = [];
  
  constructor() {
    this.anthropicClient = new Anthropic({
      apiKey: process.env.ANTHROPIC_API_KEY || ''
    });
  }
  
  async initialize(config: AgentConfig): Promise<void> {
    this.config = config;
    this.anthropicClient = new Anthropic({ apiKey: config.apiKey });
    this.conversationHistory = [];
  }
  
  async sendInitialRequest(userInstruction: string, initialScreenshot?: Buffer): Promise<AIResponse> {
    const toolVersion = "20250124";
    const betaFlag = `computer-use-2025-01-24`;
    
    const tools: BetaToolUnion[] = [
      {
        type: `computer_${toolVersion}`,
        name: "computer",
        display_width_px: this.config.displayWidth,
        display_height_px: this.config.displayHeight,
        display_number: 1
      },
      {
        type: `text_editor_${toolVersion}`,
        name: "str_replace_editor"
      },
      {
        type: `bash_${toolVersion}`,
        name: "bash"
      }
    ];
    
    let userContent: any[] = [{ type: "text", text: userInstruction }];
    
    if (initialScreenshot) {
      const screenshotBase64 = initialScreenshot.toString('base64');
      
      userContent.push({
        type: "image",
        source: {
          type: "base64",
          media_type: "image/png",
          data: screenshotBase64
        }
      });
    }
    
    const messages: BetaMessageParam[] = [{
      role: "user",
      content: userContent
    }];
    
    this.conversationHistory = [...messages];
    
    const thinking: BetaThinkingConfigParam | undefined = this.config.enableThinking ? {
      type: "enabled",
      budget_tokens: this.config.thinkingBudget || 1024
    } : undefined;
    
    try {
      const response = await this.anthropicClient.beta.messages.create({
        model: this.config.modelName,
        max_tokens: this.config.maxTokens || 4096,
        messages,
        tools,
        betas: [betaFlag],
        thinking
      });
      
      this.conversationHistory.push({
        role: "assistant",
        content: response.content
      });
      
      return this.processAnthropicResponse(response);
    } catch (error) {
      console.error('Error making Anthropic API call:', error);
      throw new Error(`Failed to get response from Anthropic: ${error}`);
    }
  }
  
  async sendActionResult(
    screenshot: Buffer, 
    previousResponseId: string, 
    callId: string,
    acknowledgedSafetyChecks?: SafetyCheck[],
    currentUrl?: string
  ): Promise<AIResponse> {
    const toolVersion = "20250124";
    const betaFlag = `computer-use-2025-01-24`;
    
    const screenshotBase64 = screenshot.toString('base64');
    
    const toolResults:any = [{
      type: "tool_result",
      tool_use_id: callId,
      content: {
        type: "image",
        source: {
          type: "base64",
          media_type: "image/png",
          data: screenshotBase64
        }
      }
    }];
    
    if (currentUrl) {
      toolResults.push({
        type: "tool_result",
        tool_use_id: callId,
        content: {
          type: "text",
          text: `Current URL: ${currentUrl}`
        }
      });
    }
    
    this.conversationHistory.push({
      role: "user",
      content: toolResults
    });
    
    try {
      const response = await this.anthropicClient.beta.messages.create({
        model: this.config.modelName,
        max_tokens: this.config.maxTokens || 4096,
        messages: this.conversationHistory,
        tools: this.getTools(),
        betas: [betaFlag],
        thinking: this.config.enableThinking ? {
          type: "enabled",
          budget_tokens: this.config.thinkingBudget || 1024
        } : undefined
      });
      
      this.conversationHistory.push({
        role: "assistant",
        content: response.content
      });
      
      return this.processAnthropicResponse(response);
    } catch (error) {
      console.error('Error sending action result to Anthropic:', error);
      throw new Error(`Failed to get response from Anthropic: ${error}`);
    }
  }
  
  getReasoning(): string | undefined {
    return this.lastReasoning;
  }
  
  private processAnthropicResponse(response: any): AIResponse {
    let action: Action | undefined;
    let callId: string | undefined;
    let textResponse: string = '';
    
    const textBlocks = response.content
      .filter((item: any) => item.type === 'text')
      .map((item: any) => item.text);
    
    if (textBlocks.length > 0) {
      textResponse = textBlocks.join('\n');
    }
    
    const toolUse = response.content.find((item: any) => item.type === 'tool_use');
    
    if (toolUse && toolUse.name === 'computer') {
      callId = toolUse.id;
      
      const input = toolUse.input;
      
      switch (input.type) {
        case 'click':
          action = {
            type: ActionType.CLICK,
            x: input.x,
            y: input.y,
            button: (input.button as MouseButton) || 'left'
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
            scroll_x: input.scroll_x || 0,
            scroll_y: input.scroll_y || 0
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
        case 'left_mouse_down':
          action = {
            type: ActionType.MOUSE_DOWN,
            x: input.x,
            y: input.y,
            button: (input.button as MouseButton) || 'left'
          };
          break;
        case 'mouse_up':
        case 'left_mouse_up':
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
      }
    }
    
    if (response.thinking && typeof response.thinking === 'string') {
      this.lastReasoning = response.thinking;
    } else if (response.thinking && response.thinking.thinking) {
      this.lastReasoning = response.thinking.thinking;
    } else {
      this.lastReasoning = undefined;
    }
    
    const pendingSafetyChecks: SafetyCheck[] = [];
    
    return {
      id: response.id,
      action,
      textResponse: textResponse.length > 0 ? textResponse : undefined,
      pendingSafetyChecks,
      reasoning: this.lastReasoning,
      callId
    };
  }
  
  private getTools(): any[] {
    const toolVersion = "20250124";
    
    return [
      {
        type: `computer_${toolVersion}`,
        name: "computer",
        display_width_px: this.config.displayWidth,
        display_height_px: this.config.displayHeight,
        display_number: 1
      },
      {
        type: `text_editor_${toolVersion}`,
        name: "str_replace_editor"
      },
      {
        type: `bash_${toolVersion}`,
        name: "bash"
      }
    ];
  }
}