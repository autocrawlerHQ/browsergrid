#!/bin/sh
set -e

# Wait for database to be ready
echo "Waiting for database..."
while ! nc -z "$BROWSERGRID_POSTGRES_HOST" "$BROWSERGRID_POSTGRES_PORT"; do
  echo "Database is unavailable - sleeping"
  sleep 2
done

echo "Database is up - starting API server"

# Build database URL for Go application
export DATABASE_URL="postgres://${BROWSERGRID_POSTGRES_USER}:${BROWSERGRID_POSTGRES_PASSWORD}@${BROWSERGRID_POSTGRES_HOST}:${BROWSERGRID_POSTGRES_PORT}/${BROWSERGRID_POSTGRES_DB}?sslmode=disable"

# Run Atlas migrations
if command -v atlas >/dev/null 2>&1; then
  echo "Running Atlas migrations..."
  atlas migrate apply --dir "file:///migrations" --url "$DATABASE_URL" --allow-dirty
else
  echo "Atlas not found, skipping migrations"
fi

# Export port for the Go application
export PORT="${BROWSERGRID_API_PORT:-8765}"

# Start the Go API server
echo "Starting browsergrid API server on port $PORT"
exec ./api