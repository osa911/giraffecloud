# Development database commands
.PHONY: db-init db-migrate-apply db-migrate-create db-migrate-revert db-reset db-backup db-restore

# Initialize development database
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