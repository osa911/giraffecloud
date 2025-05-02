# Use official Golang Alpine image
FROM golang:1.24-alpine

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd make bash curl

# Set working directory
WORKDIR /app

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set up Go environment
ENV GOCACHE=/go/cache \
    GO111MODULE=on

# Create Go directories with proper permissions
RUN mkdir -p /go/pkg/mod /go/cache && \
    chown -R appuser:appgroup /go && \
    chmod -R 777 /go/pkg/mod && \
    chmod -R 777 /go/cache

# Copy application files
COPY --chown=appuser:appgroup Makefile /app/
COPY --chown=appuser:appgroup makefiles/ /app/makefiles/
COPY --chown=appuser:appgroup scripts/ /app/scripts/
COPY --chown=appuser:appgroup cmd /app/cmd
COPY --chown=appuser:appgroup internal /app/internal
COPY --chown=appuser:appgroup go.mod go.sum /app/

# Copy config files
COPY --chown=appuser:appgroup internal/config/env/.env.production /app/internal/config/env/.env.production
COPY --chown=appuser:appgroup internal/config/firebase/service-account.json /app/internal/config/firebase/service-account.json

# Make scripts executable
RUN chmod +x /app/scripts/*.sh

# Create logs directory and set permissions
RUN mkdir -p /app/logs && \
    chown -R appuser:appgroup /app/logs && \
    chmod -R 777 /app/logs

# Add a script to fix permissions on startup
COPY --chown=root:root scripts/fix-permissions.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/fix-permissions.sh

# Set non-root user
USER appuser

# Expose the port
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/fix-permissions.sh"]
CMD ["/app/scripts/docker-entrypoint.sh"]