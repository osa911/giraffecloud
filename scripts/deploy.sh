#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

ENV_DIR="internal/config/env"
PROD_ENV="${ENV_DIR}/.env.production"
EXAMPLE_ENV="${ENV_DIR}/.env.example"

echo -e "${GREEN}====== GiraffeCloud Deployment Script ======${NC}"

# Check if docker and docker-compose are installed
if ! command -v docker &> /dev/null || ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}Error: Docker and Docker Compose are required.${NC}"
    echo "Please install them before running this script."
    echo "Visit https://docs.docker.com/get-docker/ and https://docs.docker.com/compose/install/"
    exit 1
fi

# Check if .env.production file exists
if [ ! -f "$PROD_ENV" ]; then
    echo -e "${YELLOW}Production environment file not found at $PROD_ENV${NC}"
    if [ -f "$EXAMPLE_ENV" ]; then
        echo -e "${YELLOW}Creating production environment file from example...${NC}"
        cp "$EXAMPLE_ENV" "$PROD_ENV"
        echo -e "${RED}IMPORTANT: Edit $PROD_ENV with your production settings before continuing.${NC}"
        exit 1
    else
        echo -e "${RED}Error: Example environment file not found at $EXAMPLE_ENV${NC}"
        echo "Please ensure the repository is complete."
        exit 1
    fi
else
    echo -e "${GREEN}Using existing production environment file at $PROD_ENV${NC}"
fi

# Check for Firebase service account file
if [ ! -f "internal/config/firebase/service-account.json" ]; then
    echo -e "${RED}Error: Firebase service account file not found.${NC}"
    echo "Please place your service-account.json file in internal/config/firebase/ directory."
    exit 1
fi

# Extract database variables from production environment file
echo -e "${GREEN}Extracting database configuration from production environment...${NC}"
source ./scripts/extract-db-env.sh

# Deploy options
echo -e "${GREEN}Select an option:${NC}"
echo "1. Deploy (build and start containers)"
echo "2. Update (pull changes and redeploy)"
echo "3. Start (start existing containers)"
echo "4. Stop (stop running containers)"
echo "5. Logs (view logs)"
echo "6. Backup database"
echo "7. Restore database from backup"
echo "8. Exit"

read -p "Enter your choice (1-8): " choice

run_docker_compose() {
    # Pass the extracted environment variables to docker-compose
    DB_USER=$DB_USER DB_PASSWORD=$DB_PASSWORD DB_NAME=$DB_NAME docker-compose "$@"
}

case $choice in
    1)
        echo -e "${GREEN}Building and starting containers...${NC}"
        run_docker_compose build
        run_docker_compose up -d
        echo -e "${GREEN}Deployment complete. Services are running.${NC}"
        echo "API is available at http://$(hostname -I | awk '{print $1}'):8080"
        ;;
    2)
        echo -e "${GREEN}Updating application...${NC}"
        git pull
        run_docker_compose down
        run_docker_compose build
        run_docker_compose up -d
        echo -e "${GREEN}Update complete. Services are running.${NC}"
        ;;
    3)
        echo -e "${GREEN}Starting containers...${NC}"
        run_docker_compose up -d
        echo -e "${GREEN}Services are running.${NC}"
        ;;
    4)
        echo -e "${YELLOW}Stopping containers...${NC}"
        run_docker_compose down
        echo -e "${GREEN}Services stopped.${NC}"
        ;;
    5)
        echo -e "${GREEN}Viewing logs (press Ctrl+C to exit)...${NC}"
        run_docker_compose logs -f
        ;;
    6)
        BACKUP_FILE="giraffecloud_backup_$(date +%Y%m%d_%H%M%S).sql"
        mkdir -p backups
        echo -e "${GREEN}Backing up database to $BACKUP_FILE...${NC}"
        run_docker_compose exec postgres pg_dump -U ${DB_USER} -d ${DB_NAME} > backups/$BACKUP_FILE
        echo -e "${GREEN}Backup completed.${NC}"
        ;;
    7)
        # List available backups
        mkdir -p backups
        echo -e "${GREEN}Available backups:${NC}"
        ls -1 backups/ | grep .sql

        read -p "Enter backup filename to restore: " BACKUP_FILE
        if [ -f "backups/$BACKUP_FILE" ]; then
            echo -e "${YELLOW}Restoring from backups/$BACKUP_FILE...${NC}"
            run_docker_compose exec postgres psql -U ${DB_USER} -d ${DB_NAME} < backups/$BACKUP_FILE
            echo -e "${GREEN}Restore completed.${NC}"
        else
            echo -e "${RED}Error: Backup file not found.${NC}"
        fi
        ;;
    8)
        echo -e "${GREEN}Exiting.${NC}"
        exit 0
        ;;
    *)
        echo -e "${RED}Invalid option.${NC}"
        exit 1
        ;;
esac