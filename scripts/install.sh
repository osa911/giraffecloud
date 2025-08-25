#!/usr/bin/env bash
set -euo pipefail

# GiraffeCloud installer
# - Installs the giraffecloud CLI for the current user (default) or system-wide (--system)
# - Optionally installs and starts the service (--service user|system)
# - Optionally logs in with a provided token (--token <API_TOKEN>)
# - If --url is omitted, tries to fetch latest release asset for the detected OS/ARCH from GitHub API

REPO_OWNER="osa911"
REPO_NAME="giraffecloud"

INSTALL_MODE="user"          # user | system
SERVICE_MODE="none"          # none | user | system
RELEASE_URL=""
LOGIN_TOKEN=""

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --system                 Install system-wide to /usr/local/bin (requires sudo). Default: user (~/.local/bin)
  --service [user|system]  Install and start the service (user-level or system-level). Default: none
  --url <tar.gz>           Release asset URL to install (skips GitHub API lookup)
  --token <API_TOKEN>      Perform 'giraffecloud login --token <API_TOKEN>' after install
  -h, --help               Show this help

Examples:
  $0 --service user
  $0 --url https://github.com/osa911/giraffecloud/releases/download/test-XXXX/giraffecloud_linux_amd64_v0.0.0-test.XXXX.tar.gz
  $0 --system --service system --token YOUR_API_TOKEN
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --system)
      INSTALL_MODE="system"; shift ;;
    --service)
      SERVICE_MODE="${2:-user}"; shift 2 ;;
    --url)
      RELEASE_URL="${2:-}"; shift 2 ;;
    --token)
      LOGIN_TOKEN="${2:-}"; shift 2 ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "Error: required command '$1' not found" >&2; exit 1; }
}

require_cmd curl
require_cmd tar

# Detect OS/ARCH
GOOS=$(uname | tr '[:upper:]' '[:lower:]')
case "$GOOS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *) echo "Unsupported OS: $GOOS" >&2; exit 1 ;;
esac

GOARCH=$(uname -m)
case "$GOARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $GOARCH" >&2; exit 1 ;;
esac

# Interactive prompts when running in a TTY and no explicit options were passed
if [[ -t 0 && -t 1 ]]; then
  # Offer to install as a system service on Linux
  if [[ "$OS" == "linux" && "$SERVICE_MODE" == "none" ]]; then
    read -r -p "Install and start GiraffeCloud as a system service now? [Y/n] " _ans || true
    case "${_ans:-Y}" in
      [nN]*) : ;;
      *) SERVICE_MODE="system"; INSTALL_MODE="system" ;;
    esac
  fi
  # Offer to capture API token if not provided
  if [[ -z "$LOGIN_TOKEN" ]]; then
    read -r -p "Provide API token now to login after install? [y/N] " _ans || true
    case "${_ans:-N}" in
      [yY]*)
        printf "Enter API token: "
        stty -echo || true
        read -r LOGIN_TOKEN || true
        stty echo || true
        printf "\n"
        ;;
    esac
  fi
fi

# If a system service is requested, ensure we install system-wide for a stable ExecStart path
if [[ "$SERVICE_MODE" == "system" && "$INSTALL_MODE" != "system" ]]; then
  echo "Note: --service system requested; switching to system-wide install at /usr/local/bin"
  INSTALL_MODE="system"
fi

# If user service is requested, warn and fall back to system service for simplicity
if [[ "$SERVICE_MODE" == "user" ]]; then
  echo "Note: user service mode is not yet fully supported by this installer; using system service instead"
  SERVICE_MODE="system"
  if [[ "$INSTALL_MODE" != "system" ]]; then
    echo "Switching to system-wide install at /usr/local/bin"
    INSTALL_MODE="system"
  fi
fi

TMPDIR="$(mktemp -d 2>/dev/null || mktemp -d -t giraffecloud)"
cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

resolve_latest_url() {
  # Try to fetch latest release asset matching current OS/ARCH
  local api="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
  # We avoid jq; use grep/sed to extract a matching tar.gz url
  local url
  url=$(curl -fsSL "$api" \
    | grep -Eo '"browser_download_url"\s*:\s*"[^"]+"' \
    | sed -E 's/"browser_download_url"\s*:\s*"(.*)"/\1/' \
    | grep "giraffecloud_${OS}_${ARCH}_.*\\.tar\\.gz" \
    | head -n1 || true)
  echo "$url"
}

if [[ -z "$RELEASE_URL" ]]; then
  echo "Resolving latest release for ${OS}/${ARCH}..."
  RELEASE_URL="$(resolve_latest_url)"
  if [[ -z "$RELEASE_URL" ]]; then
    echo "Failed to resolve latest release asset automatically. Provide --url <tar.gz>." >&2
    exit 1
  fi
fi

echo "Downloading: $RELEASE_URL"
curl -fL -o "$TMPDIR/giraffecloud.tar.gz" "$RELEASE_URL"

echo "Extracting..."
tar -xzf "$TMPDIR/giraffecloud.tar.gz" -C "$TMPDIR"

echo "Locating binary..."
BIN_PATH="$(find "$TMPDIR" -type f -name giraffecloud -perm -u+x | head -n1 || true)"
if [[ -z "$BIN_PATH" ]]; then
  echo "Error: giraffecloud binary not found in archive" >&2
  exit 1
fi

