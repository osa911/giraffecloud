.PHONY: db-init db-migrate-apply db-migrate-create db-migrate-revert db-reset db-backup db-restore db-init-prod db-migrate-prod db-recreate-prod db-backup-prod db-restore-prod db-gen db-migrate-status db-hash

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

# Development database commands
db-init: validate-dev-env db-gen
	@echo "Creating development database if it doesn't exist..."
	$(call check_postgres_connection,$(DEV_ENV))
	set -a && . $(DEV_ENV) && set +a && createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true
	@echo "Development database setup completed"

# Development database reset (for clean slate during development)
db-reset: validate-dev-env
	@echo "WARNING: This will reset the development database!"
	@if [ "$(FORCE)" = "1" ]; then \
		echo "Resetting development database..."; \
		$(call check_postgres_connection,$(DEV_ENV)); \
		set -a && . $(DEV_ENV) && set +a && \
			echo "Terminating all connections..." && \
			psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
			echo "Dropping database..." && \
			dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true && \
			echo "Creating database..." && \
			createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME; \
		echo "Database reset completed"; \
	else \
		echo "This is a destructive operation. Run with FORCE=1 to proceed:"; \
		echo "  make db-reset FORCE=1"; \
		exit 1; \
	fi

# Generate migration hash
db-hash: validate-dev-env
	$(call check_atlas_installation)
	@echo "Generating hash for development database..."
	@. $(DEV_ENV) && atlas migrate hash \
		--dir "file://internal/db/migrations"
	@echo "Hash generated successfully"

# Migration commands
db-migrate-create: validate-dev-env
	$(call check_atlas_installation)
	@if [ -z "$(NAME)" ]; then \
		echo "Error: Migration name is required. Usage: make db-migrate-create NAME=your_migration_name"; \
		exit 1; \
	fi
	@echo "Creating new migration: $(NAME)..."
	@mkdir -p internal/db/migrations
	@. $(DEV_ENV) && \
		echo "Creating detection database..." && \
		dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER --if-exists $${DB_NAME}_detect || true && \
		createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $${DB_NAME}_detect && \
		echo "Generating migration..." && \
		atlas migrate diff $(NAME) \
			--env dev \
			--var db_dev_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$${DB_NAME}_detect?sslmode=$$DB_SSL_MODE" \
			--var db_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" && \
		echo "Cleaning up detection database..." && \
		dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $${DB_NAME}_detect || true

# Apply pending migrations
db-migrate-apply: validate-dev-env
	$(call check_atlas_installation)
	@echo "Applying pending migrations..."
	@. $(DEV_ENV) && atlas migrate apply \
		--env dev \
		--var db_dev_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" \
		--var db_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE"
	@echo "Migrations applied successfully"

# Check migration status
db-migrate-status: validate-dev-env
	$(call check_atlas_installation)
	@echo "Checking migration status..."
	@. $(DEV_ENV) && atlas migrate status \
		--env dev \
		--var db_dev_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" \
		--var db_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE"

# Revert last migration
db-migrate-revert: validate-dev-env
	$(call check_atlas_installation)
	@echo "WARNING: This will revert migrations!"
	@if [ "$(FORCE)" = "1" ]; then \
		if [ -z "$(N)" ]; then \
			echo "Error: Number of migrations to revert is required. Usage: make db-migrate-revert FORCE=1 N=1"; \
			exit 1; \
		fi; \
		echo "Reverting $(N) migration(s)..."; \
		. $(DEV_ENV) && \
		echo "Creating detection database..." && \
		dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER --if-exists $${DB_NAME}_detect || true && \
		createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $${DB_NAME}_detect && \
		atlas migrate down $(N) \
			--env dev \
			--var db_dev_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$${DB_NAME}_detect?sslmode=$$DB_SSL_MODE" \
			--var db_url="postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" && \
		echo "Cleaning up detection database..." && \
		dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $${DB_NAME}_detect || true && \
		echo "Migration(s) reverted successfully"; \
	else \
		echo "This operation may result in data loss. Run with FORCE=1 and N=<number> to proceed:"; \
		echo "  make db-migrate-revert FORCE=1 N=1"; \
		exit 1; \
	fi

# Development backup commands
db-backup: validate-dev-env $(DEV_BACKUP_DIR)
	@echo "Creating development database backup..."
	$(call check_postgres_connection,$(DEV_ENV))
	@. $(DEV_ENV) && pg_dump -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -Fc $$DB_NAME > $(DEV_BACKUP_DIR)/$$DB_NAME_$$(date +%Y%m%d_%H%M%S).dump
	@echo "Backup created successfully"

