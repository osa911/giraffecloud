.PHONY: dev dev-hot prod build build-client test caddy-start caddy-stop caddy-reload
.PHONY: test-tunnel

# Caddy commands
caddy-start:
	@echo "Starting Caddy server..."
	@caddy run --config configs/caddy/Caddyfile &

caddy-stop:
	@echo "Stopping Caddy server..."
	@caddy stop

caddy-reload:
	@echo "Reloading Caddy configuration..."
	@caddy reload --config configs/caddy/Caddyfile

# Development commands
dev: validate-dev-env caddy-start
	@echo "Starting development server..."
	@./scripts/server.sh

# Development with hot reload
dev-hot: validate-dev-env caddy-start
	@echo "Starting development server with hot-reload..."
	@./scripts/hot-reload.sh

# Production commands
prod: validate-prod-env
	@echo "Starting production server..."
	@set -a && source $(PROD_ENV) && set +a && go run cmd/server/main.go

# Build commands
build: ## Build application with hybrid tunnel support
	@echo "Building application with hybrid tunnel support..."
	@go build -o bin/giraffecloud ./cmd/server/

build-client: ## Build CLI client with hybrid tunnel support
	@echo "Building CLI client with hybrid tunnel support..."
	@echo "Building multi-platform CLI releases..."
	@./scripts/build-cli.sh

# Test commands
test:
	@echo "Running tests..."
	@go test ./...

test-tunnel: ## Test tunnel functionality
	@echo "Testing hybrid tunnel connections..."
	@go test -v ./internal/tunnel/... -run TestHybrid