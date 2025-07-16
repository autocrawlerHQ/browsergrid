// BrowserGrid Deployment Example
import { chromium } from 'playwright';

async function main() {
  // Browser connection is injected via environment variables
  const browser = await chromium.connect({
    wsEndpoint: process.env.BROWSER_WS_ENDPOINT
  });
  
  const page = await browser.newPage();
  await page.goto('https://example.com');
  
  // Your automation logic here
  const title = await page.title();
  console.log('Page title:', title);
  
  // Return results (will be captured by BrowserGrid)
  return {
    title: title,
    url: page.url(),
    timestamp: new Date().toISOString()
  };
}

export default main;
