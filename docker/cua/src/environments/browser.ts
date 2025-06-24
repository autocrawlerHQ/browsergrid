import { chromium, Browser, BrowserContext, Page } from 'playwright';
import { ComputerEnvironment, Action, ActionType } from '../types';

export interface BrowserEnvironmentOptions {
    width: number;
    height: number;
    cdpEndpoint?: string;
    contextId?: string;
    pageIdOrIndex?: string | number;
    headless?: boolean;
    browserOptions?: Record<string, any>;
}

export class BrowserEnvironment implements ComputerEnvironment {
    private browser: Browser | null = null;
    private context: BrowserContext | null = null;
    private page: Page | null = null;
    private options: BrowserEnvironmentOptions | null = null;

    async initialize(options: BrowserEnvironmentOptions): Promise<void> {
        this.options = options;

        try {
            if (options.cdpEndpoint) {
                console.log(`Connecting to remote browser at ${options.cdpEndpoint}`);
                this.browser = await chromium.connectOverCDP(options.cdpEndpoint);
            } else {
                console.log('Launching new browser');
                this.browser = await chromium.launch({
                    headless: options.headless ?? false,
                    ...options.browserOptions
                });
            }

            if (options.contextId && this.browser) {
                const contexts = this.browser.contexts();
                const targetContext = contexts.find((ctx) => {
                    return (ctx as any)._id === options.contextId;
                });

                if (targetContext) {
                    console.log(`Using existing browser context with ID ${options.contextId}`);
                    this.context = targetContext;
                } else {
                    throw new Error(`Browser context with ID ${options.contextId} not found`);
                }
            } else {
                console.log('Creating new browser context');
                this.context = await this.browser.newContext({
                    viewport: { width: options.width, height: options.height }
                });
            }

            if (options.pageIdOrIndex !== undefined && this.context) {
                const pages = this.context.pages();

                if (typeof options.pageIdOrIndex === 'number') {
                    if (options.pageIdOrIndex >= 0 && options.pageIdOrIndex < pages.length) {
                        console.log(`Using existing page at index ${options.pageIdOrIndex}`);
                        this.page = pages[options.pageIdOrIndex];
                    } else {
                        throw new Error(`Page index ${options.pageIdOrIndex} out of bounds (0-${pages.length - 1})`);
                    }
                } else {
                    const targetPage = pages.find(page => (page as any)._guid === options.pageIdOrIndex);
                    if (targetPage) {
                        console.log(`Using existing page with ID ${options.pageIdOrIndex}`);
                        this.page = targetPage;
                    } else {
                        throw new Error(`Page with ID ${options.pageIdOrIndex} not found`);
                    }
                }
            } else {
                console.log('Creating new page');
                this.page = await this.context.newPage();

                await this.page.goto('about:blank');
            }

            this._setupPageEventListeners();

            console.log('Browser environment initialized successfully');
        } catch (error) {
            console.error('Error initializing browser environment:', error);
            throw new Error(`Failed to initialize browser environment: ${error}`);
        }
    }

    async executeAction(action: Action): Promise<void> {
        if (!this.page) {
            throw new Error('Page not initialized');
        }

        try {
            await this.page.evaluate('1 + 1');
        } catch (error: any) {
            console.error('Page is no longer active, attempting to reconnect');
            await this._attemptPageReconnect();
        }

        try {
            switch (action.type) {
                case ActionType.CLICK: {
                    const clickAction = action as any;
                    await this.page.mouse.click(clickAction.x, clickAction.y, { button: clickAction.button });
                    break;
                }
                case ActionType.DOUBLE_CLICK: {
                    const doubleClickAction = action as any;
                    await this.page.mouse.dblclick(doubleClickAction.x, doubleClickAction.y, { button: doubleClickAction.button });
                    break;
                }
                case ActionType.TYPE: {
                    const typeAction = action as any;
                    await this.page.keyboard.type(typeAction.text);
                    break;
                }
                case ActionType.KEYPRESS: {
                    const keypressAction = action as any;
                    for (const key of keypressAction.keys) {
                        await this.page.keyboard.press(key);
                    }
                    break;
                }
                case ActionType.SCROLL: {
                    const scrollAction = action as any;
                    await this.page.mouse.move(scrollAction.x, scrollAction.y);
                    await this.page.evaluate(
                        ({ scrollX, scrollY }) => { window.scrollBy(scrollX, scrollY); },
                        { scrollX: scrollAction.scroll_x, scrollY: scrollAction.scroll_y }
                    );
                    break;
                }
                case ActionType.WAIT: {
                    const waitAction = action as any;
                    await new Promise(resolve => setTimeout(resolve, waitAction.duration_ms || 1000));
                    break;
                }
                case ActionType.MOUSE_DOWN: {
                    const mouseDownAction = action as any;
                    await this.page.mouse.down({ button: mouseDownAction.button });
                    break;
                }
                case ActionType.MOUSE_UP: {
                    const mouseUpAction = action as any;
                    await this.page.mouse.up({ button: mouseUpAction.button });
                    break;
                }
                case ActionType.MOUSE_MOVE: {
                    const mouseMoveAction = action as any;
                    await this.page.mouse.move(mouseMoveAction.x, mouseMoveAction.y);
                    break;
                }
                case ActionType.SCREENSHOT:
                    break;
                default:
                    throw new Error(`Unsupported action type: ${action}`);
            }
        } catch (error) {
            console.error(`Error executing action ${action}:`, error);
            throw new Error(`Failed to execute action ${action}: ${error}`);
        }
    }

