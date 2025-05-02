# Production database commands
.PHONY: db-init-prod db-migrate-prod db-recreate-prod db-backup-prod db-restore-prod

# Initialize production database
db-init-prod: validate-prod-env db-gen
	@echo "Creating production database..."
	@if [ ! -f "$(PROD_ENV)" ]; then \
		echo "Error: Production environment file $(PROD_ENV) not found"; \
		exit 1; \
	fi
	@set -a && . $(PROD_ENV) && set +a && \
	echo "Creating database $$DB_NAME..." && \
	docker exec -i giraffecloud_postgres createdb -U "$$DB_USER" "$$DB_NAME" || true
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
	@if [ ! -f "$(PROD_ENV)" ]; then \
		echo "Error: Production environment file $(PROD_ENV) not found"; \
		exit 1; \
	fi
	@set -a && . $(PROD_ENV) && set +a && \
	echo "Creating backup of database $$DB_NAME..." && \
	docker exec -i giraffecloud_postgres pg_dump -U "$$DB_USER" -Fc "$$DB_NAME" > $(PROD_BACKUP_DIR)/$$DB_NAME_$$(date +%Y%m%d_%H%M%S).dump
	@echo "Backup created successfully in $(PROD_BACKUP_DIR)"

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
		if [ ! -f "$(PROD_ENV)" ]; then \
			echo "Error: Production environment file $(PROD_ENV) not found"; \
			exit 1; \
		fi; \
		set -a && . $(PROD_ENV) && set +a && \
		echo "Terminating all connections to $$DB_NAME..." && \
		docker exec -i giraffecloud_postgres psql -U "$$DB_USER" -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$$DB_NAME' AND pid <> pg_backend_pid();" && \
		echo "Dropping database $$DB_NAME..." && \
		docker exec -i giraffecloud_postgres dropdb -U "$$DB_USER" "$$DB_NAME" || true && \
		echo "Creating database $$DB_NAME..." && \
		docker exec -i giraffecloud_postgres createdb -U "$$DB_USER" "$$DB_NAME" && \
		echo "Restoring from backup..." && \
		cat $(BACKUP) | docker exec -i giraffecloud_postgres pg_restore -U "$$DB_USER" -d "$$DB_NAME" && \
		echo "Production database restored successfully"; \
	else \
		echo "This is a destructive operation that will overwrite the current database."; \
		echo "Run with FORCE=1 to proceed:"; \
		echo "  make db-restore-prod BACKUP=$(BACKUP) FORCE=1"; \
		exit 1; \
	fi