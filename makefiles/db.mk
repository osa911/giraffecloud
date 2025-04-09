.PHONY: db-init db-migrate db-rollback db-reset db-backup db-restore db-init-prod db-migrate-prod db-rollback-prod db-reset-prod db-backup-prod db-restore-prod db-recreate db-setup db-setup-prod

# Backup directory
BACKUP_DIR=internal/db/backups
DEV_BACKUP_DIR=$(BACKUP_DIR)/development
PROD_BACKUP_DIR=$(BACKUP_DIR)/production

# Create backup directories
$(DEV_BACKUP_DIR):
	@mkdir -p $(DEV_BACKUP_DIR)

$(PROD_BACKUP_DIR):
	@mkdir -p $(PROD_BACKUP_DIR)

# Helper function for checking PostgreSQL connection
define check_postgres_connection
	@source $(1) && \
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

# Development database commands
db-init: validate-dev-env
	@echo "Creating development database..."
	$(call check_postgres_connection,$(DEV_ENV))
	@source $(DEV_ENV) && createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true
	@echo "Development database created successfully"

db-recreate: validate-dev-env
	@echo "Dropping and recreating development database..."
	$(call check_postgres_connection,$(DEV_ENV))
	@source $(DEV_ENV) && \
		echo "Terminating all connections to $$DB_NAME..." && \
		psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
		echo "Dropping database $$DB_NAME..." && \
		dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true && \
		echo "Creating database $$DB_NAME..." && \
		createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME
	@echo "Database recreated successfully. Run 'make db-migrate' to apply migrations."

db-migrate: validate-dev-env
	@echo "Generating migration..."
	$(call check_postgres_connection,$(DEV_ENV))
	@source $(DEV_ENV) && go run internal/db/migrate/main.go

db-rollback: validate-dev-env
	@echo "Rolling back last migration..."
	$(call check_postgres_connection,$(DEV_ENV))
	@source $(DEV_ENV) && psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME -c "DROP TABLE IF EXISTS $$(ls -t migrations/*.sql | head -n1 | xargs basename | cut -d'_' -f1);"

db-reset: validate-dev-env
	@echo "Resetting development database..."
	$(call check_postgres_connection,$(DEV_ENV))
	@source $(DEV_ENV) && \
		echo "Terminating all connections to $$DB_NAME..." && \
		psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
		echo "Dropping database $$DB_NAME..." && \
		dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true && \
		echo "Creating database $$DB_NAME..." && \
		createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME && \
		echo "Applying migrations..." && \
		psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME -f migrations/$$(ls -t migrations/*.sql | head -n1 | xargs basename)
	@echo "Database reset completed"

# Development backup commands
db-backup: validate-dev-env $(DEV_BACKUP_DIR)
	@echo "Creating development database backup..."
	$(call check_postgres_connection,$(DEV_ENV))
	@source $(DEV_ENV) && pg_dump -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -Fc $$DB_NAME > $(DEV_BACKUP_DIR)/$$DB_NAME_$$(date +%Y%m%d_%H%M%S).dump
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
	@source $(DEV_ENV) && pg_restore -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME $(BACKUP)
	@echo "Database restored successfully"

# Production database commands
db-init-prod: validate-prod-env
	@echo "Creating production database..."
	$(call check_postgres_connection,$(PROD_ENV))
	@source $(PROD_ENV) && createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true
	@echo "Production database created successfully"

db-migrate-prod: validate-prod-env
	@echo "Generating production migration..."
	$(call check_postgres_connection,$(PROD_ENV))
	@source $(PROD_ENV) && go run internal/db/migrate/main.go

db-reset-prod: validate-prod-env
	@echo "Resetting production database..."
	@if [ "$(FORCE)" = "1" ]; then \
		$(call check_postgres_connection,$(PROD_ENV)); \
		source $(PROD_ENV) && \
			echo "Terminating all connections to $$DB_NAME..." && \
			psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
			echo "Dropping database $$DB_NAME..." && \
			dropdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true && \
			echo "Creating database $$DB_NAME..." && \
			createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME && \
			echo "Applying migrations..." && \
			psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME -f migrations/$$(ls -t migrations/*.sql | head -n1 | xargs basename); \
		echo "Production database reset completed"; \
	else \
		echo "This is a destructive operation. Run with FORCE=1 to proceed:"; \
		echo "  make db-reset-prod FORCE=1"; \
		exit 1; \
	fi

db-rollback-prod: validate-prod-env
	@echo "Rolling back last production migration..."
	$(call check_postgres_connection,$(PROD_ENV))
	@source $(PROD_ENV) && psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME -c "DROP TABLE IF EXISTS $$(ls -t migrations/*.sql | head -n1 | xargs basename | cut -d'_' -f1);"

# Production backup commands
db-backup-prod: validate-prod-env $(PROD_BACKUP_DIR)
	@echo "Creating production database backup..."
	$(call check_postgres_connection,$(PROD_ENV))
	@source $(PROD_ENV) && pg_dump -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -Fc $$DB_NAME > $(PROD_BACKUP_DIR)/$$DB_NAME_$$(date +%Y%m%d_%H%M%S).dump
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
		source $(PROD_ENV) && \
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

# Setup targets
db-setup: validate-dev-env
	@echo "Setting up development database environment..."
	$(call check_postgres_connection,$(DEV_ENV))
	@source $(DEV_ENV) && \
		createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true && \
		go run internal/db/migrate/main.go
	@echo "Development database setup complete!"

db-setup-prod: validate-prod-env
	@echo "Setting up production database environment..."
	$(call check_postgres_connection,$(PROD_ENV))
	@source $(PROD_ENV) && \
		createdb -h $$DB_HOST -p $$DB_PORT -U $$DB_USER $$DB_NAME || true && \
		go run internal/db/migrate/main.go
	@echo "Production database setup complete!"