## Installation

This guide shows how to install the GiraffeCloud CLI on macOS, Linux, and Windows with a single line.

### Quick install

macOS/Linux (Bash):

```bash
curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash
```

Windows (PowerShell):

```powershell
iwr -useb https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1 | iex
```

After installation, you can verify:

```bash
giraffecloud version
```

### Optional parameters

macOS/Linux script (`install.sh`):

- **--system**: Install system-wide to `/usr/local/bin` (requires sudo). Default: user install to `~/.local/bin`.
- **--service [user|system]**: Install and start the service using systemd (Linux only). Default: none.
- **--url <tar.gz>**: Install from a specific release asset URL instead of auto-detecting the latest.
- **--token <API_TOKEN>**: Run `giraffecloud login --token <API_TOKEN>` after install.

Example:

```bash
curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash -s -- --service user --token YOUR_API_TOKEN
```

Windows script (`install.ps1`):

- **-System**: Install system-wide to `C:\\Program Files\\GiraffeCloud\\bin` (requires elevated PowerShell). Default: user install to `%LOCALAPPDATA%\giraffecloud\bin`.
- **-Url <zip|tar.gz>**: Install from a specific release asset URL instead of auto-detecting the latest.
- **-Token <API_TOKEN>**: Run `giraffecloud login --token <API_TOKEN>` after install.

Example:

```powershell
iex "& { $(iwr -useb https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1) } -System -Token 'YOUR_API_TOKEN'"
```

### PATH updates and shell refresh

- macOS/Linux user install goes to `~/.local/bin`. If it is not on your `PATH`, the installer will add it to your shell rc (`~/.zshrc` or `~/.bashrc`) and prompt you to `source` it or open a new terminal.
- System installs go to `/usr/local/bin`, which is typically already on `PATH`.
- Windows installer adds the chosen install directory to the User or Machine `PATH` and broadcasts the change so new terminals pick it up. You may need to open a new terminal.

### Service installation (Linux only)

When `--service user|system` is provided on Linux, the installer invokes `giraffecloud service install` and will attempt to restart the systemd unit (`giraffecloud`). This is skipped on macOS and Windows.

### Install from a specific release

- macOS/Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash -s -- --url https://github.com/osa911/giraffecloud/releases/download/vX.Y.Z/giraffecloud_linux_amd64_vX.Y.Z.tar.gz
```

- Windows (PowerShell):

```powershell
iex "& { $(iwr -useb https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1) } -Url 'https://github.com/osa911/giraffecloud/releases/download/vX.Y.Z/giraffecloud_windows_amd64_vX.Y.Z.zip'"
```

Asset naming convention (examples):

- `giraffecloud_linux_amd64_vX.Y.Z.tar.gz`
- `giraffecloud_darwin_arm64_vX.Y.Z.tar.gz`
- `giraffecloud_windows_amd64_vX.Y.Z.zip`

### Uninstall

macOS/Linux (user install):

```bash
rm -f ~/.local/bin/giraffecloud
# Optionally remove the PATH line from ~/.zshrc or ~/.bashrc if you added it manually
```

macOS/Linux (system install):

```bash
sudo rm -f /usr/local/bin/giraffecloud
```

Windows:

```powershell
Remove-Item -LiteralPath "$env:LOCALAPPDATA\giraffecloud\bin\giraffecloud.exe" -ErrorAction SilentlyContinue
# Or, if installed system-wide:
# Remove-Item -LiteralPath 'C:\\Program Files\\GiraffeCloud\\bin\\giraffecloud.exe' -ErrorAction SilentlyContinue
```

### Troubleshooting

- "command not found" after install:
  - Open a new terminal to pick up `PATH` changes
  - Ensure `~/.local/bin` (Linux/macOS user install) is in your `PATH`
  - On Windows, verify the install directory exists and is in `PATH`
- Service install skipped on macOS:
  - This is expected. Service installation is supported on Linux (systemd) only.
- Corporate proxy / SSL interception:
  - Download the release archive manually and pass `--url` (Linux/macOS) or `-Url` (Windows) to the installer.

### Security note

The one-liner uses `curl | bash` (or PowerShell `iwr | iex`). Review the script before running by opening its URL in a browser.
