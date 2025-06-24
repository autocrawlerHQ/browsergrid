#!/bin/sh
set -e

echo "Development entrypoint starting..."

# Wait for database to be ready
echo "Waiting for database..."
while ! nc -z ${BROWSERGRID_POSTGRES_HOST:-db} ${BROWSERGRID_POSTGRES_PORT:-5432}; do
  sleep 1
done
echo "Database is ready!"

# Build database URL for Go application
export DATABASE_URL="postgres://${BROWSERGRID_POSTGRES_USER:-browsergrid}:${BROWSERGRID_POSTGRES_PASSWORD:-browsergrid}@${BROWSERGRID_POSTGRES_HOST:-db}:${BROWSERGRID_POSTGRES_PORT:-5432}/${BROWSERGRID_POSTGRES_DB:-browsergrid}?sslmode=disable"

# Apply migrations using Atlas
echo "Applying database migrations..."
echo "Using DATABASE_URL: $DATABASE_URL"
atlas migrate apply --dir "file:///migrations" --url "$DATABASE_URL" --allow-dirty

# Export port for the Go application
export PORT="${BROWSERGRID_API_PORT:-8765}"

echo "Starting Air for hot reloading..."
cd /app
exec air -c .air.toml 