// script to extract and manage browser versions
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Path to version manifest
const MANIFEST_PATH = path.join(__dirname, 'browser_versions.json');

/**
 * Get the latest Playwright version
 * @returns {string} The latest Playwright version
 */
function getPlaywrightVersion() {
  try {
    return execSync('npm show playwright version').toString().trim();
  } catch (error) {
    console.error('Error getting Playwright version:', error);
    process.exit(1);
  }
}

/**
 * Get the current browser versions from Playwright
 * @param {string} playwrightVersion - The Playwright version to use
 * @returns {Object} Object containing browser versions
 */
async function getBrowserVersions(playwrightVersion) {
  // Create a temporary directory
  const tempDir = path.join(__dirname, 'temp-browser-check');
  if (!fs.existsSync(tempDir)) {
    fs.mkdirSync(tempDir);
  }

  try {
    // Create a temporary package.json
    fs.writeFileSync(
      path.join(tempDir, 'package.json'),
      JSON.stringify({
        dependencies: {
          playwright: playwrightVersion
        }
      })
    );

    // Install Playwright
    execSync('npm install', { cwd: tempDir });

    // Create a script to get browser versions
    const scriptPath = path.join(tempDir, 'get-versions.js');
    fs.writeFileSync(
      scriptPath,
      `
      const { chromium, firefox, webkit } = require('playwright');
      
      async function getBrowserVersions() {
        const versions = {};
        
        // Get Chrome version
        try {
          const browser = await chromium.launch();
          versions.chrome = await browser.version();
          await browser.close();
        } catch (e) {
          console.error('Chrome error:', e);
          versions.chrome = 'unknown';
        }
        
        // Get Firefox version
        try {
          const browser = await firefox.launch();
          versions.firefox = await browser.version();
          await browser.close();
        } catch (e) {
          console.error('Firefox error:', e);
          versions.firefox = 'unknown';
        }
        
        // Get WebKit version
        try {
          const browser = await webkit.launch();
          versions.webkit = await browser.version();
          await browser.close();
        } catch (e) {
          console.error('WebKit error:', e);
          versions.webkit = 'unknown';
        }
        
        // Chromium is the same as Chrome for Playwright
        versions.chromium = versions.chrome;
        
        // Format versions to just major.minor.patch
        Object.keys(versions).forEach(browser => {
          const match = versions[browser].match(/(\d+\.\d+\.\d+)/);
          if (match) {
            versions[browser] = match[1];
          }
        });
        
        console.log(JSON.stringify(versions));
      }
      
      getBrowserVersions();
      `
    );

    // Run the script
    const output = execSync(`node ${scriptPath}`).toString().trim();
    return JSON.parse(output);
  } catch (error) {
    console.error('Error getting browser versions:', error);
    return {
      chrome: 'unknown',
      chromium: 'unknown',
      firefox: 'unknown',
      webkit: 'unknown'
    };
  } finally {
    // Clean up
    execSync(`rm -rf ${tempDir}`);
  }
}

/**
 * Load the current version manifest
 * @returns {Object} The current version manifest
 */
function loadManifest() {
  if (fs.existsSync(MANIFEST_PATH)) {
    return JSON.parse(fs.readFileSync(MANIFEST_PATH, 'utf8'));
  }
  return {
    playwrightVersion: '',
    browserVersions: {},
    lastUpdated: ''
  };
}

/**
 * Save the manifest
 * @param {Object} manifest - The manifest to save
 */
function saveManifest(manifest) {
  fs.writeFileSync(MANIFEST_PATH, JSON.stringify(manifest, null, 2));
}

/**
 * Check if versions have changed
 * @param {Object} currentManifest - The current manifest
 * @param {string} playwrightVersion - The new Playwright version
 * @param {Object} browserVersions - The new browser versions
 * @returns {Object} Object containing change information
 */
function checkForChanges(currentManifest, playwrightVersion, browserVersions) {
  const changes = {
    playwrightChanged: currentManifest.playwrightVersion !== playwrightVersion,
    browserChanges: {}
  };

  Object.keys(browserVersions).forEach(browser => {
    const oldVersion = currentManifest.browserVersions[browser];
    const newVersion = browserVersions[browser];
    changes.browserChanges[browser] = oldVersion !== newVersion;
  });

  return changes;
}

/**
 * Main function
 */
async function main() {
  // Get the latest Playwright version
  const playwrightVersion = getPlaywrightVersion();
  console.log(`Latest Playwright version: ${playwrightVersion}`);

  // Get the browser versions
  const browserVersions = await getBrowserVersions(playwrightVersion);
  console.log('Browser versions:', browserVersions);

  // Load the current manifest
  const currentManifest = loadManifest();
  console.log('Current manifest:', currentManifest);

  // Check for changes
  const changes = checkForChanges(currentManifest, playwrightVersion, browserVersions);
  console.log('Changes detected:', changes);

  // Update the manifest
  const newManifest = {
    playwrightVersion,
    browserVersions,
    lastUpdated: new Date().toISOString()
  };
  saveManifest(newManifest);

  // Return the changes
  return {
    changes,
    newManifest
  };
}

if (require.main === module) {
  main().catch(console.error);
}

module.exports = {
  getPlaywrightVersion,
  getBrowserVersions,
  loadManifest,
  saveManifest,
  checkForChanges,
  main
};