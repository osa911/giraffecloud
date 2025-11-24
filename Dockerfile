# Build Stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy dependency files first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY Makefile ./
COPY makefiles/ ./makefiles/
COPY cmd/ ./cmd/
COPY internal/ ./internal/
# Note: internal/web is now in apps/web and not needed for server build

# Build the binary
RUN make build

# Final Stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd curl openssl su-exec bash

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/giraffecloud /app/bin/giraffecloud

# Copy config files and scripts
COPY internal/config/env/.env.production /app/internal/config/env/.env.production
COPY internal/config/firebase/service-account.json /app/internal/config/firebase/service-account.json
COPY scripts/ /app/scripts/
COPY makefiles/ /app/makefiles/
COPY Makefile /app/

# Create directories
RUN mkdir -p /app/logs /app/certs && \
    chown -R appuser:appgroup /app && \
    chmod -R 755 /app/certs && \
    chmod +x /app/scripts/*.sh

# Switch to root temporarily for entrypoint (needed for cert generation)
# The entrypoint script handles switching to appuser
USER root

ENTRYPOINT ["/bin/sh", "-c", "[ -f /app/certs/tunnel.crt ] || /app/scripts/generate-tunnel-certs.sh && chown -R appuser:appgroup /app/certs && exec su-exec appuser /app/scripts/docker-entrypoint.sh"]