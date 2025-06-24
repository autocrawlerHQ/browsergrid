#!/bin/bash
set -e

# Create a properly configured nginx configuration with the correct port
sed "s/61000/${REMOTE_DEBUGGING_PORT}/g" /etc/nginx/conf.d/default.conf > /tmp/nginx.conf
mv /tmp/nginx.conf /etc/nginx/conf.d/default.conf

echo "Nginx configured with remote debugging port: ${REMOTE_DEBUGGING_PORT}" 