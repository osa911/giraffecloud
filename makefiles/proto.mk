# Protobuf generation targets

# Check if protoc tools are installed
.PHONY: proto-check
proto-check: ## Check if protobuf tools are installed
	@echo "🔍 Checking protobuf tools..."
	@which protoc > /dev/null || (echo "❌ protoc not found. Please install Protocol Buffers." && exit 1)
	@echo "✅ protoc found: $$(protoc --version)"
	@which protoc-gen-go > /dev/null || (echo "🔧 Installing protoc-gen-go..." && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)
	@which protoc-gen-go-grpc > /dev/null || (echo "🔧 Installing protoc-gen-go-grpc..." && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest)
	@echo "✅ All protobuf tools ready!"

# Generate protobuf files
.PHONY: proto-gen
proto-gen: proto-check ## Generate Go code from protobuf files
	@echo "🚀 Generating protobuf files..."
	@cd proto && export PATH=$$PATH:$$(go env GOPATH)/bin && \
		protoc --go_out=../internal/tunnel/proto --go-grpc_out=../internal/tunnel/proto tunnel.proto
	@# Move files from nested proto/ directory to correct location
	@if [ -d "internal/tunnel/proto/proto" ]; then \
		mv internal/tunnel/proto/proto/* internal/tunnel/proto/ && \
		rmdir internal/tunnel/proto/proto; \
	fi
	@echo "✅ Protobuf files generated successfully!"
	@echo "📁 Generated files:"
	@ls -la internal/tunnel/proto/*.pb.go

# Clean generated protobuf files
.PHONY: proto-clean
proto-clean: ## Remove generated protobuf files
	@echo "🧹 Cleaning generated protobuf files..."
	@rm -f internal/tunnel/proto/tunnel.pb.go internal/tunnel/proto/tunnel_grpc.pb.go
	@echo "✅ Protobuf files cleaned!"

# Verify protobuf files are up to date
.PHONY: proto-verify
proto-verify: proto-gen ## Verify protobuf files are up to date
	@echo "🔍 Verifying protobuf files are up to date..."
	@if git diff --quiet internal/tunnel/proto/; then \
		echo "✅ Protobuf files are up to date!"; \
	else \
		echo "❌ Protobuf files are out of date. Run 'make proto-gen' to update them."; \
		git diff internal/tunnel/proto/; \
		exit 1; \
	fi
