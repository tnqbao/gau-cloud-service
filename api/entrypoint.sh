#!/bin/sh

# Get service type from first argument, default to "http"
SERVICE_TYPE=${1:-http}

echo "Starting service: $SERVICE_TYPE"

# Detect migration path - container uses /app, local uses relative path
if [ -d "/app/migration" ]; then
    MIGRATION_PATH="/app/migration"
    BINARY_PATH="/app/gau-cloud-service.bin"
elif [ -d "./migration" ]; then
    MIGRATION_PATH="./migration"
    BINARY_PATH="./gau-cloud-service.bin"
else
    echo "ERROR: Migration directory not found in /app/migration or ./migration"
    exit 1
fi

echo "Using migration path: $MIGRATION_PATH"

# Debug: Check if migration files exist
echo "Checking migration files..."
ls -la "$MIGRATION_PATH/"

# Run migrations first
echo "Running migrations..."
echo "Database URL: $PGPOOL_URL"

# Check if migration directory has files
if [ -z "$(ls -A $MIGRATION_PATH)" ]; then
    echo "ERROR: Migration directory is empty!"
    exit 1
fi

# Run migration - DON'T add file:// prefix, just use the path directly
migrate -database "$PGPOOL_URL" -path "$MIGRATION_PATH" up
MIGRATE_EXIT_CODE=$?

if [ $MIGRATE_EXIT_CODE -ne 0 ]; then
    echo "Migrations failed with exit code: $MIGRATE_EXIT_CODE"
    echo "Listing migration files:"
    ls -la "$MIGRATION_PATH/"
    exit 1
fi

echo "Migrations completed successfully."

# Start the HTTP API service
echo "Starting HTTP API service..."
if [ -f "$BINARY_PATH" ]; then
    echo "Starting binary: $BINARY_PATH"
    exec "$BINARY_PATH"
elif [ -f "./main.go" ]; then
    echo "Binary not found, running with 'go run'..."
    go run main.go
else
    echo "ERROR: Binary not found at $BINARY_PATH and main.go not found"
    echo "Current directory contents:"
    ls -la
    exit 1
fi