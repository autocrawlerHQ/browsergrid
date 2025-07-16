// BrowserGrid Deployment System Demo
// This script demonstrates the complete workflow of the deployment system

import { chromium } from 'playwright';

/**
 * Main automation function
 * This function will be executed by the BrowserGrid deployment runner
 */
async function main() {
  console.log('🚀 BrowserGrid Deployment Demo Starting...');
  
  // Browser connection is automatically injected by BrowserGrid
  const wsEndpoint = process.env.BROWSER_WS_ENDPOINT;
  const sessionId = process.env.BROWSER_SESSION_ID;
  
  if (!wsEndpoint) {
    throw new Error('Browser WebSocket endpoint not provided');
  }
  
  console.log(`🔗 Connecting to browser session: ${sessionId}`);
  console.log(`🌐 WebSocket endpoint: ${wsEndpoint}`);
  
  // Connect to the browser instance
  const browser = await chromium.connect({
    wsEndpoint: wsEndpoint
  });
  
  console.log('✅ Connected to browser successfully');
  
  // Create a new page
  const page = await browser.newPage();
  
  // Example 1: Basic navigation and data extraction
  console.log('📄 Navigating to example.com...');
  await page.goto('https://example.com');
  
  const title = await page.title();
  const url = page.url();
  
  console.log(`📰 Page title: ${title}`);
  console.log(`🔗 Page URL: ${url}`);
  
  // Example 2: More complex automation
  console.log('🔍 Extracting page content...');
  
  const pageData = await page.evaluate(() => {
    return {
      title: document.title,
      heading: document.querySelector('h1')?.textContent,
      paragraphs: Array.from(document.querySelectorAll('p')).map(p => p.textContent),
      links: Array.from(document.querySelectorAll('a')).map(a => ({
        text: a.textContent,
        href: a.href
      })),
      timestamp: new Date().toISOString()
    };
  });
  
  console.log('📊 Extracted data:', JSON.stringify(pageData, null, 2));
  
  // Example 3: Screenshot capture
  console.log('📸 Taking screenshot...');
  const screenshot = await page.screenshot({ 
    fullPage: true,
    type: 'png'
  });
  
  console.log(`📷 Screenshot captured: ${screenshot.length} bytes`);
  
  // Example 4: Environment variable usage
  console.log('🌍 Environment variables:');
  console.log(`- NODE_ENV: ${process.env.NODE_ENV}`);
  console.log(`- INSTANCE_ID: ${process.env.INSTANCE_ID || 'default'}`);
  console.log(`- BROWSER_SESSION_ID: ${process.env.BROWSER_SESSION_ID}`);
  
  // Example 5: Error handling
  try {
    await page.goto('https://non-existent-domain-12345.com');
  } catch (error) {
    console.log('⚠️ Expected error handled:', error.message);
  }
  
  // Example 6: Multiple page operations
  console.log('🔄 Testing multiple page operations...');
  const page2 = await browser.newPage();
  await page2.goto('https://httpbin.org/json');
  
  const jsonData = await page2.evaluate(() => {
    return JSON.parse(document.body.textContent);
  });
  
  console.log('📋 JSON data from httpbin:', jsonData);
  
  // Clean up
  await page2.close();
  await page.close();
  
  console.log('✅ Demo completed successfully!');
  
  // Return results that will be captured by BrowserGrid
  return {
    status: 'success',
    demo_results: {
      page_title: title,
      page_url: url,
      extracted_data: pageData,
      screenshot_size: screenshot.length,
      json_test: jsonData,
      environment: {
        node_env: process.env.NODE_ENV,
        instance_id: process.env.INSTANCE_ID,
        session_id: process.env.BROWSER_SESSION_ID
      },
      execution_time: new Date().toISOString(),
      pages_processed: 2
    },
    metrics: {
      total_pages: 2,
      errors_handled: 1,
      screenshot_captured: true,
      data_extracted: true
    }
  };
}

// Error handling wrapper
async function runDemo() {
  try {
    const result = await main();
    console.log('🎉 Demo completed successfully!');
    return result;
  } catch (error) {
    console.error('❌ Demo failed:', error.message);
    console.error('📋 Stack trace:', error.stack);
    
    return {
      status: 'error',
      error: error.message,
      stack: error.stack,
      timestamp: new Date().toISOString()
    };
  }
}

// Export the main function for BrowserGrid
export default runDemo; 