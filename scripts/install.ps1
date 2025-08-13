# GiraffeCloud Windows PowerShell installer
# - Installs the giraffecloud CLI for the current user (default) or system-wide (-System)
# - Optionally logs in with a provided token (-Token <API_TOKEN>)
# - If -Url is omitted, fetches the latest release asset for Windows/ARCH from GitHub API

[CmdletBinding(PositionalBinding=$false)]
param(
  [string]$Url,
  [string]$Token,
  [switch]$System
)

$ErrorActionPreference = 'Stop'

$Owner = 'osa911'
$Repo  = 'giraffecloud'

function Write-Info($msg) { Write-Host $msg -ForegroundColor Cyan }
function Write-Err($msg)  { Write-Host $msg -ForegroundColor Red }
function Is-Admin {
  $currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
  return $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

function Ensure-Tls12 {
  try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
  } catch { }
}

function Get-Arch {
  switch -Regex ($env:PROCESSOR_ARCHITECTURE) {
    'ARM64' { 'arm64'; break }
    'AMD64' { 'amd64'; break }
    default { throw "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
  }
}

function Resolve-LatestUrl {
  param([string]$arch)
  Ensure-Tls12
  $api = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
  $release = Invoke-RestMethod -UseBasicParsing -Uri $api
  # Prefer .zip assets for Windows
  $asset = $release.assets |
    Where-Object { $_.browser_download_url -match "giraffecloud_windows_${arch}_.*\.zip$" } |
    Select-Object -First 1
  if (-not $asset) {
    # Fallback: try .tar.gz (in case assets are not zipped for Windows)
    $asset = $release.assets |
      Where-Object { $_.browser_download_url -match "giraffecloud_windows_${arch}_.*\.tar\.gz$" } |
      Select-Object -First 1
  }
  if (-not $asset) { throw "Could not find a matching Windows $arch asset in latest release." }
  return $asset.browser_download_url
}

function Add-ToPathIfMissing {
  param([string]$dir, [ValidateSet('User','Machine')] [string]$scope)

  $sep = ';'
  $currentUserPath   = [Environment]::GetEnvironmentVariable('Path','User')
  $currentMachinePath= [Environment]::GetEnvironmentVariable('Path','Machine')
  $pathForScope = if ($scope -eq 'Machine') { $currentMachinePath } else { $currentUserPath }

  $hasDir = ($pathForScope -split ';') | Where-Object { $_ -ieq $dir } | ForEach-Object { $true } | Select-Object -First 1
  if (-not $hasDir) {
    $newPath = if ([string]::IsNullOrEmpty($pathForScope)) { $dir } else { "$pathForScope$sep$dir" }
    [Environment]::SetEnvironmentVariable('Path', $newPath, $scope)
  }

  # Ensure current session picks it up immediately
  if (-not (($env:Path -split ';') -contains $dir)) {
    $env:Path = "$dir;$env:Path"
  }

  try {
    # Broadcast change so new processes pick up updated PATH
    $signature = @'
using System;
using System.Runtime.InteropServices;
public class EnvBroadcaster {
  [DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
  public static extern IntPtr SendMessageTimeout(IntPtr hWnd, int Msg, IntPtr wParam, string lParam, int fuFlags, int uTimeout, out IntPtr lpdwResult);
}
'@
    Add-Type -TypeDefinition $signature -ErrorAction SilentlyContinue | Out-Null
    $HWND_BROADCAST = [IntPtr]0xffff
    $WM_SETTINGCHANGE = 0x1A
    [IntPtr]$result = [IntPtr]::Zero
    [void][EnvBroadcaster]::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [IntPtr]::Zero, 'Environment', 0x0002, 5000, [ref]$result)
  } catch { }
}

try {
  $arch = Get-Arch

  if ([string]::IsNullOrWhiteSpace($Url)) {
    Write-Info "Resolving latest release for windows/$arch..."
    $Url = Resolve-LatestUrl -arch $arch
  }

  $tmp = Join-Path $env:TEMP ("giraffecloud_" + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Path $tmp | Out-Null
  $cleanup = {
    try { Remove-Item -Recurse -Force -LiteralPath $tmp -ErrorAction SilentlyContinue } catch {}
  }
  Register-EngineEvent PowerShell.Exiting -Action $cleanup | Out-Null

  $fileName = if ($Url -match '\.zip$') { 'giraffecloud.zip' } elseif ($Url -match '\.tar\.gz$') { 'giraffecloud.tar.gz' } else { 'giraffecloud.bin' }
  $archivePath = Join-Path $tmp $fileName

  Write-Info "Downloading: $Url"
  Ensure-Tls12
  Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $archivePath

  Write-Info 'Extracting...'
  if ($archivePath -like '*.zip') {
    Expand-Archive -LiteralPath $archivePath -DestinationPath $tmp -Force
  } elseif ($archivePath -like '*.tar.gz') {
    # Try tar if available (Windows 10+ ships bsdtar under tar.exe)
    & tar -xzf $archivePath -C $tmp
  } else {
    throw 'Unknown archive format. Expected .zip or .tar.gz'
  }

  Write-Info 'Locating binary...'
  $exe = Get-ChildItem -Path $tmp -Recurse -Filter 'giraffecloud.exe' -ErrorAction SilentlyContinue | Select-Object -First 1
  if (-not $exe) { throw 'giraffecloud.exe not found in archive' }

  $isAdmin = Is-Admin
  if ($System) {
    if (-not $isAdmin) {
      Write-Err 'System-wide install requested but PowerShell is not elevated. Falling back to user install.'
      $System = $false
    }
  }

  if ($System) {
    $destDir = 'C:\\Program Files\\GiraffeCloud\\bin'
  } else {
    $destDir = Join-Path $env:LOCALAPPDATA 'giraffecloud\\bin'
  }

  New-Item -ItemType Directory -Force -Path $destDir | Out-Null
  $dest = Join-Path $destDir 'giraffecloud.exe'
  Copy-Item -LiteralPath $exe.FullName -Destination $dest -Force

  $scope = if ($System) { 'Machine' } else { 'User' }
  Add-ToPathIfMissing -dir $destDir -scope $scope

  Write-Host "Installed: $dest" -ForegroundColor Green

  if ($Token) {
    Write-Info 'Logging in...'
    & $dest login --token $Token
  }

  Write-Host ''
  Write-Host 'Success. You can now run:'
  Write-Host '  giraffecloud.exe version'
  Write-Host '  giraffecloud.exe config path'
  Write-Host '  giraffecloud.exe connect'
  Write-Host ''
  Write-Host 'If the command is not found, open a new terminal window to pick up PATH changes.'

} catch {
  Write-Err $_.Exception.Message
  exit 1
}


