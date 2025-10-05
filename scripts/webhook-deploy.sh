#!/bin/bash
set -e

# GitHub Webhook Deployment Script for GiraffeCloud
# This script is called by the webhook handler (/webhooks/github)
# Purpose: Automate deployment when you push to main branch

PROJECT_DIR="/app"  # Adjust to your project directory
CONTAINER_NAME="giraffecloud_api"
IMAGE_NAME="giraffecloud:latest"
LOG_FILE="$PROJECT_DIR/logs/webhook-deploy.log"  # Use project logs directory (no sudo needed!)

log() {
    # Create logs directory if it doesn't exist
    mkdir -p "$(dirname "$LOG_FILE")"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] WEBHOOK: $1" | tee -a "$LOG_FILE"
}

log "🚀 GitHub webhook triggered deployment..."

# Change to project directory
cd "$PROJECT_DIR" || {
    log "❌ Failed to change to project directory: $PROJECT_DIR"
    exit 1
}

log "📂 Current directory: $(pwd)"

# Git pull latest changes
log "📥 Pulling latest changes from git..."
git fetch origin
git reset --hard origin/main  # Force reset to latest main
log "✅ Git pull completed"

# Get current version info
COMMIT_HASH=$(git rev-parse HEAD)
COMMIT_MSG=$(git log -1 --pretty=format:"%s")
log "📍 Deploying commit: $COMMIT_HASH"
log "📝 Commit message: $COMMIT_MSG"

# Check if anything actually changed
if docker inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    CURRENT_COMMIT=$(docker inspect "$CONTAINER_NAME" --format='{{.Config.Labels.git_commit}}' 2>/dev/null || echo "unknown")
    if [ "$CURRENT_COMMIT" = "$COMMIT_HASH" ]; then
        log "ℹ️  No changes detected, skipping deployment"
        exit 0
    fi
fi

# Stop existing container
log "🛑 Stopping existing container..."
docker stop "$CONTAINER_NAME" 2>/dev/null && log "✅ Container stopped" || log "ℹ️  Container was not running"
docker rm "$CONTAINER_NAME" 2>/dev/null && log "✅ Container removed" || log "ℹ️  Container was already removed"

# Build new docker image with labels for tracking
log "🔨 Building Docker image..."

# Generate clean version string (matching build-cli.sh)
COMMIT_SHORT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Use VERSION from environment if provided, otherwise generate from git
if [ -z "$VERSION" ]; then
    if git describe --tags --exact-match >/dev/null 2>&1; then
        VERSION=$(git describe --tags --exact-match)
    else
        VERSION="dev-${COMMIT_SHORT}"
    fi
    log "Generated VERSION from git: $VERSION"
else
    log "Using VERSION from environment: $VERSION"
fi

BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
log "📦 Version: $VERSION"

docker build \
    --label "git_commit=$COMMIT_HASH" \
    --label "build_time=$BUILD_TIME" \
    --label "version=$VERSION" \
    --build-arg VERSION="$VERSION" \
    --build-arg BUILD_TIME="$BUILD_TIME" \
    --build-arg GIT_COMMIT="$COMMIT_HASH" \
    -t "$IMAGE_NAME" .

log "✅ Docker build completed"

# Start new container
log "🚀 Starting new container..."
docker run -d \
    --name "$CONTAINER_NAME" \
    --restart unless-stopped \
    --env-file /app/.env \
    -p 8080:8080 \
    -p 4443:4443 \
    -p 4444:4444 \
    -v /app/logs:/app/logs \
    -v /app/certs:/app/certs \
    "$IMAGE_NAME"

log "✅ Container started successfully"

# Wait for health check
log "🔍 Waiting for application to be ready..."
for i in {1..30}; do
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        log "✅ Application is healthy!"
        break
    fi
    log "⏳ Waiting for health check... ($i/30)"
    sleep 2
done

# Check if deployment was successful
if curl -f http://localhost:8080/health >/dev/null 2>&1; then
    log "🎉 Webhook deployment completed successfully!"
    log "🌐 Application is running at http://localhost:8080"
    log "📦 Deployed version: $VERSION (commit: ${COMMIT_HASH:0:8})"

    # Clean up old images (keep last 3)
    log "🧹 Cleaning up old Docker images..."
    docker images "$IMAGE_NAME" --format "table {{.ID}}\t{{.CreatedAt}}" | tail -n +4 | awk '{print $1}' | xargs -r docker rmi 2>/dev/null || true

    # Send notification (optional)
    if command -v curl >/dev/null 2>&1 && [ -n "${SLACK_WEBHOOK_URL:-}" ]; then
        curl -X POST -H 'Content-type: application/json' \
            --data "{\"text\":\"🚀 GiraffeCloud deployed successfully\\nVersion: $VERSION\\nCommit: ${COMMIT_HASH:0:8}\\nMessage: $COMMIT_MSG\"}" \
            "$SLACK_WEBHOOK_URL" 2>/dev/null || log "📱 Failed to send Slack notification"
    fi

    exit 0
else
    log "❌ Webhook deployment failed - application is not responding"

    # Show recent logs for debugging
    log "📋 Recent container logs:"
    docker logs --tail 20 "$CONTAINER_NAME" 2>&1 | tee -a "$LOG_FILE"

    # Try to restore previous version if available
    PREVIOUS_IMAGE=$(docker images "$IMAGE_NAME" --format "{{.ID}}" | sed -n '2p')
    if [ -n "$PREVIOUS_IMAGE" ]; then
        log "🔄 Attempting to restore previous version..."
        docker stop "$CONTAINER_NAME" 2>/dev/null || true
        docker rm "$CONTAINER_NAME" 2>/dev/null || true

        docker run -d \
            --name "$CONTAINER_NAME" \
            --restart unless-stopped \
            --env-file /app/.env \
            -p 8080:8080 \
            -p 4443:4443 \
            -p 4444:4444 \
            -v /app/logs:/app/logs \
            -v /app/certs:/app/certs \
            "$PREVIOUS_IMAGE"

        log "🔄 Attempted rollback to previous version"
    fi

    exit 1
fi