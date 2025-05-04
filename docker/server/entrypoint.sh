#!/bin/bash
set -e

# Run migrations
python -m browsergrid.server.manage migrate

# Start the server with auto-reload in development
if [ "$BROWSERGRID_ENV" = "dev" ]; then
    echo "Starting server in development mode with auto-reload"
    exec python -m browsergrid.server.manage runserver --host $BROWSERGRID_API_HOST --port $BROWSERGRID_API_PORT --reload
else
    echo "Starting server in production mode"
    exec python -m browsergrid.server.manage runserver --host $BROWSERGRID_API_HOST --port $BROWSERGRID_API_PORT
fi