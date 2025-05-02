#!/bin/bash

# Increase file descriptor limit
echo "Setting higher file watch limit..."
ulimit -n 4096 2>/dev/null || ulimit -n 2048 2>/dev/null || true

# Load environment variables properly with export
echo "Loading environment variables..."
set -a
source internal/config/env/.env.development
set +a

# Set development environment and logging
export ENV="development"

# Show loaded database variables for debugging
echo "Database connection parameters:"
echo "DB_HOST: $DB_HOST"
echo "DB_PORT: $DB_PORT"
echo "DB_USER: $DB_USER"
echo "DB_NAME: $DB_NAME"
echo "DB_SSL_MODE: $DB_SSL_MODE"

# Check database connection
echo "Checking database connection..."
if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -p $DB_PORT -c '\l' | grep -q $DB_NAME; then
    echo "Database $DB_NAME exists."
else
    echo "Creating database $DB_NAME..."
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -p $DB_PORT -c "CREATE DATABASE $DB_NAME;"
fi

# Execute air command with explicit environment variables
echo "Starting hot reload..."
$HOME/go/bin/air