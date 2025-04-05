.PHONY: db-init db-migrate db-rollback db-reset db-backup db-restore db-init-prod db-migrate-prod db-rollback-prod db-reset-prod db-backup-prod db-restore-prod db-recreate

# Backup directory
BACKUP_DIR=internal/db/backups
DEV_BACKUP_DIR=$(BACKUP_DIR)/development
PROD_BACKUP_DIR=$(BACKUP_DIR)/production

# Create backup directories
$(DEV_BACKUP_DIR):
	@mkdir -p $(DEV_BACKUP_DIR)

$(PROD_BACKUP_DIR):
	@mkdir -p $(PROD_BACKUP_DIR)

# Development database commands
db-init: validate-env
	@echo "Creating development database..."
	@source $(DEV_ENV) && createdb $$DB_NAME || true

# New command to drop and recreate the database
db-recreate: validate-env
	@echo "Dropping and recreating development database..."
	@source $(DEV_ENV) && \
		echo "Terminating all connections to $$DB_NAME..." && \
		psql -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
		echo "Dropping database $$DB_NAME..." && \
		dropdb $$DB_NAME || true && \
		echo "Creating database $$DB_NAME..." && \
		createdb $$DB_NAME
	@echo "Database recreated successfully. Run 'make db-migrate' to apply migrations."

db-migrate: validate-env
	@echo "Generating migration..."
	@source $(DEV_ENV) && go run internal/db/migrate/main.go

db-rollback: validate-env
	@echo "Rolling back last migration..."
	@source $(DEV_ENV) && psql $$DB_NAME -c "DROP TABLE IF EXISTS $$(ls -t migrations/*.sql | head -n1 | xargs basename | cut -d'_' -f1);"

db-reset: validate-env db-rollback
	@echo "Resetting development database..."
	@source $(DEV_ENV) && \
		echo "Terminating all connections to $$DB_NAME..." && \
		psql -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
		echo "Dropping database $$DB_NAME..." && \
		dropdb $$DB_NAME || true
	@make db-init
	@source $(DEV_ENV) && psql $$DB_NAME -f migrations/$$(ls -t migrations/*.sql | head -n1 | xargs basename)

# Development backup commands
db-backup: validate-env $(DEV_BACKUP_DIR)
	@echo "Creating development database backup..."
	@source $(DEV_ENV) && pg_dump -Fc $$DB_NAME > $(DEV_BACKUP_DIR)/$$DB_NAME_$$(date +%Y%m%d_%H%M%S).dump
	@echo "Backup created successfully"

db-restore: validate-env
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
	@source $(DEV_ENV) && pg_restore -d $$DB_NAME $(BACKUP)
	@echo "Database restored successfully"

# Production database commands
db-init-prod: validate-env
	@echo "Creating production database..."
	@source $(PROD_ENV) && createdb $$DB_NAME || true

db-migrate-prod: validate-env
	@echo "Generating production migration..."
	@source $(PROD_ENV) && go run internal/db/migrate/main.go

db-reset-prod: validate-env db-rollback-prod
	@echo "Resetting production database..."
	@source $(PROD_ENV) && dropdb $$DB_NAME || true
	@make db-init-prod
	@source $(PROD_ENV) && psql $$DB_NAME -f migrations/$$(ls -t migrations/*.sql | head -n1 | xargs basename)

db-rollback-prod: validate-env
	@echo "Rolling back last production migration..."
	@source $(PROD_ENV) && psql $$DB_NAME -c "DROP TABLE IF EXISTS $$(ls -t migrations/*.sql | head -n1 | xargs basename | cut -d'_' -f1);"

# Production backup commands
db-backup-prod: validate-env $(PROD_BACKUP_DIR)
	@echo "Creating production database backup..."
	@source $(PROD_ENV) && pg_dump -Fc $$DB_NAME > $(PROD_BACKUP_DIR)/$$DB_NAME_$$(date +%Y%m%d_%H%M%S).dump
	@echo "Backup created successfully"

db-restore-prod: validate-env
	@echo "Available backups:"
	@ls -l $(PROD_BACKUP_DIR)/*.dump 2>/dev/null || echo "No backups found"
	@echo ""
	@echo "To restore a backup, run:"
	@echo "  make db-restore-prod BACKUP=path/to/backup.dump"
	@if [ "$(BACKUP)" = "" ]; then \
		exit 1; \
	fi
	@if [ ! -f "$(BACKUP)" ]; then \
		echo "Error: Backup file $(BACKUP) not found"; \
		exit 1; \
	fi
	@echo "Restoring database from $(BACKUP)..."
	@source $(PROD_ENV) && pg_restore -d $$DB_NAME $(BACKUP)
	@echo "Database restored successfully"