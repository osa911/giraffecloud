.PHONY: dev dev-hot prod build test caddy-start caddy-stop caddy-reload

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
build:
	@echo "Building application..."
	@go build -o bin/giraffecloud cmd/server/main.go

build-client:
	@echo "Building CLI client..."
	@./scripts/build-cli.sh

# Test commands
test:
	@echo "Running tests..."
	@go test ./...