# Pre-clean: remove broken /usr/local/bin/giraffecloud symlink if present
if [[ -L "/usr/local/bin/giraffecloud" && ! -e "/usr/local/bin/giraffecloud" ]]; then
  echo "Removing broken symlink: /usr/local/bin/giraffecloud"
  sudo rm -f /usr/local/bin/giraffecloud || true
fi
# Also force-remove any existing file to avoid circular symlink issues
sudo rm -f /usr/local/bin/giraffecloud || true

# Stop existing service before overwrite (best effort)
if command -v systemctl >/dev/null 2>&1; then
  if [[ "$SERVICE_MODE" == "system" ]]; then
    sudo systemctl stop giraffecloud >/dev/null 2>&1 || true
  fi
fi

if [[ "$INSTALL_MODE" == "system" ]]; then
  echo "Installing system-wide to /usr/local/bin (requires sudo)..."
  sudo install -m 0755 "$BIN_PATH" /usr/local/bin/giraffecloud
  DEST="/usr/local/bin/giraffecloud"
else
  echo "Installing to user bin (~/.local/bin)..."
  mkdir -p "$HOME/.local/bin"
  install -m 0755 "$BIN_PATH" "$HOME/.local/bin/giraffecloud"
  DEST="$HOME/.local/bin/giraffecloud"
  # Ensure PATH at runtime
  case "${SHELL##*/}" in
    bash)
      if ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' "$HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
      fi ;;
    zsh)
      if ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' "$HOME/.zshrc" 2>/dev/null; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc"
      fi ;;
  esac
  export PATH="$HOME/.local/bin:$PATH"
fi

echo "Installed: $DEST"
"$DEST" version || true

# Optional login
if [[ -n "$LOGIN_TOKEN" ]]; then
  echo "Logging in..."
  "$DEST" login --token "$LOGIN_TOKEN"
fi

# Determine if destination directory is already in PATH of the caller's environment
case "$DEST" in
  /usr/local/bin/*)
    DEST_DIR="/usr/local/bin" ;;
  $HOME/.local/bin/*)
    DEST_DIR="$HOME/.local/bin" ;;
  *)
    DEST_DIR="$(dirname "$DEST")" ;;

esac

PATH_HAS_DEST=0
echo "$PATH" | tr ':' '\n' | grep -Fxq "$DEST_DIR" && PATH_HAS_DEST=1 || true

# Optional service install (Linux only for now)
if [[ "$SERVICE_MODE" != "none" ]]; then
  if [[ "$OS" != "linux" ]]; then
    echo "Service installation is currently supported on Linux only; skipping." >&2
  else
    echo "Installing service: $SERVICE_MODE"
    # Pre-auth sudo interactively for a smoother flow
    if [[ -t 0 && -t 1 ]]; then
      sudo -v || true
    fi
    # Always system for now
    sudo "$DEST" service install
    # After service install, some older binaries may create a self-referential symlink.
    # Force-replace /usr/local/bin/giraffecloud with the fresh binary to eliminate loops.
    sudo rm -f /usr/local/bin/giraffecloud || true
    sudo install -m 0755 "$BIN_PATH" /usr/local/bin/giraffecloud || true
    # Patch unit to ensure correct ExecStart path and required env flags
    UNIT_PATH="/etc/systemd/system/giraffecloud.service"
    if [[ -f "$UNIT_PATH" ]]; then
      sudo sed -i 's#^ExecStart=.*#ExecStart=/usr/local/bin/giraffecloud connect#' "$UNIT_PATH"
      if ! grep -q '^Environment=GIRAFFECLOUD_HOME=' "$UNIT_PATH"; then
        sudo sed -i "/^\\[Service\\]/a Environment=GIRAFFECLOUD_HOME=$HOME/.giraffecloud" "$UNIT_PATH"
      fi
      if ! grep -q '^Environment=GIRAFFECLOUD_IS_SERVICE=' "$UNIT_PATH"; then
        sudo sed -i "/^\\[Service\\]/a Environment=GIRAFFECLOUD_IS_SERVICE=1" "$UNIT_PATH"
      fi
    fi
    # Apply and start
    sudo systemctl daemon-reload || true
    sudo systemctl enable giraffecloud || true
    sudo systemctl restart giraffecloud || true
  fi
fi

echo
if [[ $PATH_HAS_DEST -eq 1 ]]; then
  echo "Success: Installed to $DEST and it's already on your PATH."
  echo "Try: giraffecloud version"
else
  echo "Success: Installed to $DEST."
  echo "It looks like $DEST_DIR is not on your PATH in this shell."
  if [[ "$INSTALL_MODE" == "system" ]]; then
    echo "Add /usr/local/bin to your PATH or open a new terminal."
  else
    case "${SHELL##*/}" in
      zsh)
        echo "Add this to your ~/.zshrc then run 'source ~/.zshrc':"
        echo "  export PATH=\"$HOME/.local/bin:$PATH\"" ;;
      bash)
        echo "Add this to your ~/.bashrc then run 'source ~/.bashrc':"
        echo "  export PATH=\"$HOME/.local/bin:$PATH\"" ;;
      *)
        echo "Add $HOME/.local/bin to your PATH and restart your shell:"
        echo "  export PATH=\"$HOME/.local/bin:$PATH\"" ;;
    esac
  fi
fi

echo
echo "Next steps:"
echo "  giraffecloud config path"
echo "  giraffecloud connect"


