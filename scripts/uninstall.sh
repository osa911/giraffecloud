#!/usr/bin/env bash
set -euo pipefail

# GiraffeCloud uninstaller
# - Removes the giraffecloud CLI binary
# - Stops and removes the systemd service (if installed)
# - Optionally removes config and data files

REPO_NAME="giraffecloud"
REMOVE_DATA="false"

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --remove-data    Also remove configuration and data directory (~/.giraffecloud)
  -h, --help       Show this help

Examples:
  $0                      # Remove binary and service only
  $0 --remove-data        # Remove everything including configs
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remove-data)
      REMOVE_DATA="true"; shift ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  GiraffeCloud Uninstaller"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# Detect OS
GOOS=$(uname | tr '[:upper:]' '[:lower:]')
case "$GOOS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *) echo "Unsupported OS: $GOOS" >&2; exit 1 ;;
esac

# 1. Stop and remove systemd service (Linux only)
if [[ "$OS" == "linux" ]] && command -v systemctl >/dev/null 2>&1; then
  SERVICE_FILE="/etc/systemd/system/giraffecloud.service"
  if [[ -f "$SERVICE_FILE" ]]; then
    echo "🛑 Stopping and disabling service..."
    sudo systemctl stop giraffecloud 2>/dev/null || true
    sudo systemctl disable giraffecloud 2>/dev/null || true

    echo "🗑️  Removing service file..."
    sudo rm -f "$SERVICE_FILE"
    sudo systemctl daemon-reload
    echo "✅ Service removed"
  else
    echo "ℹ️  No systemd service found"
  fi
fi

# 2. Remove binary from system-wide location
REMOVED_BINARY=false
if [[ -f "/usr/local/bin/giraffecloud" ]]; then
  echo "🗑️  Removing system-wide binary: /usr/local/bin/giraffecloud"
  sudo rm -f /usr/local/bin/giraffecloud
  REMOVED_BINARY=true
  echo "✅ System binary removed"
fi

# 3. Remove binary from user location
if [[ -f "$HOME/.local/bin/giraffecloud" ]]; then
  echo "🗑️  Removing user binary: $HOME/.local/bin/giraffecloud"
  rm -f "$HOME/.local/bin/giraffecloud"
  REMOVED_BINARY=true
  echo "✅ User binary removed"
fi

if [[ "$REMOVED_BINARY" == false ]]; then
  echo "⚠️  No giraffecloud binary found in standard locations"
  echo "   Checked: /usr/local/bin/giraffecloud, $HOME/.local/bin/giraffecloud"
fi

# 4. Optionally remove data directory
if [[ "$REMOVE_DATA" == "true" ]]; then
  if [[ -d "$HOME/.giraffecloud" ]]; then
    echo "🗑️  Removing configuration and data directory: $HOME/.giraffecloud"
    rm -rf "$HOME/.giraffecloud"
    echo "✅ Data directory removed"
  else
    echo "ℹ️  No data directory found at $HOME/.giraffecloud"
  fi
else
  if [[ -d "$HOME/.giraffecloud" ]]; then
    echo "ℹ️  Configuration preserved at: $HOME/.giraffecloud"
    echo "   Run with --remove-data to remove it"
  fi
fi

# 5. Inform about PATH modifications (don't auto-remove to avoid breaking shell configs)
echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📝 Note: PATH modifications (if any) were NOT removed"
echo
if [[ -f "$HOME/.bashrc" ]] && grep -q '.local/bin' "$HOME/.bashrc" 2>/dev/null; then
  echo "   Found PATH entry in ~/.bashrc"
fi
if [[ -f "$HOME/.zshrc" ]] && grep -q '.local/bin' "$HOME/.zshrc" 2>/dev/null; then
  echo "   Found PATH entry in ~/.zshrc"
fi
echo
echo "   To remove PATH modifications manually, edit your shell"
echo "   config file and remove lines containing:"
echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

if [[ "$REMOVED_BINARY" == true ]]; then
  echo "✅ GiraffeCloud has been uninstalled successfully!"
else
  echo "⚠️  No GiraffeCloud installation found"
  echo "   The tool may have already been removed or was not installed"
fi
echo


