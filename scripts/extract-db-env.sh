#!/bin/bash
# This script extracts database variables from .env.production for use with Docker Compose

ENV_FILE="internal/config/env/.env.production"

if [ ! -f "$ENV_FILE" ]; then
    echo "Error: Environment file not found at $ENV_FILE"
    exit 1
fi

# Create a temporary env file for Docker Compose with just the relevant variables
# This is necessary because Docker Compose needs these variables when parsing the
# docker-compose.yml file, before the env_file directive is processed

# Extract database variables from .env.production
DB_USER=$(grep -E "^DB_USER=" "$ENV_FILE" | cut -d= -f2)
DB_PASSWORD=$(grep -E "^DB_PASSWORD=" "$ENV_FILE" | cut -d= -f2)
DB_NAME=$(grep -E "^DB_NAME=" "$ENV_FILE" | cut -d= -f2)

# Set defaults if variables are not found
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}
DB_NAME=${DB_NAME:-db_name}

# Export variables for docker-compose
export DB_USER
export DB_PASSWORD
export DB_NAME

# Print confirmation
echo "Extracted database environment variables from $ENV_FILE"
echo "  DB_USER=$DB_USER"
echo "  DB_PASSWORD=[hidden]"
echo "  DB_NAME=$DB_NAME"