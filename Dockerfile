# Build stage
FROM golang:1.24.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Final stage - use the same golang image as the base
FROM golang:1.24.1-alpine

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd make bash curl

# Verify Go installation
RUN go version

# Set working directory
WORKDIR /app

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the binary, Makefile, and scripts
COPY --from=builder /app/server /app/server
COPY Makefile /app/
COPY makefiles/ /app/makefiles/
COPY scripts/ /app/scripts/

# Copy source code for 'go run' commands
COPY --from=builder /app/cmd /app/cmd
COPY --from=builder /app/internal /app/internal
COPY --from=builder /app/go.mod /app/go.sum /app/

# Copy any additional required files
COPY internal/config/env/.env.production /app/internal/config/env/.env.production
COPY --from=builder /app/internal/config/firebase/service-account.json /app/internal/config/firebase/service-account.json

# Make scripts executable
RUN chmod +x /app/scripts/*.sh

# Create logs directory and set permissions
RUN mkdir -p /app/logs && \
    chown -R appuser:appgroup /app/logs

# Set non-root user
USER appuser

# Expose the port
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/app/scripts/docker-entrypoint.sh"]