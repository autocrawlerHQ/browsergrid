#!/bin/bash
set -e

# Setup Nginx for noVNC - if using VNC
if [ "${ENABLE_VNC:-true}" = "true" ]; then
  /usr/local/bin/setup_nginx.sh
fi

# Start supervisord to manage processes
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

# Create Firefox profile directory if it doesn't exist
mkdir -p /home/user/firefox-profile

# Configure Firefox for remote debugging
cat > /home/user/firefox-profile/user.js << EOL
user_pref("devtools.debugger.remote-enabled", true);
user_pref("devtools.chrome.enabled", true);
user_pref("devtools.debugger.prompt-connection", false);
user_pref("browser.dom.window.dump.enabled", true);
EOL

# Find the Firefox executable path
FIREFOX_PATH=$(find /root/.cache/ms-playwright -name "firefox" -type f -executable 2>/dev/null | head -n 1)

# Check for headless mode
FIREFOX_ARGS=""
if [ "${HEADLESS:-false}" = "true" ]; then
  FIREFOX_ARGS="-headless"
fi

# Start Firefox with remote debugging enabled
${FIREFOX_PATH} \
  --no-remote \
  --profile /home/user/firefox-profile \
  --start-debugger-server 9222 \
  ${FIREFOX_ARGS} \
  about:blank &

# Keep container running
wait