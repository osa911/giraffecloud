.PHONY: dev dev-hot prod build test

# Development commands
dev: validate-env
	@echo "Starting development server..."
	@./scripts/server.sh

# Development with hot reload
dev-hot: validate-env
	@echo "Starting development server with hot-reload..."
	@./scripts/hot-reload.sh

# Production commands
prod: validate-env
	@echo "Starting production server..."
	@set -a && source $(PROD_ENV) && set +a && go run cmd/server/main.go

# Build commands
build:
	@echo "Building application..."
	@go build -o bin/giraffecloud cmd/server/main.go

# Test commands
test:
	@echo "Running tests..."
	@go test ./...