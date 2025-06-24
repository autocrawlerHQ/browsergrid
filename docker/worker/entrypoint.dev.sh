#!/bin/sh
set -e

echo "Development worker entrypoint starting..."

# Wait for database to be ready
echo "Waiting for database..."
while ! nc -z ${BROWSERGRID_POSTGRES_HOST:-db} ${BROWSERGRID_POSTGRES_PORT:-5432}; do
  sleep 1
done
echo "Database is ready!"

# Build database URL for worker
export DATABASE_URL="postgres://${BROWSERGRID_POSTGRES_USER:-browsergrid}:${BROWSERGRID_POSTGRES_PASSWORD:-browsergrid}@${BROWSERGRID_POSTGRES_HOST:-db}:${BROWSERGRID_POSTGRES_PORT:-5432}/${BROWSERGRID_POSTGRES_DB:-browsergrid}?sslmode=disable"

echo "Starting Air for hot reloading worker..."
cd /app
exec air -c .air.toml 