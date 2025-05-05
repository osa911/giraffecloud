# Use official Golang Alpine image
FROM golang:1.24-alpine

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd make bash curl openssl su-exec

# Create non-root user and group
RUN addgroup -S appgroup && \
    adduser -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Set up Go environment
ENV GOCACHE=/go/cache \
    GO111MODULE=on

# Create Go directories and set permissions
RUN mkdir -p /go/pkg/mod /go/cache && \
    chown -R appuser:appgroup /go

# Copy application files
COPY Makefile /app/
COPY makefiles/ /app/makefiles/
COPY scripts/ /app/scripts/
COPY cmd /app/cmd
COPY internal /app/internal
COPY go.mod go.sum /app/

# Copy config files
COPY internal/config/env/.env.production /app/internal/config/env/.env.production
COPY internal/config/firebase/service-account.json /app/internal/config/firebase/service-account.json

# Make scripts executable
RUN chmod +x /app/scripts/*.sh

# Create necessary directories and ensure proper permissions
RUN mkdir -p /app/logs /app/certs && \
    chown -R appuser:appgroup /app && \
    chmod -R 755 /app/certs

# Switch to root temporarily for entrypoint (needed for cert generation)
USER root

# Generate certificates on container start if they don't exist and switch back to appuser
ENTRYPOINT ["/bin/sh", "-c", "[ -f /app/certs/tunnel.crt ] || /app/scripts/generate-tunnel-certs.sh && chown -R appuser:appgroup /app/certs && exec su-exec appuser /app/scripts/docker-entrypoint.sh"]