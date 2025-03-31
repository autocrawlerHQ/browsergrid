#!/bin/bash
set -e

rm -f /tmp/.X0-lock

until xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; do
  echo "Waiting for X server on $DISPLAY..."
  sleep 0.1
done

# Create Firefox profile directory if it doesn't exist
mkdir -p ${HOME}/firefox-profile

# Configure Firefox for remote debugging
cat > ${HOME}/firefox-profile/user.js << EOL
user_pref("devtools.debugger.remote-enabled", true);
user_pref("devtools.chrome.enabled", true);
user_pref("devtools.debugger.prompt-connection", false);
user_pref("browser.dom.window.dump.enabled", true);
user_pref("network.websocket.allowInsecureFromHTTPS", true);
user_pref("devtools.debugger.force-local", false);
user_pref("devtools.debugger.remote.port", ${REMOTE_DEBUGGING_PORT:-9222});
EOL

PROXY_ARG=""
if [ -n "$PROXY_SERVER" ]; then
  echo "Using proxy server: $PROXY_SERVER"
  cat >> ${HOME}/firefox-profile/user.js << EOL
user_pref("network.proxy.type", 1);
user_pref("network.proxy.http", "${PROXY_SERVER%:*}");
user_pref("network.proxy.http_port", ${PROXY_SERVER##*:});
user_pref("network.proxy.ssl", "${PROXY_SERVER%:*}");
user_pref("network.proxy.ssl_port", ${PROXY_SERVER##*:});
EOL
fi

# Find the Playwright Firefox executable
BROWSER_PATH=$(find ${HOME}/.cache/ms-playwright -name "firefox" -type f -executable | head -1)

if [ -z "$BROWSER_PATH" ]; then
  echo "Playwright Firefox not found, falling back to system Firefox"
  BROWSER_PATH="/usr/bin/firefox"
fi

echo "Starting Firefox..."
$BROWSER_PATH \
  --no-remote \
  --profile ${HOME}/firefox-profile \
  --window-size=${RESOLUTION_WIDTH},${RESOLUTION_HEIGHT} \
  --start-debugger-server=0.0.0.0:${REMOTE_DEBUGGING_PORT:-9222} \
  --recording-output ${HOME}/firefox-media \
  --first-startup \
  --allow-downgrade \
  --disable-backgrounding-occluded-windows \
  --disable-background-timer-throttling \
  --disable-background-networking \
  --disable-breakpad \
  --disable-component-update \
  --disable-crash-reporter \
  --disable-dev-shm-usage \
  --disable-hang-monitor \
  --disable-infobars \
  --disable-popup-blocking \
  --disable-prompt-on-repost \
  --disable-renderer-backgrounding \
  --disable-session-crashed-bubble \
  --disable-sync \
  --no-default-browser-check \
  --no-first-run \
  --no-service-autorun \
  --remote-allow-origins=* \
  --deny-permission-prompts \
  --accept-lang=en-US \
  --lang=en-US \
  about:blank > /var/log/firefox.log 2> /var/log/firefox.err &

# Keep container running by tailing the logs
tail -f /var/log/firefox.log /var/log/firefox.err