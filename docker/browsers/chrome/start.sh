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



# Handle profile mounting when running in Docker
if [ -n "$BROWSERGRID_PROFILE_ID" ]; then
  PROFILE_PATH="/var/lib/browsergrid/profiles/${BROWSERGRID_PROFILE_ID}/user-data"
  if [ -d "$PROFILE_PATH" ]; then
    echo "Using existing profile: $BROWSERGRID_PROFILE_ID"
    echo "Profile path: $PROFILE_PATH"
    
    # Remove existing data-dir if it's not a symlink
    if [ -d "${HOME}/data-dir" ] && [ ! -L "${HOME}/data-dir" ]; then
      rm -rf ${HOME}/data-dir
    fi
    
    # Create symlink to profile data
    ln -sfn $PROFILE_PATH ${HOME}/data-dir
    
    # Ensure proper permissions (only if we can write to it)
    if [ -w ${HOME}/data-dir ]; then
      chmod 755 ${HOME}/data-dir
    else
      echo "Warning: Cannot change permissions on ${HOME}/data-dir (this is normal for volume mounts)"
    fi
    
    echo "Profile mounted successfully"
  else
    echo "Profile $BROWSERGRID_PROFILE_ID not found at $PROFILE_PATH"
    echo "Creating new data directory"
    mkdir -p ${HOME}/data-dir
    if [ -w ${HOME}/data-dir ]; then
      chmod 755 ${HOME}/data-dir
    else
      echo "Warning: Cannot change permissions on ${HOME}/data-dir (this is normal for volume mounts)"
    fi
  fi
else
  # No profile specified, use default behavior
  if [ ! -d "${HOME}/data-dir" ]; then
    echo "Creating Chrome data directory"
    mkdir -p ${HOME}/data-dir
    if [ -w ${HOME}/data-dir ]; then
      chmod 755 ${HOME}/data-dir
    else
      echo "Warning: Cannot change permissions on ${HOME}/data-dir (this is normal for volume mounts)"
    fi
  else
    echo "Chrome data directory exists, ensuring correct permissions"
    if [ -w ${HOME}/data-dir ]; then
      chmod 755 ${HOME}/data-dir
    else
      echo "Warning: Cannot change permissions on ${HOME}/data-dir (this is normal for volume mounts)"
    fi
  fi
fi

PROXY_ARG=""
if [ -n "$PROXY_SERVER" ]; then
  echo "Using proxy server: $PROXY_SERVER"
  PROXY_ARG="--proxy-server=$PROXY_SERVER"
fi

echo "Starting Chrome with data directory: ${HOME}/data-dir"
exec /usr/bin/google-chrome-stable \
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
  $PROXY_ARG > /var/log/chrome.log 2> /var/log/chrome.err


# confirm we c


# ideally we connect to the container and run the following command
# ws://localhost:9222/devtools/browser/<id> 
# get the id from 
# http://localhost:9222/json/version webSocketDebuggerUrl

