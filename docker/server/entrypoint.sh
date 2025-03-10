#!/bin/bash
set -e

# Run migrations
python -m browserfleet.server.manage migrate

# Start the server
exec python -m browserfleet.server.manage runserver --host $BROWSERFLEET_API_HOST --port $BROWSERFLEET_API_PORT 