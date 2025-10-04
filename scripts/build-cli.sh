#!/bin/bash
set -e

# Generate clean version string
GIT_COMMIT=$(git rev-parse HEAD)
COMMIT_SHORT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Try to get semantic version from tags, fallback to commit hash
if git describe --tags --exact-match >/dev/null 2>&1; then
    # On a tag - use semantic version (e.g., v1.2.3)
    VERSION=$(git describe --tags --exact-match)
else
    # Not on a tag - use commit hash (e.g., dev-8c0bb52)
    VERSION="dev-${COMMIT_SHORT}"
fi

LDFLAGS="-s -w -X giraffecloud/internal/version.Version=${VERSION} -X giraffecloud/internal/version.BuildTime=${BUILD_TIME} -X giraffecloud/internal/version.GitCommit=${GIT_COMMIT}"

# Build for multiple platforms
GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-darwin-amd64 ./cmd/giraffecloud
GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-darwin-arm64 ./cmd/giraffecloud
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-linux-amd64 ./cmd/giraffecloud
GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-linux-arm64 ./cmd/giraffecloud
GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-windows-amd64.exe ./cmd/giraffecloud
GOOS=windows GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-windows-arm64.exe ./cmd/giraffecloud

# Create checksums
cd bin
shasum -a 256 * > checksums.txt
cd ..

echo "CLI build complete. Binaries in ./bin directory."