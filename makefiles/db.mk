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

# Helper function to execute PostgreSQL commands (common implementation)
define run_pg_cmd
	@if command -v $(1) >/dev/null 2>&1; then \
		source $(2) && $(3); \
	else \
		echo "$(1) command not found, trying Docker alternative..."; \
		if [ -n "$(shell docker ps -q -f name=postgres)" ]; then \
			source $(2) && docker exec -i $(shell docker ps -q -f name=postgres) $(4) || true; \
		else \
			echo "Error: PostgreSQL client tools not installed and no running PostgreSQL container found"; \
			echo "Please install PostgreSQL client tools or ensure a PostgreSQL container is running"; \
			exit 1; \
		fi; \
	fi
endef

# Helper functions for development environment
define run_pg_cmd_dev
	$(call run_pg_cmd,$(1),$(DEV_ENV),$(2),$(3))
endef

# Helper function for production environment
define run_pg_cmd_prod
	$(call run_pg_cmd,$(1),$(PROD_ENV),$(2),$(3))
endef

# Helper function for database creation
define create_db
	@echo "Creating $(1) database..."
	$(call run_pg_cmd,createdb,$(2),createdb $$DB_NAME || true,createdb -U $$DB_USER $$DB_NAME || true)
	@echo "$(1) database created successfully"
endef

# Helper function for database migration
define migrate_db
	@echo "Running migrations on $(1)..."
	$(call run_pg_cmd,migrate,$(2),migrate -path=internal/db/migrations -database "postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" up,psql -U $$DB_USER -d $$DB_NAME -f /tmp/migrations.sql)
	@echo "$(1) migrations completed"
endef

# Helper function for database backup
define backup_db
	@echo "Creating backup of $(1) database..."
	@timestamp=$$(date +%Y%m%d_%H%M%S); \
	backup_file="$(3)/$(DB_NAME)_$$timestamp.dump"; \
	$(call run_pg_cmd,pg_dump,$(2),pg_dump -Fc $$DB_NAME > $$backup_file,pg_dump -U $$DB_USER -Fc $$DB_NAME > $$backup_file); \
	echo "Backup created at $$backup_file"
endef

# Development database commands
db-init: validate-dev-env
	$(call create_db,development,$(DEV_ENV))

# New command to drop and recreate the database
db-recreate: validate-dev-env
	@echo "Dropping and recreating development database..."
	$(call run_pg_cmd_dev,psql,psql -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();",psql -U $$DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();")
	$(call run_pg_cmd_dev,dropdb,dropdb $$DB_NAME || true,dropdb -U $$DB_USER $$DB_NAME || true)
	$(call create_db,development,$(DEV_ENV))
	@echo "Database recreated successfully. Run 'make db-migrate' to apply migrations."

db-migrate: validate-dev-env
	$(call migrate_db,development,$(DEV_ENV))

db-rollback: validate-dev-env
	@echo "Rolling back last migration on development..."
	$(call run_pg_cmd_dev,psql,psql $$DB_NAME -c "DROP TABLE IF EXISTS $$(ls -t migrations/*.sql | head -n1 | xargs basename | cut -d'_' -f1);",psql -U $$DB_USER -d $$DB_NAME -c "DROP TABLE IF EXISTS $$(ls -t migrations/*.sql | head -n1 | xargs basename | cut -d'_' -f1);")
	@echo "Development rollback completed"

db-reset: validate-dev-env
	@echo "Resetting development database..."
	$(call run_pg_cmd_dev,psql,psql -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();",psql -U $$DB_USER -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();")
	$(call run_pg_cmd_dev,dropdb,dropdb $$DB_NAME || true,dropdb -U $$DB_USER $$DB_NAME || true)
	$(call create_db,development,$(DEV_ENV))
	$(call migrate_db,development,$(DEV_ENV))

# Development backup commands
db-backup: validate-dev-env $(DEV_BACKUP_DIR)
	$(call backup_db,development,$(DEV_ENV),$(DEV_BACKUP_DIR))

db-restore: validate-dev-env
	@echo "Available backups:"
	@ls -l $(DEV_BACKUP_DIR)/*.dump 2>/dev/null || echo "No backups found"
	@echo ""
	@if [ -z "$(BACKUP)" ]; then \
		echo "To restore a backup, run:"; \
		echo "  make db-restore BACKUP=path/to/backup.dump"; \
		exit 1; \
	fi
	@if [ ! -f "$(BACKUP)" ]; then \
		echo "Error: Backup file '$(BACKUP)' not found"; \
		exit 1; \
	fi
	@echo "Restoring development database from $(BACKUP)..."
	$(call run_pg_cmd_dev,pg_restore,pg_restore -d $$DB_NAME $(BACKUP),pg_restore -U $$DB_USER -d $$DB_NAME < $(BACKUP))
	@echo "Database restored successfully"

# Production database commands
db-init-prod: validate-prod-env
	$(call create_db,production,$(PROD_ENV))

db-migrate-prod: validate-prod-env
	$(call migrate_db,production,$(PROD_ENV))

db-rollback-prod: validate-prod-env
	@echo "Rolling back last migration on production..."
	$(call run_pg_cmd_prod,migrate,migrate -path=internal/db/migrations -database "postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSL_MODE" down 1,psql -U $$DB_USER -d $$DB_NAME -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1" -t | xargs -I {} psql -U $$DB_USER -d $$DB_NAME -f internal/db/migrations/{}.down.sql)
	@echo "Production rollback completed"

db-reset-prod: validate-prod-env
	@echo "Resetting production database (dropping and recreating)..."
	@read -p "Are you sure you want to reset the PRODUCTION database? [y/N] " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(call run_pg_cmd_prod,dropdb,dropdb $$DB_NAME --if-exists && createdb $$DB_NAME,dropdb -U $$DB_USER --if-exists $$DB_NAME && createdb -U $$DB_USER $$DB_NAME); \
		echo "Production database reset successfully"; \
		$(MAKE) db-migrate-prod; \
	else \
		echo "Operation cancelled"; \
	fi

# Production backup commands
db-backup-prod: validate-prod-env $(PROD_BACKUP_DIR)
	$(call backup_db,production,$(PROD_ENV),$(PROD_BACKUP_DIR))

db-restore-prod: validate-prod-env
	@if [ -z "$(BACKUP)" ]; then \
		echo "Error: Please specify a backup file with BACKUP=path/to/backup"; \
		exit 1; \
	fi
	@if [ ! -f "$(BACKUP)" ]; then \
		echo "Error: Backup file '$(BACKUP)' not found"; \
		exit 1; \
	fi
	@echo "Restoring production database from $(BACKUP)..."
	@read -p "Are you sure you want to restore the PRODUCTION database? Current data will be lost. [y/N] " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(call run_pg_cmd_prod,dropdb,dropdb --if-exists $$DB_NAME && createdb $$DB_NAME && pg_restore -d $$DB_NAME $(BACKUP),dropdb -U $$DB_USER --if-exists $$DB_NAME && createdb -U $$DB_USER $$DB_NAME && cat $(BACKUP) | pg_restore -U $$DB_USER -d $$DB_NAME); \
		echo "Production database restored successfully"; \
	else \
		echo "Operation cancelled"; \
	fi