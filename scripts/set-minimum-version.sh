#!/bin/bash

# Script to update minimum required client version
# Usage: ./scripts/set-minimum-version.sh [options]
#
# Examples:
#   ./scripts/set-minimum-version.sh --version v1.1.0 --force
#   ./scripts/set-minimum-version.sh -v v1.1.0 -c stable --env prod

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
VERSION=""
CHANNEL="stable"
PLATFORM="all"
ARCH="all"
FORCE_UPDATE="true"
ENV="dev"
NOTES=""
DRY_RUN=false

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Function to print colored messages
print_info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Function to show usage
show_usage() {
    cat << EOF
${BLUE}Set Minimum Required Client Version${NC}

This script updates the minimum required version for GiraffeCloud CLI clients.

${GREEN}USAGE:${NC}
    $0 [OPTIONS]

${GREEN}OPTIONS:${NC}
    -v, --version VERSION       Set minimum version (required)
    -c, --channel CHANNEL       Target channel (default: stable)
                                Options: stable, beta, test
    -p, --platform PLATFORM     Target platform (default: all)
                                Options: all, linux, darwin, windows
    -a, --arch ARCH            Target architecture (default: all)
                                Options: all, amd64, arm64
    -f, --force-update         Force update even if auto-update disabled (default: true)
    --no-force-update          Don't force update
    -e, --env ENV              Environment (default: dev)
                                Options: dev, prod
    -n, --notes NOTES          Release notes explaining why update is required
    --dry-run                  Show what would be done without making changes
    -h, --help                 Show this help message

${GREEN}EXAMPLES:${NC}
    # Set minimum version for stable channel in production
    $0 -v v1.1.0 -e prod

    # Set with custom release notes
    $0 -v v1.2.0 -n "REQUIRED: API breaking changes" -e prod

    # Test the command without making changes
    $0 -v v1.1.0 --dry-run

    # Set for test channel only
    $0 -v v0.0.0-test.abc123 -c test -e dev

    # Set for specific platform
    $0 -v v1.1.0 -p darwin -a arm64 -e prod

${GREEN}WORKFLOW:${NC}
    1. Test in dev environment first
    2. Use --dry-run to verify the changes
    3. Apply to production

${YELLOW}IMPORTANT:${NC}
    - Always test in dev environment first
    - Production updates require confirmation
    - Old clients will be forced to update immediately
    - Make sure new version is released before setting as required

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -c|--channel)
            CHANNEL="$2"
            shift 2
            ;;
        -p|--platform)
            PLATFORM="$2"
            shift 2
            ;;
        -a|--arch)
            ARCH="$2"
            shift 2
            ;;
        -f|--force-update)
            FORCE_UPDATE="true"
            shift
            ;;
        --no-force-update)
            FORCE_UPDATE="false"
            shift
            ;;
        -e|--env)
            ENV="$2"
            shift 2
            ;;
        -n|--notes)
            NOTES="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo ""
            show_usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "$VERSION" ]; then
    print_error "Version is required!"
    echo ""
    show_usage
    exit 1
fi

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    print_error "Invalid version format: $VERSION"
    print_info "Version must be in format: v1.2.3 or v1.2.3-beta.xyz"
    exit 1
fi

# Set environment file
if [ "$ENV" = "prod" ]; then
    ENV_FILE="$PROJECT_ROOT/internal/config/env/.env.production"
    CONTAINER_NAME="giraffecloud_postgres"
else
    ENV_FILE="$PROJECT_ROOT/internal/config/env/.env.development"
    CONTAINER_NAME="" # For dev, we use host connection
fi

# Check if environment file exists
if [ ! -f "$ENV_FILE" ]; then
    print_error "Environment file not found: $ENV_FILE"
    exit 1
fi

# Load environment variables
set -a
source "$ENV_FILE"
set +a

# Set default release notes if not provided
if [ -z "$NOTES" ]; then
    NOTES="Minimum version updated to $VERSION. Please update your client."
fi

# Show configuration
echo ""
print_info "Configuration:"
echo "  Environment:    $ENV"
echo "  Version:        $VERSION"
echo "  Channel:        $CHANNEL"
echo "  Platform:       $PLATFORM"
echo "  Architecture:   $ARCH"
echo "  Force Update:   $FORCE_UPDATE"
echo "  Release Notes:  $NOTES"
echo "  Database:       $DB_NAME@$DB_HOST:$DB_PORT"
echo ""

# Prepare SQL command
SQL_COMMAND="UPDATE client_versions
SET
  minimum_version = '$VERSION',
  force_update = $FORCE_UPDATE,
  release_notes = '$NOTES',
  updated_at = NOW()
WHERE
  channel = '$CHANNEL'
  AND platform = '$PLATFORM'
  AND arch = '$ARCH';"

# Show SQL that will be executed
print_info "SQL to be executed:"
echo -e "${YELLOW}$SQL_COMMAND${NC}"
echo ""

# Dry run mode
if [ "$DRY_RUN" = true ]; then
    print_warning "DRY RUN MODE - No changes will be made"
    echo ""
    print_info "The command above would be executed against: $DB_NAME"
    exit 0
fi

# Production safety check
if [ "$ENV" = "prod" ]; then
    print_warning "⚠️  PRODUCTION ENVIRONMENT DETECTED ⚠️"
    echo ""
    print_warning "This will immediately affect all users on the $CHANNEL channel!"
    print_warning "Users running versions below $VERSION will be forced to update."
    echo ""
    read -p "Are you sure you want to continue? (yes/no): " CONFIRM

    if [ "$CONFIRM" != "yes" ]; then
        print_info "Operation cancelled"
        exit 0
    fi
fi

# Execute SQL
print_info "Updating database..."

if [ "$ENV" = "prod" ]; then
    # Production: Use Docker container
    PGPASSWORD="$DB_PASSWORD" docker exec -i "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" << EOF
$SQL_COMMAND
EOF
else
    # Development: Direct connection
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << EOF
$SQL_COMMAND
EOF
fi

if [ $? -eq 0 ]; then
    echo ""
    print_success "Minimum version updated successfully!"
    echo ""
    print_info "Next steps:"
    echo "  1. Verify the update:"
    echo "     SELECT * FROM client_versions WHERE channel = '$CHANNEL';"
    echo ""
    echo "  2. Test with a client:"
    echo "     giraffecloud update --check-only"
    echo ""
    print_warning "Note: Clients below $VERSION will see 'Update Required' immediately"
else
    echo ""
    print_error "Failed to update minimum version"
    exit 1
fi

