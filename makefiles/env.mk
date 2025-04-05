# Environment files
ENV_DIR=internal/config/env
DEV_ENV=$(ENV_DIR)/.env.development
PROD_ENV=$(ENV_DIR)/.env.production
EXAMPLE_ENV=$(ENV_DIR)/.env.example

.PHONY: setup-dev setup-prod validate-env

# Environment setup commands
setup-dev:
	@if [ ! -f "$(DEV_ENV)" ]; then \
		cp $(EXAMPLE_ENV) $(DEV_ENV); \
		echo "Created development environment file at $(DEV_ENV)"; \
	else \
		echo "Development environment file already exists at $(DEV_ENV)"; \
	fi

setup-prod:
	@if [ ! -f "$(PROD_ENV)" ]; then \
		cp $(EXAMPLE_ENV) $(PROD_ENV); \
		echo "Created production environment file at $(PROD_ENV)"; \
	else \
		echo "Production environment file already exists at $(PROD_ENV)"; \
	fi

validate-env:
	@echo "Validating environment files..."
	@if [ ! -f "$(DEV_ENV)" ]; then \
		echo "Error: Development environment file not found at $(DEV_ENV)"; \
		echo "Run 'make setup-dev' to create it"; \
		exit 1; \
	fi
	@if [ ! -f "$(PROD_ENV)" ]; then \
		echo "Error: Production environment file not found at $(PROD_ENV)"; \
		echo "Run 'make setup-prod' to create it"; \
		exit 1; \
	fi
	@echo "Environment files validated successfully"