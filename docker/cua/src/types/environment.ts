
import { Action } from './actions';


export interface ComputerEnvironment {
  initialize(options: { width: number, height: number }): Promise<void>;
  executeAction(action: Action): Promise<void>;
  takeScreenshot(): Promise<Buffer>;
  getCurrentUrl?(): Promise<string | undefined>;
  cleanup(): Promise<void>;
}

