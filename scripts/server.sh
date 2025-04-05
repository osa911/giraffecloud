#!/bin/bash

# Set environment variables from .env.development file
set -a
source internal/config/env/.env.development
set +a

# Print database connection details (for debugging)
echo "DB_HOST=$DB_HOST"
echo "DB_PORT=$DB_PORT"
echo "DB_USER=$DB_USER"
echo "DB_PASSWORD=******"
echo "DB_NAME=$DB_NAME"
echo "DB_SSL_MODE=$DB_SSL_MODE"

# Check database connection
echo "Checking database connection..."
if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -p $DB_PORT -c '\l' | grep -q $DB_NAME; then
    echo "Database $DB_NAME exists."
else
    echo "Creating database $DB_NAME..."
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -p $DB_PORT -c "CREATE DATABASE $DB_NAME;"
fi

# Run the server
echo "Starting server..."
go run cmd/server/main.go