#!/bin/sh

# Get service type from first argument, default to "http"
SERVICE_TYPE=${1:-http}

echo "Starting service: $SERVICE_TYPE"

# Detect migration path - container uses /app, local uses relative path
if [ -d "/app/migration" ]; then
    MIGRATION_PATH="/app/migration"
    HTTP_BINARY="/app/http-server.bin"
    CONSUMER_BINARY="/app/consumer-worker.bin"
    HTTP_SOURCE="/app/http/main.go"
    CONSUMER_SOURCE="/app/consumer/consumer_main.go"
elif [ -d "./migration" ]; then
    MIGRATION_PATH="./migration"
    HTTP_BINARY="./http-server.bin"
    CONSUMER_BINARY="./consumer-worker.bin"
    HTTP_SOURCE="./http/main.go"
    CONSUMER_SOURCE="./consumer/consumer_main.go"
else
    echo "ERROR: Migration directory not found in /app/migration or ./migration"
    exit 1
fi

echo "Using migration path: $MIGRATION_PATH"

# Debug: Check if migration files exist
echo "Checking migration files..."
ls -la "$MIGRATION_PATH/"

# Run migrations first (only for HTTP service)
if [ "$SERVICE_TYPE" = "http" ]; then
    echo "Running migrations..."
    echo "Database URL: $PGPOOL_URL"

    # Check if migration directory has files
    if [ -z "$(ls -A $MIGRATION_PATH)" ]; then
        echo "ERROR: Migration directory is empty!"
        exit 1
    fi

    # Run migration
    migrate -database "$PGPOOL_URL" -path "$MIGRATION_PATH" up
    MIGRATE_EXIT_CODE=$?

    if [ $MIGRATE_EXIT_CODE -ne 0 ]; then
        echo "Migrations failed with exit code: $MIGRATE_EXIT_CODE"
        echo "Listing migration files:"
        ls -la "$MIGRATION_PATH/"
        exit 1
    fi

    echo "Migrations completed successfully."
fi

# Start the appropriate service
case "$SERVICE_TYPE" in
    http)
        echo "Starting HTTP Server..."
        if [ -f "$HTTP_BINARY" ]; then
            echo "Starting binary: $HTTP_BINARY"
            exec "$HTTP_BINARY"
        elif [ -f "$HTTP_SOURCE" ]; then
            echo "Binary not found, running from source: $HTTP_SOURCE"
            cd "$(dirname "$HTTP_SOURCE")" && exec go run main.go
        else
            echo "ERROR: HTTP binary not found at $HTTP_BINARY and source not found at $HTTP_SOURCE"
            echo "Current directory contents:"
            ls -la
            exit 1
        fi
        ;;
    consumer)
        echo "Starting Consumer Worker..."
        if [ -f "$CONSUMER_BINARY" ]; then
            echo "Starting binary: $CONSUMER_BINARY"
            exec "$CONSUMER_BINARY"
        elif [ -f "$CONSUMER_SOURCE" ]; then
            echo "Binary not found, running from source: $CONSUMER_SOURCE"
            cd "$(dirname "$CONSUMER_SOURCE")" && exec go run consumer_main.go
        else
            echo "ERROR: Consumer binary not found at $CONSUMER_BINARY and source not found at $CONSUMER_SOURCE"
            echo "Current directory contents:"
            ls -la
            exit 1
        fi
        ;;
    *)
        echo "ERROR: Invalid service type: $SERVICE_TYPE"
        echo "Valid options: http, consumer"
        exit 1
        ;;
esac
