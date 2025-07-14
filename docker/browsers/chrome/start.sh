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

# Function to safely start Chrome with error handling
start_chrome() {
  echo "Starting Chrome with data directory: ${HOME}/data-dir"
  echo "Chrome will be started with the following command:"
  echo "/usr/bin/google-chrome-stable --no-sandbox --no-first-run --user-data-dir=${HOME}/data-dir ..."
  
  # Check if data directory exists and is accessible
  if [ ! -d "${HOME}/data-dir" ]; then
    echo "ERROR: Data directory ${HOME}/data-dir does not exist!"
    return 1
  fi
  
  # Check if we can write to the data directory
  if [ ! -w "${HOME}/data-dir" ]; then
    echo "WARNING: Cannot write to data directory ${HOME}/data-dir"
    echo "This may cause Chrome to fail. Attempting to fix permissions..."
    
    # Try to fix permissions if possible
    if sudo chmod 755 "${HOME}/data-dir" 2>/dev/null; then
      echo "Fixed permissions on data directory"
    else
      echo "Cannot fix permissions - this may cause Chrome to fail"
    fi
  fi
  
  # List contents of data directory for debugging
  echo "Data directory contents:"
  ls -la "${HOME}/data-dir" || echo "Cannot list directory contents"
  
  # Test if Chrome can at least start with --version
  echo "Testing Chrome binary..."
  if ! /usr/bin/google-chrome-stable --version >/dev/null 2>&1; then
    echo "ERROR: Chrome binary is not working!"
    return 1
  fi
  
  echo "Chrome binary test passed"
  
  # Start Chrome with error handling
  /usr/bin/google-chrome-stable \
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
  
  CHROME_PID=$!
  echo "Chrome started with PID: $CHROME_PID"
  
  # Wait a bit to see if Chrome starts successfully
  sleep 3
  
  # Check if Chrome is still running
  if ! kill -0 $CHROME_PID 2>/dev/null; then
    echo "ERROR: Chrome process died immediately!"
    echo "Chrome stdout:"
    cat /var/log/chrome.log 2>/dev/null || echo "No stdout log available"
    echo "Chrome stderr:"
    cat /var/log/chrome.err 2>/dev/null || echo "No stderr log available"
    return 1
  fi
  
  echo "Chrome appears to be running successfully"
  
  # Wait for Chrome process to finish
  wait $CHROME_PID
  local exit_code=$?
  
  echo "Chrome process exited with code: $exit_code"
  echo "Chrome stdout:"
  cat /var/log/chrome.log 2>/dev/null || echo "No stdout log available"
  echo "Chrome stderr:"
  cat /var/log/chrome.err 2>/dev/null || echo "No stderr log available"
  
  return $exit_code
}

# Handle profile data directory setup
# Two scenarios:
# 1. Direct mount: Profile is mounted directly to /home/user/data-dir (preferred)
# 2. Fallback: Full profiles volume mounted with BROWSERGRID_PROFILE_ID env var

if [ -n "$BROWSERGRID_PROFILE_ID" ]; then
  # Fallback scenario: Full profiles volume mounted, use environment variable
  echo "Using fallback profile mounting with BROWSERGRID_PROFILE_ID: $BROWSERGRID_PROFILE_ID"
  PROFILE_PATH="/var/lib/browsergrid/profiles/${BROWSERGRID_PROFILE_ID}/user-data"
  
  if [ -d "$PROFILE_PATH" ]; then
    echo "Profile path found: $PROFILE_PATH"
    
    # Check profile directory permissions and ownership
    echo "Profile directory info:"
    ls -la "$PROFILE_PATH" || echo "Cannot list profile directory"
    
    # Use the permission checker script to diagnose and potentially fix issues
    if [ -f "/usr/local/bin/check_profile_permissions.sh" ]; then
      echo "Running profile permission checker..."
      /usr/local/bin/check_profile_permissions.sh "$PROFILE_PATH"
    fi
    
    # Check if profile directory is accessible
    if [ ! -r "$PROFILE_PATH" ]; then
      echo "ERROR: Profile directory is not readable: $PROFILE_PATH"
      echo "Creating new data directory instead"
      mkdir -p ${HOME}/data-dir
      chmod 755 ${HOME}/data-dir
    else
      # Remove existing data-dir if it exists
      if [ -d "${HOME}/data-dir" ]; then
        rm -rf ${HOME}/data-dir
      fi
      
      # Create symlink to profile data
      ln -sfn $PROFILE_PATH ${HOME}/data-dir
      
      # Verify symlink was created successfully
      if [ ! -L "${HOME}/data-dir" ]; then
        echo "ERROR: Failed to create symlink to profile data"
        echo "Creating new data directory instead"
        rm -rf ${HOME}/data-dir
        mkdir -p ${HOME}/data-dir
        chmod 755 ${HOME}/data-dir
      else
        echo "Profile symlinked successfully"
        echo "Symlink info:"
        ls -la ${HOME}/data-dir
      fi
    fi
  else
    echo "Profile $BROWSERGRID_PROFILE_ID not found at $PROFILE_PATH"
    echo "Creating new data directory"
    mkdir -p ${HOME}/data-dir
    chmod 755 ${HOME}/data-dir
  fi
