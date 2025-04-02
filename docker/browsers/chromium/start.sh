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



if [ ! -d "${HOME}/data-dir" ]; then
  echo "Creating Chromium data directory"
  mkdir -p ${HOME}/data-dir
  chmod 755 ${HOME}/data-dir
else
  echo "Chromium data directory exists, ensuring correct permissions"
  chmod 755 ${HOME}/data-dir
fi

PROXY_ARG=""
if [ -n "$PROXY_SERVER" ]; then
  echo "Using proxy server: $PROXY_SERVER"
  PROXY_ARG="--proxy-server=$PROXY_SERVER"
fi

# Find the Playwright Chromium executable
BROWSER_PATH=$(find /home/user/.cache/ms-playwright -path "*/chrome-linux/chrome" -type f -executable | head -1)

if [ -z "$BROWSER_PATH" ]; then
  echo "Playwright Chromium not found, falling back to system Chrome"
  BROWSER_PATH="/usr/bin/chromium-browser"
fi

export GOOGLE_API_KEY="AIzaSyCkfPOPZXDKNn8hhgu3JrA62wIgC93d44k"
export GOOGLE_DEFAULT_CLIENT_ID="811574891467.apps.googleusercontent.com"
export GOOGLE_DEFAULT_CLIENT_SECRET="kdloedMFGdGla2P1zacGjAQh"

BROWSER_VERSION=$(node /opt/browsergrid/scripts/playwright-version-tracker.js --get-single-browser-version chromium)

echo "Starting Chromium ${BROWSER_VERSION} with data directory: ${HOME}/data-dir"
echo "Browser path: ${BROWSER_PATH}"

$BROWSER_PATH \
  --no-sandbox \
  --no-first-run \
  --disable-dev-shm-usage \
  --disable-component-update \
  --no-service-autorun \
  --password-store=basic \
  --disable-backgrounding-occluded-windows \
  --disable-renderer-backgrounding \
  --disable-background-timer-throttling \
  --disable-background-networking \
  --no-pings \
  --disable-infobars \
  --disable-breakpad \
  --no-default-browser-check \
  --remote-debugging-address=0.0.0.0 \
  --remote-debugging-port=${REMOTE_DEBUGGING_PORT} \
  --remote-allow-origins=* \
  --window-size=${RESOLUTION_WIDTH},${RESOLUTION_HEIGHT} \
  --user-data-dir=${HOME}/data-dir \
  --allow-insecure-localhost \
  --disable-blink-features=AutomationControlled \
  --flag-switches-begin \
  --flag-switches-end \
  --force-color-profile=srgb \
  --metrics-recording-only \
  --use-mock-keychain \
  --disable-background-mode \
  --enable-features=NetworkService,NetworkServiceInProcess,LoadCryptoTokenExtension,PermuteTLSExtensions \
  --disable-features=FlashDeprecationWarning,EnablePasswordsAccountStorage \
  --deny-permission-prompts \
  --accept-lang=en-US \
  --lang=en-US \
  --disable-gpu \
  --enable-unsafe-webgpu \
  $PROXY_ARG > /var/log/chrome.log 2> /var/log/chrome.err &

sleep 2

echo "Chromium process started, checking logs:"
head -20 /var/log/chrome.err

# Keep container running by tailing the logs
tail -f /var/log/chrome.log /var/log/chrome.err