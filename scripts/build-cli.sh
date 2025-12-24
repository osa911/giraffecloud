#!/bin/bash
set -e

# Generate clean version string
GIT_COMMIT=$(git rev-parse HEAD)
COMMIT_SHORT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Use VERSION from environment (GitHub Actions), or generate from git
if [ -z "$VERSION" ]; then
    # VERSION not provided - generate from git
    if git describe --tags --exact-match >/dev/null 2>&1; then
        # On a tag - use semantic version (e.g., v1.2.3)
        VERSION=$(git describe --tags --exact-match)
    else
        # Not on a tag - use commit hash (e.g., dev-8c0bb52)
        VERSION="dev-${COMMIT_SHORT}"
    fi
    echo "Generated VERSION from git: $VERSION"
else
    # VERSION provided by environment (e.g., from GitHub Actions)
    echo "Using VERSION from environment: $VERSION"
fi

LDFLAGS="-s -w -X giraffecloud/internal/version.Version=${VERSION} -X giraffecloud/internal/version.BuildTime=${BUILD_TIME} -X giraffecloud/internal/version.GitCommit=${GIT_COMMIT}"

# Build for specified platform or all platforms
build_target() {
    local os=$1
    local arch=$2
    local ext=""
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    echo "Building ${os}/${arch}..."
    GOOS=$os GOARCH=$arch go build -ldflags="${LDFLAGS}" -o "bin/giraffecloud-${os}-${arch}${ext}" ./cmd/giraffecloud
    echo "✓ ${os}/${arch} complete"
}

if [ -n "$1" ] && [ -n "$2" ]; then
    # Build specific target
    build_target "$1" "$2"
else
    # Build all platforms
    build_target "darwin" "amd64"
    build_target "darwin" "arm64"
    build_target "linux" "amd64"
    build_target "linux" "arm64"
    build_target "windows" "amd64"
    build_target "windows" "arm64"
fi

# Create checksums
cd bin
# Only generate checksums if we built everything, or append if single target
if [ -n "$1" ]; then
    # For single target, we just build. Checksums will be handled by the CI collector.
    echo "Single target build complete."
else
    shasum -a 256 * > checksums.txt
    echo "All platforms built and checksums generated."
fi
cd ..

echo "CLI build process finished. Binaries in ./bin directory."