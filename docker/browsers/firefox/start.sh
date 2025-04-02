#!/bin/bash
set -e

# Create log directory with proper permissions if it doesn't exist
if [ ! -d "/var/log" ] || [ ! -w "/var/log" ]; then
  sudo mkdir -p /var/log
  sudo chmod 777 /var/log
fi

# Wait for X server to be ready
rm -f /tmp/.X0-lock

until xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; do
  echo "Waiting for X server on $DISPLAY..."
  sleep 0.1
done

# Debug: Check home directory permissions
echo "Home directory permissions:"
ls -la ${HOME}
echo "Current user: $(whoami)"
echo "UID: $(id -u)"
echo "GID: $(id -g)"

# Ensure Firefox profile directory exists with proper permissions
if [ ! -d "${HOME}/firefox-profile" ]; then
  echo "Creating Firefox profile directory"
  mkdir -p ${HOME}/firefox-profile
  chmod 755 ${HOME}/firefox-profile
else
  echo "Firefox profile directory exists, ensuring correct permissions"
  chmod 755 ${HOME}/firefox-profile
fi

# If using a custom Firefox profile instead of the default one
if [ -n "$FIREFOX_PROFILE_DIR" ]; then
  echo "Using custom Firefox profile from $FIREFOX_PROFILE_DIR"
  mkdir -p ${HOME}/firefox-profile-custom
  cp -r $FIREFOX_PROFILE_DIR/* ${HOME}/firefox-profile-custom/
  PROFILE_PATH="${HOME}/firefox-profile-custom"
else
  PROFILE_PATH="${HOME}/firefox-profile"
fi

# Configure Firefox for remote debugging and anti-fingerprinting
echo "Writing Firefox preferences to ${PROFILE_PATH}/user.js"
cat > ${PROFILE_PATH}/user.js << EOL

// Disguise automation
user_pref("dom.webdriver.enabled", false);
user_pref("media.navigator.enabled", true);
user_pref("media.navigator.permission.disabled", true);
user_pref("general.appname.override", "");
user_pref("general.appversion.override", "");
user_pref("general.oscpu.override", "");
user_pref("general.platform.override", "");
user_pref("webgl.disabled", false);
user_pref("privacy.resistFingerprinting", false);
user_pref("privacy.trackingprotection.enabled", false);
user_pref("browser.startup.page", 3);
user_pref("browser.startup.homepage", "about:blank");

// Avoid showing automation-related UI elements
user_pref("marionette.enabled", false);
user_pref("toolkit.telemetry.enabled", false);
user_pref("toolkit.telemetry.server", "");
user_pref("datareporting.healthreport.uploadEnabled", false);
user_pref("datareporting.policy.dataSubmissionEnabled", false);

// More natural browser behavior
user_pref("dom.enable_performance", true);
user_pref("dom.enable_resource_timing", true);
user_pref("dom.enable_user_timing", true);
user_pref("dom.ipc.plugins.enabled", true);
EOL

PROXY_ARG=""
if [ -n "$PROXY_SERVER" ]; then
  echo "Using proxy server: $PROXY_SERVER"
  cat >> ${PROFILE_PATH}/user.js << EOL
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

# Create media directory for Firefox
mkdir -p ${HOME}/firefox-media
chmod 755 ${HOME}/firefox-media

BROWSER_VERSION=$(node /opt/browsergrid/scripts/playwright-version-tracker.js --get-single-browser-version firefox)

echo "Starting Firefox ${BROWSER_VERSION} with profile: ${PROFILE_PATH}"
echo "Browser path: ${BROWSER_PATH}"

$BROWSER_PATH \
  --no-remote \
  --profile ${PROFILE_PATH} \
  --window-size=${RESOLUTION_WIDTH:-1920},${RESOLUTION_HEIGHT:-1080} \
  --height=${RESOLUTION_HEIGHT:-1080} \
  --width=${RESOLUTION_WIDTH:-1920} \
  --remote-debugging-port=${REMOTE_DEBUGGING_PORT:-9222} \
  --recording-output ${HOME}/firefox-media \
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
  --deny-permission-prompts \
  --remote-allow-origins=* \
  --display=${DISPLAY} \
  about:blank > /var/log/firefox.log 2> /var/log/firefox.err &

sleep 2

echo "Firefox process started, checking logs:"
head -20 /var/log/firefox.err

# Keep container running by tailing the logs
tail -f /var/log/firefox.log /var/log/firefox.err