export enum ActionType {
    CLICK = 'click',
    DOUBLE_CLICK = 'double_click',
    RIGHT_CLICK = 'right_click',
    TYPE = 'type',
    KEYPRESS = 'keypress',
    SCROLL = 'scroll',
    WAIT = 'wait',
    SCREENSHOT = 'screenshot',
    MOUSE_DOWN = 'mouse_down',
    MOUSE_UP = 'mouse_up',
    MOUSE_MOVE = 'mouse_move',
}




export type MouseButton = 'left' | 'right' | 'middle';


export interface BaseAction {
    type: ActionType;
}


export interface ClickAction extends BaseAction {
    type: ActionType.CLICK;
    x: number;
    y: number;
    button: MouseButton;
}

export interface DoubleClickAction extends BaseAction {
    type: ActionType.DOUBLE_CLICK;
    x: number;
    y: number;
    button: MouseButton;
}



export interface TypeAction extends BaseAction {
    type: ActionType.TYPE;
    text: string;
}


export interface KeypressAction extends BaseAction {
    type: ActionType.KEYPRESS;
    keys: string[];
}


export interface ScrollAction extends BaseAction {
    type: ActionType.SCROLL;
    x: number;
    y: number;
    scroll_x: number;
    scroll_y: number;
}


export interface WaitAction extends BaseAction {
    type: ActionType.WAIT;
    duration_ms?: number;
}

export interface ScreenshotAction extends BaseAction {
    type: ActionType.SCREENSHOT;
}



export interface MouseDownAction extends BaseAction {
    type: ActionType.MOUSE_DOWN;
    x: number;
    y: number;
    button: MouseButton;
}

export interface MouseUpAction extends BaseAction {
    type: ActionType.MOUSE_UP;
    x: number;
    y: number;
    button: MouseButton;
}


export interface MouseMoveAction extends BaseAction {
    type: ActionType.MOUSE_MOVE;
    x: number;
    y: number;
}


export type Action =
    | ClickAction
    | DoubleClickAction
    | TypeAction
    | KeypressAction
    | ScrollAction
    | WaitAction
    | ScreenshotAction
    | MouseDownAction
    | MouseUpAction
    | MouseMoveAction;