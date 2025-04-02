#!/bin/bash
set -e

# Create log directory with proper permissions if it doesn't exist
if [ ! -d "/var/log" ] || [ ! -w "/var/log" ]; then
  sudo mkdir -p /var/log
  sudo chmod 777 /var/log
fi

rm -f /tmp/.X0-lock

until xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; do
  echo "Waiting for X server on $DISPLAY..."
  sleep 0.1
done

# Debug: Check home directory and data directory permissions
echo "Home directory permissions:"
ls -la ${HOME}
echo "Current user: $(whoami)"
echo "UID: $(id -u)"
echo "GID: $(id -g)"

PROXY_ARG=""
if [ -n "$PROXY_SERVER" ]; then
  echo "Using proxy server: $PROXY_SERVER"
  PROXY_ARG="--http-proxy=${PROXY_SERVER} --https-proxy=${PROXY_SERVER}"
fi

# Find the WebKit browser path
WEBKIT_PATH=$(find ${HOME}/.cache/ms-playwright -name "webkit" -type d | head -1)
if [ -z "$WEBKIT_PATH" ]; then
  echo "Playwright WebKit not found"
  exit 1
fi
BROWSER_PATH="${WEBKIT_PATH}/minibrowser-gtk"

# Make a local directory for WebKit data with proper permissions
if [ ! -d "${HOME}/webkit-data" ]; then
  echo "Creating WebKit data directory"
  mkdir -p ${HOME}/webkit-data
  chmod 755 ${HOME}/webkit-data
else
  echo "WebKit data directory exists, ensuring correct permissions"
  chmod 755 ${HOME}/webkit-data
fi

# Create a WebKit to CDP proxy service
echo "Creating WebKit proxy script"
cat > ${HOME}/webkit-proxy.js << EOL
const http = require('http');
const WebSocket = require('ws');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

// Configuration
const WEBKIT_PATH = '${BROWSER_PATH}';
const PORT = ${REMOTE_DEBUGGING_PORT:-9222};
const DATA_DIR = '${HOME}/webkit-data';
const DISPLAY = process.env.DISPLAY || ':1';
const WIDTH = ${RESOLUTION_WIDTH:-1920};
const HEIGHT = ${RESOLUTION_HEIGHT:-1080};
const PROXY = '${PROXY_SERVER}';

// Start HTTP server for browser information
const httpServer = http.createServer((req, res) => {
  console.log(\`Request: \${req.url}\`);
  
  if (req.url === '/json/version') {
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify({
      'Browser': 'WebKit',
      'Protocol-Version': '1.3',
      'User-Agent': 'WebKit',
      'webSocketDebuggerUrl': \`ws://localhost:\${PORT}/devtools/browser\`
    }));
  } else if (req.url === '/json' || req.url === '/json/list') {
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify([{
      description: '',
      devtoolsFrontendUrl: \`/devtools/inspector.html?ws=localhost:\${PORT}/devtools/page/1\`,
      id: '1',
      title: 'WebKit',
      type: 'page',
      url: 'about:blank',
      webSocketDebuggerUrl: \`ws://localhost:\${PORT}/devtools/page/1\`
    }]));
  } else {
    res.statusCode = 404;
    res.end('Not found');
  }
});

// Launch WebKit browser
console.log('Starting WebKit browser...');
const args = [
  '--inspector-pipe',
  '--enable-remote-inspector',
  '--enable-memory-pressure-handler',
  '--window-size=${RESOLUTION_WIDTH},${RESOLUTION_HEIGHT}',
  '--user-data-dir=${HOME}/webkit-data',
  'about:blank'
];

if (PROXY) {
  args.push(\`--http-proxy=\${PROXY}\`);
  args.push(\`--https-proxy=\${PROXY}\`);
}

console.log(\`Launching: \${WEBKIT_PATH} \${args.join(' ')}\`);

const browser = spawn(WEBKIT_PATH, args, {
  env: {
    ...process.env,
    DISPLAY: DISPLAY,
  }
});

browser.stdout.pipe(process.stdout);
browser.stderr.pipe(process.stderr);

browser.on('error', (err) => {
  console.error('Failed to start WebKit:', err);
  process.exit(1);
});

browser.on('exit', (code) => {
  console.log(\`WebKit exited with code \${code}\`);
  process.exit(code);
});

// Start WebSocket server for CDP
const wss = new WebSocket.Server({ noServer: true });

wss.on('connection', (ws, request) => {
  console.log(\`WebSocket connection established: \${request.url}\`);
  ws.send(JSON.stringify({
    id: 0,
    method: 'Target.attachedToTarget',
    params: {
      sessionId: '1',
      targetInfo: {
        targetId: '1',
        type: 'page',
        title: 'WebKit',
        url: 'about:blank',
        attached: true
      }
    }
  }));
  
  // Handle messages (simplified)
  ws.on('message', (message) => {
    try {
      const data = JSON.parse(message);
      // Log message for debugging
      console.log('Received message:', data.method || 'response');
      
      // Send basic responses
      if (data.id) {
        ws.send(JSON.stringify({
          id: data.id,
          result: {}
        }));
      }
    } catch (e) {
      console.error('Error handling message:', e);
    }
  });
});

httpServer.on('upgrade', (request, socket, head) => {
  wss.handleUpgrade(request, socket, head, (ws) => {
    wss.emit('connection', ws, request);
  });
});

httpServer.listen(PORT, '0.0.0.0', () => {
  console.log(\`WebKit CDP proxy server running on port \${PORT}\`);
});
EOL
BROWSER_VERSION=$(node /opt/browsergrid/scripts/playwright-version-tracker.js --get-single-browser-version webkit)
echo "Starting WebKit ${BROWSER_VERSION}..."
node ${HOME}/webkit-proxy.js > /var/log/webkit.log 2> /var/log/webkit.err &

# Keep container running by tailing the logs
tail -f /var/log/webkit.log /var/log/webkit.err