    async takeScreenshot(): Promise<Buffer> {
        if (!this.page) {
            throw new Error('Page not initialized');
        }

        try {
            return await this.page.screenshot({ fullPage: true });
        } catch (error) {
            console.error('Error taking screenshot:', error);

            await this._attemptPageReconnect();

            return await this.page.screenshot({ fullPage: true });
        }
    }

    async getCurrentUrl(): Promise<string | undefined> {
        if (!this.page) {
            throw new Error('Page not initialized');
        }

        try {
            return this.page.url();
        } catch (error) {
            console.error('Error getting current URL:', error);

            await this._attemptPageReconnect();

            return this.page.url();
        }
    }

    async getBrowserContexts(): Promise<string[]> {
        if (!this.browser) {
            throw new Error('Browser not initialized');
        }

        const contexts = this.browser.contexts();

        return contexts.map((ctx, index) => {
            const id = (ctx as any)._id || `context_${index}`;
            return id;
        });
    }

    async getPages(): Promise<{ id: string, url: string, title: string }[]> {
        if (!this.context) {
            throw new Error('Browser context not initialized');
        }

        const pages = this.context.pages();

        const pageInfos = await Promise.all(pages.map(async (page, index) => {
            try {
                const url = page.url();
                const title = await page.title().catch(() => `Page ${index}`);
                const id = (page as any)._guid || `page_${index}`;

                return { id, url, title };
            } catch (error) {
                return { id: `page_${index}`, url: 'unknown', title: `Page ${index} (error)` };
            }
        }));

        return pageInfos;
    }

    async setActivePage(idOrIndex: string | number): Promise<void> {
        if (!this.context) {
            throw new Error('Browser context not initialized');
        }

        const pages = this.context.pages();

        if (typeof idOrIndex === 'number') {
            if (idOrIndex >= 0 && idOrIndex < pages.length) {
                console.log(`Switching to page at index ${idOrIndex}`);
                this.page = pages[idOrIndex];
            } else {
                throw new Error(`Page index ${idOrIndex} out of bounds (0-${pages.length - 1})`);
            }
        } else {
            const targetPage = pages.find(page => (page as any)._guid === idOrIndex);
            if (targetPage) {
                console.log(`Switching to page with ID ${idOrIndex}`);
                this.page = targetPage;
            } else {
                throw new Error(`Page with ID ${idOrIndex} not found`);
            }
        }

        this._setupPageEventListeners();
    }

    async cleanup(): Promise<void> {
        console.log('TODO: Cleaning up browser environment');
    }

    private _setupPageEventListeners(): void {
        if (!this.page) return;

        this.page.on('close', () => {
            console.log('Page was closed externally');
            this.page = null;
        });
    }

    private async _attemptPageReconnect(): Promise<void> {
        if (!this.context || !this.options) {
            throw new Error('Cannot reconnect: context or options not available');
        }

        console.log('Attempting to reconnect to page');

        if (!this.page || this.page.isClosed()) {
            this.page = await this.context.newPage();

            this._setupPageEventListeners();

            console.log('Created new page after reconnection');
        }
    }
}