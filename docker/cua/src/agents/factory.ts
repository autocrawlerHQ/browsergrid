/**
 * Factory for creating AI agents based on provider
 */

import { AIAgent, AIProvider } from '../types';
import { AnthropicAgent } from './anthropic-agent';
import { OpenAIAgent } from './openai-agent';

/**
 * Factory for creating AI agents based on provider
 */
export class AIAgentFactory {
  /**
   * Create an agent instance based on the provider
   */
  static createAgent(provider: AIProvider): AIAgent {
    switch (provider) {
      case AIProvider.ANTHROPIC:
        return new AnthropicAgent();
      case AIProvider.OPENAI:
        return new OpenAIAgent();
        default:
                throw new Error(`Unsupported AI provider: ${provider}`);
        }
    }
}