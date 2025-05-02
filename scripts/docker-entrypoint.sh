#!/bin/sh
set -e

echo "Starting GiraffeCloud server in Docker..."

ENV_FILE="/app/internal/config/env/.env.production"

# Verify we have the production env file
if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: Production environment file not found at $ENV_FILE"
    exit 1
fi

# Load environment variables first so we can use CADDY_ADMIN_API
set -a
source "$ENV_FILE"
set +a

# Check if CADDY_ADMIN_API is set
if [ -z "$CADDY_ADMIN_API" ]; then
    echo "ERROR: CADDY_ADMIN_API environment variable is not set"
    exit 1
fi

# Extract host and port from CADDY_ADMIN_API
CADDY_HOST=$(echo "$CADDY_ADMIN_API" | sed -E 's|^https?://||' | cut -d: -f1)
CADDY_PORT=$(echo "$CADDY_ADMIN_API" | sed -E 's|^https?://||' | cut -d: -f2)

echo "Checking Caddy connectivity at $CADDY_HOST:$CADDY_PORT..."

# Try to connect to Caddy
max_retries=5
retries=0

until nc -z -v -w5 $CADDY_HOST $CADDY_PORT; do
    retries=$((retries+1))
    if [ $retries -ge $max_retries ]; then
        echo "ERROR: Failed to connect to Caddy after $retries attempts."
        echo "Please ensure Caddy is running and accessible at $CADDY_ADMIN_API"
        echo "If running on host machine, use host.docker.internal instead of localhost"
        exit 1
    fi
    echo "Waiting for Caddy... ($retries/$max_retries)"
    sleep 2
done

echo "Caddy is reachable!"

# Test Caddy API endpoint
echo "Testing Caddy API..."
if curl -s -f "$CADDY_ADMIN_API/config/" > /dev/null; then
    echo "Caddy API is responding correctly!"
else
    echo "ERROR: Caddy API test failed. Ensure Caddy admin API is enabled and accessible."
    exit 1
fi

# Wait for PostgreSQL to be ready
if [ -n "$DB_HOST" ] && [ -n "$DB_PORT" ]; then
    echo "Checking PostgreSQL connection at $DB_HOST:$DB_PORT..."

    # Try to connect to PostgreSQL
    max_retries=30
    retries=0

    # Use nc command which is more reliable than pg_isready in containers
    until nc -z -v -w5 $DB_HOST $DB_PORT; do
        retries=$((retries+1))
        if [ $retries -ge $max_retries ]; then
            echo "ERROR: Failed to connect to PostgreSQL after $retries attempts."
            echo "The application will start anyway, but it might fail if database operations are attempted."
            break
        fi
        echo "Waiting for PostgreSQL... ($retries/$max_retries)"
        sleep 2
    done

    if [ $retries -lt $max_retries ]; then
        echo "PostgreSQL is ready!"
    fi
else
    echo "WARNING: DB_HOST or DB_PORT not set, skipping PostgreSQL connection check"
fi

# Create logs directory if needed
mkdir -p /app/logs
echo "Log directory created at /app/logs"

# Run the production server using the Makefile
echo "Starting server with Makefile..."
cd /app && exec make prod