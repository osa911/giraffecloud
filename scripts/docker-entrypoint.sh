#!/bin/sh
set -e

echo "Starting GiraffeCloud server in Docker..."

# Verify we have the production env file
if [ ! -f "/app/internal/config/env/.env.production" ]; then
    echo "ERROR: Production environment file not found"
    exit 1
fi

# Wait for PostgreSQL to be ready (optional - can be removed if not needed)
if [ -n "$DB_HOST" ] && [ -n "$DB_PORT" ]; then
    echo "Waiting for PostgreSQL to be ready..."
    timeout=60
    while ! nc -z $DB_HOST $DB_PORT >/dev/null 2>&1; do
        timeout=$(($timeout - 1))
        if [ $timeout -eq 0 ]; then
            echo "ERROR: Timed out waiting for PostgreSQL to start"
            exit 1
        fi
        echo "Waiting for PostgreSQL... ($timeout seconds left)"
        sleep 1
    done
    echo "PostgreSQL is ready!"
fi

# Create logs directory if needed
mkdir -p /app/logs

# Run the production server using the Makefile
echo "Starting production server..."
cd /app && make prod