db-restore: validate-dev-env
	@echo "Available backups:"
	@ls -l $(DEV_BACKUP_DIR)/*.dump 2>/dev/null || echo "No backups found"
	@echo ""
	@echo "To restore a backup, run:"
	@echo "  make db-restore BACKUP=path/to/backup.dump"
	@if [ "$(BACKUP)" = "" ]; then \
		exit 1; \
	fi
	@if [ ! -f "$(BACKUP)" ]; then \
		echo "Error: Backup file $(BACKUP) not found"; \
		exit 1; \
	fi
	@echo "Restoring database from $(BACKUP)..."
	$(call check_postgres_connection,$(DEV_ENV))
	@. $(DEV_ENV) && pg_restore -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME $(BACKUP)
	@echo "Database restored successfully"

# Production database commands
db-init-prod: validate-prod-env db-gen
	@echo "Creating production database..."
	$(call check_postgres_connection,$(PROD_ENV))
	@. $(PROD_ENV) && createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true
	@echo "Production database created successfully"

# Production database recreation with Docker
db-recreate-prod: validate-prod-env db-gen
	@echo "Dropping and recreating production database..."
	@if [ "$(FORCE)" = "1" ]; then \
		if [ ! -f "$(PROD_ENV)" ]; then \
			echo "Error: Production environment file $(PROD_ENV) not found"; \
			exit 1; \
		fi; \
		set -a && . $(PROD_ENV) && set +a && \
		echo "Terminating all connections and recreating database $$DB_NAME..." && \
		docker exec -i giraffecloud_postgres psql -U "$$DB_USER" -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
		docker exec -i giraffecloud_postgres dropdb -U "$$DB_USER" "$$DB_NAME" || true && \
		docker exec -i giraffecloud_postgres createdb -U "$$DB_USER" "$$DB_NAME" && \
		echo "Production database recreated successfully. Run 'make db-migrate-prod FORCE=1' to apply migrations."; \
	else \
		echo "This is a destructive operation. Run with FORCE=1 to proceed:"; \
		echo "  make db-recreate-prod FORCE=1"; \
		exit 1; \
	fi

# Update migration command to use Docker
db-migrate-prod: validate-prod-env
	$(call check_atlas_installation)
	@echo "WARNING: You are about to apply migrations to PRODUCTION!"
	@if [ "$(FORCE)" = "1" ]; then \
		echo "Applying migrations to production..."; \
		. $(PROD_ENV) && atlas migrate apply \
			--env prod \
			--allow-dirty \
			--var db_dev_url="postgres://$$DB_USER:$$DB_PASSWORD@localhost:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" \
			--var db_url="postgres://$$DB_USER:$$DB_PASSWORD@localhost:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" \
			--dry-run; \
		echo "Dry run completed. Review the changes above."; \
		echo "To apply the changes, run with CONFIRM=1:"; \
		echo "  make db-migrate-prod FORCE=1 CONFIRM=1"; \
		if [ "$(CONFIRM)" = "1" ]; then \
			echo "Applying migrations..."; \
			. $(PROD_ENV) && atlas migrate apply \
				--env prod \
				--allow-dirty \
				--var db_dev_url="postgres://$$DB_USER:$$DB_PASSWORD@localhost:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" \
				--var db_url="postgres://$$DB_USER:$$DB_PASSWORD@localhost:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE"; \
		fi \
	else \
		echo "This is a production operation. Run with FORCE=1 to proceed:"; \
		echo "  make db-migrate-prod FORCE=1"; \
		exit 1; \
	fi

# Production backup commands
db-backup-prod: validate-prod-env $(PROD_BACKUP_DIR)
	@echo "Creating production database backup..."
	$(call check_postgres_connection,$(PROD_ENV))
	@. $(PROD_ENV) && pg_dump -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -Fc $$DB_NAME > $(PROD_BACKUP_DIR)/$$DB_NAME_$$(date +%Y%m%d_%H%M%S).dump
	@echo "Backup created successfully"

db-restore-prod: validate-prod-env
	@if [ -z "$(BACKUP)" ]; then \
		echo "Error: Please specify a backup file with BACKUP=path/to/backup"; \
		exit 1; \
	fi
	@if [ ! -f "$(BACKUP)" ]; then \
		echo "Error: Backup file $(BACKUP) not found"; \
		exit 1; \
	fi
	@if [ "$(FORCE)" = "1" ]; then \
		echo "Restoring production database from $(BACKUP)..."; \
		$(call check_postgres_connection,$(PROD_ENV)); \
		. $(PROD_ENV) && \
			echo "Terminating all connections to $$DB_NAME..." && \
			psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
			echo "Dropping database $$DB_NAME..." && \
			dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true && \
			echo "Creating database $$DB_NAME..." && \
			createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME && \
			echo "Restoring from backup..." && \
			pg_restore -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME $(BACKUP); \
		echo "Production database restored successfully"; \
	else \
		echo "This is a destructive operation that will overwrite the current database."; \
		echo "Run with FORCE=1 to proceed:"; \
		echo "  make db-restore-prod BACKUP=$(BACKUP) FORCE=1"; \
		exit 1; \
	fi