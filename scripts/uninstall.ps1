# GiraffeCloud Windows Uninstaller
# Requires Administrator privileges for service removal

param(
    [switch]$RemoveData = $false,
    [switch]$Help = $false
)

function Show-Usage {
    Write-Host @"
GiraffeCloud Uninstaller for Windows

Usage: .\uninstall.ps1 [options]

Options:
  -RemoveData    Also remove configuration and data directory
  -Help          Show this help message

Examples:
  .\uninstall.ps1                # Remove binary and service only
  .\uninstall.ps1 -RemoveData    # Remove everything including configs

Note: Run as Administrator to remove Windows Service
"@
}

if ($Help) {
    Show-Usage
    exit 0
}

# Check for Administrator privileges
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Host "  GiraffeCloud Uninstaller for Windows"
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Host ""

$removedBinary = $false

# 1. Stop and remove Windows Service
$serviceName = "GiraffeCloud"
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue

if ($service) {
    if (-not $isAdmin) {
        Write-Host "⚠️  Warning: Administrator privileges required to remove service" -ForegroundColor Yellow
        Write-Host "   Please run PowerShell as Administrator" -ForegroundColor Yellow
        Write-Host ""
    } else {
        Write-Host "🛑 Stopping and removing service..." -ForegroundColor Cyan

        # Stop the service if running
        if ($service.Status -eq 'Running') {
            Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
            Write-Host "   Service stopped" -ForegroundColor Gray
        }

        # Remove the service
        sc.exe delete $serviceName | Out-Null
        Write-Host "✅ Service removed" -ForegroundColor Green
    }
} else {
    Write-Host "ℹ️  No Windows service found" -ForegroundColor Gray
}

# 2. Remove binary from Program Files
$programFilesPath = "$env:ProgramFiles\GiraffeCloud\giraffecloud.exe"
if (Test-Path $programFilesPath) {
    if (-not $isAdmin) {
        Write-Host "⚠️  Warning: Administrator privileges required to remove from Program Files" -ForegroundColor Yellow
        Write-Host "   Please run PowerShell as Administrator" -ForegroundColor Yellow
        Write-Host ""
    } else {
        Write-Host "🗑️  Removing binary: $programFilesPath" -ForegroundColor Cyan
        Remove-Item -Path "$env:ProgramFiles\GiraffeCloud" -Recurse -Force -ErrorAction SilentlyContinue
        $removedBinary = $true
        Write-Host "✅ Program Files binary removed" -ForegroundColor Green
    }
}

# 3. Remove binary from user's local AppData
$localAppDataPath = "$env:LOCALAPPDATA\GiraffeCloud\giraffecloud.exe"
if (Test-Path $localAppDataPath) {
    Write-Host "🗑️  Removing binary: $localAppDataPath" -ForegroundColor Cyan
    Remove-Item -Path "$env:LOCALAPPDATA\GiraffeCloud" -Recurse -Force -ErrorAction SilentlyContinue
    $removedBinary = $true
    Write-Host "✅ Local AppData binary removed" -ForegroundColor Green
}

# 4. Remove from PATH (user and system)
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$systemPath = [Environment]::GetEnvironmentVariable("Path", "Machine")

$pathsToRemove = @(
    "$env:ProgramFiles\GiraffeCloud",
    "$env:LOCALAPPDATA\GiraffeCloud"
)

$userPathModified = $false
foreach ($pathToRemove in $pathsToRemove) {
    if ($userPath -like "*$pathToRemove*") {
        $userPath = ($userPath.Split(';') | Where-Object { $_ -ne $pathToRemove }) -join ';'
        $userPathModified = $true
    }
}

if ($userPathModified) {
    [Environment]::SetEnvironmentVariable("Path", $userPath, "User")
    Write-Host "✅ Removed from user PATH" -ForegroundColor Green
}

# System PATH requires admin
if ($isAdmin) {
    $systemPathModified = $false
    foreach ($pathToRemove in $pathsToRemove) {
        if ($systemPath -like "*$pathToRemove*") {
            $systemPath = ($systemPath.Split(';') | Where-Object { $_ -ne $pathToRemove }) -join ';'
            $systemPathModified = $true
        }
    }

    if ($systemPathModified) {
        [Environment]::SetEnvironmentVariable("Path", $systemPath, "Machine")
        Write-Host "✅ Removed from system PATH" -ForegroundColor Green
    }
}

# 5. Optionally remove data directory
$dataDir = "$env:USERPROFILE\.giraffecloud"
if ($RemoveData) {
    if (Test-Path $dataDir) {
        Write-Host "🗑️  Removing configuration and data directory: $dataDir" -ForegroundColor Cyan
        Remove-Item -Path $dataDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "✅ Data directory removed" -ForegroundColor Green
    } else {
        Write-Host "ℹ️  No data directory found at $dataDir" -ForegroundColor Gray
    }
} else {
    if (Test-Path $dataDir) {
        Write-Host "ℹ️  Configuration preserved at: $dataDir" -ForegroundColor Gray
        Write-Host "   Run with -RemoveData to remove it" -ForegroundColor Gray
    }
}

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if ($removedBinary) {
    Write-Host "✅ GiraffeCloud has been uninstalled successfully!" -ForegroundColor Green
} else {
    Write-Host "⚠️  No GiraffeCloud installation found" -ForegroundColor Yellow
    Write-Host "   The tool may have already been removed or was not installed" -ForegroundColor Gray
}

if (-not $isAdmin) {
    Write-Host ""
    Write-Host "💡 Tip: Run as Administrator to remove service and system-wide files" -ForegroundColor Yellow
}

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
Write-Host ""




