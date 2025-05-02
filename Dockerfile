# Use official Golang Alpine image
FROM golang:1.24-alpine

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd make bash curl

# Set working directory
WORKDIR /app

# Set up Go environment
ENV GOCACHE=/go/cache \
    GO111MODULE=on

# Create Go directories
RUN mkdir -p /go/pkg/mod /go/cache

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

# Create logs directory
RUN mkdir -p /app/logs

# Expose the port
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/app/scripts/docker-entrypoint.sh"]