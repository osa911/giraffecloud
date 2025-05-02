# Common database-related variables and functions
.PHONY: db-gen db-migrate-status db-hash

# Backup directory
BACKUP_DIR=internal/db/backups
DEV_BACKUP_DIR=$(BACKUP_DIR)/development
PROD_BACKUP_DIR=$(BACKUP_DIR)/production

# Create backup directories
$(DEV_BACKUP_DIR):
	@mkdir -p $(DEV_BACKUP_DIR)

$(PROD_BACKUP_DIR):
	@mkdir -p $(PROD_BACKUP_DIR)

# Check if Atlas is installed
ATLAS_VERSION := $(shell atlas version 2>/dev/null)

# Helper function for checking PostgreSQL connection
define check_postgres_connection
	@. $(1) && \
	echo "Testing connection to PostgreSQL server..." && \
	if ! psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -c "\l" > /dev/null 2>&1; then \
		echo "Error: Could not connect to PostgreSQL server at $$DB_HOST:$$DB_PORT"; \
		echo "Please ensure:"; \
		echo "  1. PostgreSQL server is running"; \
		echo "  2. Connection credentials in $(1) are correct"; \
		echo "  3. PostgreSQL client tools are installed"; \
		echo "     • Ubuntu/Debian: sudo apt install postgresql-client"; \
		echo "     • macOS: brew install postgresql"; \
		echo "     • Windows: https://www.postgresql.org/download/windows/"; \
		exit 1; \
	else \
		echo "Connected to PostgreSQL server at $$DB_HOST:$$DB_PORT successfully"; \
	fi
endef

# Helper function for checking Atlas installation
define check_atlas_installation
	@if [ -z "$(ATLAS_VERSION)" ]; then \
		echo "Atlas is not installed. Installing..."; \
		curl -sSf https://atlasgo.sh | sh; \
	fi
endef

# Generate Ent code
db-gen:
	@echo "Generating Ent code..."
	@go generate ./...
	@echo "Ent code generated successfully"

# Generate migration hash
db-hash: validate-dev-env
	$(call check_atlas_installation)
	@echo "Generating hash for development database..."
	@. $(DEV_ENV) && atlas migrate hash \
		--dir "file://internal/db/migrations"
	@echo "Hash generated successfully"