elif [ -d "${HOME}/data-dir" ]; then
  # Direct mount scenario: Profile is already mounted to /home/user/data-dir
  echo "Using directly mounted profile directory: ${HOME}/data-dir"
  
  # Check if it looks like a mounted profile (has Chrome profile structure)
  if [ -f "${HOME}/data-dir/Preferences" ] || [ -d "${HOME}/data-dir/Default" ] || [ -f "${HOME}/data-dir/Local State" ]; then
    echo "Detected existing Chrome profile data"
    
    # Check profile directory permissions and ownership
    echo "Profile directory info:"
    ls -la "${HOME}/data-dir" || echo "Cannot list profile directory"
    
    # Use the permission checker script to diagnose and potentially fix issues
    if [ -f "/usr/local/bin/check_profile_permissions.sh" ]; then
      echo "Running profile permission checker..."
      /usr/local/bin/check_profile_permissions.sh "${HOME}/data-dir"
    fi
  else
    echo "Data directory exists but appears to be empty or new"
  fi
  
  # Verify the directory is accessible
  if [ ! -r "${HOME}/data-dir" ]; then
    echo "ERROR: Data directory is not readable: ${HOME}/data-dir"
    echo "Creating new data directory instead"
    rm -rf ${HOME}/data-dir
    mkdir -p ${HOME}/data-dir
    chmod 755 ${HOME}/data-dir
  elif [ ! -w "${HOME}/data-dir" ]; then
    echo "WARNING: Data directory is not writable: ${HOME}/data-dir"
    echo "Chrome may fail to start or function properly"
  else
    echo "Data directory is accessible"
  fi
else
  # No profile specified and no data directory exists, create it
  echo "Creating Chrome data directory"
  mkdir -p ${HOME}/data-dir
  chmod 755 ${HOME}/data-dir
fi

PROXY_ARG=""
if [ -n "$PROXY_SERVER" ]; then
  echo "Using proxy server: $PROXY_SERVER"
  PROXY_ARG="--proxy-server=$PROXY_SERVER"
fi

# Try to start Chrome with retries
MAX_RETRIES=3
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  echo "Chrome startup attempt $((RETRY_COUNT + 1)) of $MAX_RETRIES"
  
  if start_chrome; then
    echo "Chrome exited normally"
    exit 0
  else
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "Chrome failed to start or crashed (attempt $RETRY_COUNT)"
    
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
      echo "Retrying in 5 seconds..."
      sleep 5
      
      # For persistent failures, ensure we have a clean data directory
      if [ $RETRY_COUNT -eq 2 ]; then
        echo "Trying with a fresh data directory due to repeated failures..."
        rm -rf ${HOME}/data-dir
        mkdir -p ${HOME}/data-dir
        chmod 755 ${HOME}/data-dir
      fi
    fi
  fi
done

echo "Chrome failed to start after $MAX_RETRIES attempts"
echo "Keeping container alive for debugging..."

# Keep container alive so we can debug
tail -f /var/log/chrome.log /var/log/chrome.err &
sleep infinity

