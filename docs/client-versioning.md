# GiraffeCloud CLI Client Versioning & Auto-Update

## Overview

GiraffeCloud uses a professional GitHub Releases-based versioning system with automatic client updates. This approach is industry-standard, secure, and scalable.

## Architecture

```
GitHub Release → Client Auto-Check → Download & Install → Service Restart → Connection Restore
```

## How It Works

### 1. **Release Process**

```bash
# Create a new release
git tag v1.2.3
git push origin v1.2.3

# GitHub Actions automatically:
# - Builds client binaries for all platforms (Linux/macOS/Windows, AMD64/ARM64)
# - Creates GitHub release with versioned binaries
# - Generates checksums for integrity verification
```

### 2. **Client Update Check**

```bash
# Manual update check
giraffecloud update --check-only

# Manual update installation
giraffecloud update

# Force update (even if same version)
giraffecloud update --force
```

### 3. **Auto-Update Configuration**

```bash
# Check auto-update status
giraffecloud auto-update status

# Enable auto-updates
giraffecloud auto-update enable

# Configure auto-update settings
giraffecloud auto-update config --interval 24h --window 02:00-06:00
```

## Version Endpoint

The server provides version information at `/api/v1/tunnels/version`:

```json
{
  "server_version": "v1.2.3",
  "build_time": "2024-01-15T10:30:00Z",
  "git_commit": "abc123def456",
  "go_version": "go1.21.0",
  "platform": "linux/amd64",
  "minimum_client_version": "v1.0.0",
  "client_version": "v1.2.0",
  "update_available": true,
  "update_required": false,
  "download_url": "https://github.com/osa911/giraffecloud/releases/latest"
}
```

## Testing the System

### 1. **Create First Release**

```bash
# Ensure all changes are committed
git add .
git commit -m "Prepare v1.0.0 release"

# Create and push tag
git tag v1.0.0
git push origin v1.0.0

# Check GitHub Actions: https://github.com/osa911/giraffecloud/actions
# Check Release: https://github.com/osa911/giraffecloud/releases
```

### 2. **Test Client Update**

```bash
# Build current client locally
go build -o giraffecloud-test cmd/giraffecloud/main.go

# Test version check
./giraffecloud-test version

# Test update check (should find GitHub release)
./giraffecloud-test update --check-only
```

### 3. **Test Auto-Update**

```bash
# Configure auto-update
./giraffecloud-test auto-update enable
./giraffecloud-test auto-update config --required-only false

# Check status
./giraffecloud-test auto-update status
```

## Configuration

### Auto-Update Settings (in ~/.giraffecloud/config.json)

```json
{
  "auto_update": {
    "enabled": true,
    "check_interval": "24h",
    "required_only": true,
    "download_url": "https://github.com/osa911/giraffecloud/releases/download",
    "preserve_connection": true,
    "restart_service": true,
    "backup_count": 5,
    "update_window": {
      "start_hour": 2,
      "end_hour": 6,
      "timezone": "UTC"
    }
  }
}
```

### Environment Variables

```bash
# Optional: Custom download URL
GIRAFFECLOUD_DOWNLOAD_URL="https://github.com/osa911/giraffecloud/releases/download"

# Optional: Disable auto-updates
GIRAFFECLOUD_DISABLE_AUTO_UPDATE=true
```

## Security Features

- **Checksum Verification**: All downloads verified against SHA256 checksums
- **Backup & Rollback**: Automatic backup of previous version before update
- **Signed Releases**: GitHub release signatures (if enabled)
- **Connection Preservation**: Minimal downtime during updates

## Production Workflow

### For You (Developer):

1. **Develop features** in feature branches
2. **Merge to main** when ready
3. **Create release tags** for versions you want to distribute
4. **GitHub handles the rest** (build, release, distribution)

### For Users (Automatic):

1. **Client checks for updates** daily (configurable)
2. **Downloads new version** if available
3. **Verifies integrity** with checksums
4. **Backs up current version**
5. **Installs update** and restarts service
6. **Restores connection** with minimal downtime

## Release Versioning

Use semantic versioning:

- `v1.0.0` - Major release
- `v1.1.0` - Minor features
- `v1.0.1` - Bug fixes
- `v1.0.0-beta.1` - Pre-releases (auto-detected)

## Monitoring & Troubleshooting

```bash
# Check update logs
tail -f ~/.giraffecloud/logs/updater.log

# View current version
giraffecloud version

# Check server compatibility
giraffecloud connect --check-only

# Manual rollback (if needed)
giraffecloud rollback
```

## Benefits

✅ **Professional**: Industry-standard approach used by major projects
✅ **Automatic**: Zero-effort updates for users
✅ **Reliable**: Checksum verification and rollback capability
✅ **Scalable**: GitHub's CDN handles global distribution
✅ **Secure**: Signed releases and integrity checks
✅ **Cross-Platform**: Support for all major platforms
✅ **Zero-Downtime**: Connection preservation during updates
