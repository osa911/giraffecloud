.PHONY: dev dev-hot prod build build-client test caddy-start caddy-stop caddy-reload
.PHONY: test-tunnel observability-start observability-stop observability-status

# Observability commands
observability-start:
	@echo "Starting Grafana observability stack..."
	@docker-compose -f docker-compose.observability.yml up -d
	@echo "✅ Observability stack started!"
	@echo "  📊 Grafana UI:    http://localhost:3001 (admin/admin)"
	@echo "  🔍 Tempo:         http://localhost:3200"
	@echo "  📈 Prometheus:    http://localhost:19090"
	@echo "  📝 Loki:          http://localhost:3100"

observability-stop:
	@echo "Stopping Grafana observability stack..."
	@docker-compose -f docker-compose.observability.yml down

observability-status:
	@echo "Observability stack status:"
	@docker-compose -f docker-compose.observability.yml ps

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
# dev: validate-dev-env observability-start caddy-start
dev: validate-dev-env caddy-start
	@echo "Starting development server..."
	@./scripts/server.sh

# Development with hot reload
# dev-hot: validate-dev-env observability-start caddy-start
dev-hot: validate-dev-env caddy-start
	@echo "Starting development server with hot-reload..."
	@./scripts/hot-reload.sh

# Production commands
prod: validate-prod-env build
	@echo "Starting production server..."
	@set -a && source $(PROD_ENV) && set +a && ./bin/giraffecloud

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