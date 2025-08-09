#!/bin/sh

# Wait for database to be ready
echo "Waiting for database..."
while ! nc -z "$BROWSERGRID_POSTGRES_HOST" "$BROWSERGRID_POSTGRES_PORT"; do
  echo "Database is unavailable - sleeping"
  sleep 2
done

echo "Database is up - starting worker"

# Build database URL
DB_URL="postgres://${BROWSERGRID_POSTGRES_USER}:${BROWSERGRID_POSTGRES_PASSWORD}@${BROWSERGRID_POSTGRES_HOST}:${BROWSERGRID_POSTGRES_PORT}/${BROWSERGRID_POSTGRES_DB}?sslmode=disable"

# Only pass --pool when an explicit ID is supplied
POOL_ARGS=""
if [ -n "$BROWSERGRID_WORKER_POOL_ID" ]; then
  POOL_ARGS="--pool ${BROWSERGRID_WORKER_POOL_ID}"
fi

# Start the worker with configuration
exec ./worker \
  ${POOL_ARGS} \
  --name "${BROWSERGRID_WORKER_NAME:-worker-$(hostname)}" \
  --provider "${BROWSERGRID_WORKER_PROVIDER:-docker}" \
  --concurrency "${BROWSERGRID_WORKER_CONCURRENCY:-2}" \
  --db "$DB_URL" \
