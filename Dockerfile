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

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd make bash wget

# Install Go 1.24.1 manually since apk only has older versions
RUN echo "Installing Go 1.24.1..."
RUN mkdir -p /tmp/go-install && \
    cd /tmp/go-install && \
    wget https://go.dev/dl/go1.24.1.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz && \
    rm -rf /tmp/go-install

# Add Go to the PATH
ENV PATH="/usr/local/go/bin:${PATH}"

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