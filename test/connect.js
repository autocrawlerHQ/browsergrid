const puppeteer = require('puppeteer');

async function testBrowserMuxConnection(browserMuxUrl) {
    console.log(`🔍 Testing BrowserMux connection to: ${browserMuxUrl}`);
    
    try {
        // Test 1: Check if the /json endpoint is accessible
        console.log('\n📋 Test 1: Checking /json endpoint...');
        const response = await fetch(`${browserMuxUrl}/json`);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const targets = await response.json();
        console.log(`✅ Found ${targets.length} targets`);
        
        // Display target information
        targets.forEach((target, index) => {
            console.log(`  ${index + 1}. ${target.title} (${target.type})`);
            console.log(`     ID: ${target.id}`);
            console.log(`     WebSocket URL: ${target.webSocketDebuggerUrl}`);
            
            // Check if the WebSocket URL uses the correct port
            const url = new URL(target.webSocketDebuggerUrl);
            const expectedPort = new URL(browserMuxUrl).port;
            if (url.port === expectedPort) {
                console.log(`     ✅ Port correct: ${url.port}`);
            } else {
                console.log(`     ❌ Port mismatch: expected ${expectedPort}, got ${url.port}`);
            }
        });
        
        // Test 2: Try to connect to the first available page
        if (targets.length > 0) {
            const firstPage = targets.find(t => t.type === 'page');
            if (firstPage) {
                console.log(`\n🌐 Test 2: Connecting to page "${firstPage.title}"...`);
                
                const browser = await puppeteer.connect({
                    browserWSEndpoint: firstPage.webSocketDebuggerUrl,
                    defaultViewport: null
                });
                
                const pages = await browser.pages();
                console.log(`✅ Connected successfully! Found ${pages.length} pages`);
                
                // Get or create a page to work with
                let page;
                if (pages.length > 0) {
                    page = pages[0];
                    console.log(`📄 Using existing page: ${await page.title()} (${page.url()})`);
                } else {
                    console.log('📄 Creating new page...');
                    page = await browser.newPage();
                    console.log('✅ New page created');
                }
                
                // Navigate to example.com
                console.log('\n🌐 Navigating to books.toscrape.com...');
                await page.goto('https://books.toscrape.com/', { waitUntil: 'networkidle2' });
                console.log('✅ Successfully navigated to books.toscrape.com');
                
                // Get the page title and URL after navigation
                const title = await page.title();
                const url = page.url();
                console.log(`📄 Page title: ${title}`);
                console.log(`🔗 Current URL: ${url}`);
                
                // Take a screenshot to verify everything works
                const screenshot = await page.screenshot({ 
                    type: 'png',
                    fullPage: false,
                    clip: { x: 0, y: 0, width: 800, height: 600 }
                });
                console.log(`📸 Screenshot taken (${screenshot.length} bytes)`);
                
                await browser.disconnect();
                console.log('✅ Disconnected successfully');
            } else {
                console.log('⚠️  No page targets found to connect to');
            }
        }
        
        // Test 3: Check browser version info
        console.log('\n🔧 Test 3: Checking browser version...');
        const versionResponse = await fetch(`${browserMuxUrl}/json/version`);
        if (versionResponse.ok) {
            const version = await versionResponse.json();
            console.log(`✅ Browser: ${version.Browser}`);
            console.log(`✅ Protocol: ${version['Protocol-Version']}`);
            console.log(`✅ User-Agent: ${version['User-Agent']}`);
        }
        
        console.log('\n🎉 All tests passed! BrowserMux is working correctly.');
        
    } catch (error) {
        console.error('\n❌ Test failed:', error.message);
        
        if (error.message.includes('fetch')) {
            console.log('\n💡 Troubleshooting tips:');
            console.log('  1. Make sure BrowserMux is running');
            console.log('  2. Check if the URL is correct');
            console.log('  3. Verify the port is accessible');
            console.log('  4. Check if Chrome is running with remote debugging enabled');
        }
        
        process.exit(1);
    }
}

// Get URL from command line argument or use default
const url = process.argv[2] || 'http://localhost:32773';
testBrowserMuxConnection(url);
