#!/bin/sh
set -e

echo "Starting GiraffeCloud server in Docker..."

ENV_FILE="/app/internal/config/env/.env.production"

# Verify we have the production env file
if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: Production environment file not found at $ENV_FILE"
    exit 1
fi

# Load environment variables
set -a
source "$ENV_FILE"
set +a

echo "Checking Caddy connectivity..."

# Try to connect to Caddy using curl over Unix socket with http+unix scheme
max_retries=5
retries=0
until curl -s --unix-socket /run/caddy/admin.sock http://unix/config/ > /dev/null; do
    retries=$((retries+1))
    if [ $retries -ge $max_retries ]; then
        echo "ERROR: Failed to connect to Caddy after $retries attempts."
        echo "Please ensure Caddy is running and the Unix socket exists at /run/caddy/admin.sock"
        exit 1
    fi
    echo "Waiting for Caddy... ($retries/$max_retries)"
    sleep 2
done

echo "Caddy is reachable!"

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

# Run the production server using the Makefile and host's Go
echo "Starting server with host's Go installation..."
cd /app && GOPATH=/go exec make prod