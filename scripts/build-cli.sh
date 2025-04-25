#!/bin/bash
set -e

VERSION=$(git describe --tags --always --dirty)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -X giraffecloud/internal/version.Version=${VERSION} -X giraffecloud/internal/version.BuildTime=${BUILD_TIME}"

# Build for multiple platforms
GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-darwin-amd64 cmd/giraffecloud/main.go
GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-darwin-arm64 cmd/giraffecloud/main.go
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-linux-amd64 cmd/giraffecloud/main.go
GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o bin/giraffecloud-linux-arm64 cmd/giraffecloud/main.go

# Create checksums
cd bin
shasum -a 256 * > checksums.txt
cd ..

echo "CLI build complete. Binaries in ./bin directory."