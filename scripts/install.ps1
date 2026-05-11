#Requires -Version 5.1
<#
.SYNOPSIS
  Installs solactl for the current user on Windows (no admin rights required).

.DESCRIPTION
  Downloads the latest solactl release for Windows, verifies its SHA256 checksum,
  extracts the binary to %LOCALAPPDATA%\Programs\solactl, and ensures that location
  is on the user's PATH. Mirrors the behavior of scripts/install.sh for Linux/macOS.

.PARAMETER InstallDir
  Override the install directory. Defaults to %LOCALAPPDATA%\Programs\solactl.

.PARAMETER Version
  Install a specific tag (e.g. "v0.1.6") instead of the latest release.

.EXAMPLE
  irm https://raw.githubusercontent.com/solapi/solactl/main/scripts/install.ps1 | iex

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\install.ps1 -Version v0.1.6
#>

[CmdletBinding()]
param(
    [string] $InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\solactl'),
    [string] $Version = ''
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# GitHub requires TLS 1.2+; older PowerShell defaults to SSL3/TLS1.0.
try {
    [Net.ServicePointManager]::SecurityProtocol = `
        [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch {
    # PowerShell Core ignores this; safe to skip.
}

$Repo = 'solapi/solactl'

function Die([string] $Message) {
    Write-Host "ERROR: $Message" -ForegroundColor Red
    exit 1
}

function Invoke-Download([string] $Uri, [string] $OutFile) {
    try {
        Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing -MaximumRedirection 5
    } catch {
        Die "Download failed: $Uri`n$($_.Exception.Message)"
    }
}

# --- 1. Detect architecture --------------------------------------------------
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    'x86'   { Die 'Unsupported architecture: x86 (32-bit). solactl ships amd64/arm64 only.' }
    default { Die "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

# --- 2. Resolve target tag ---------------------------------------------------
if ([string]::IsNullOrWhiteSpace($Version)) {
    Write-Host 'Checking latest version...'
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    } catch {
        Die "Failed to fetch release info: $($_.Exception.Message)"
    }
    $tag = $release.tag_name
    if ([string]::IsNullOrWhiteSpace($tag)) { Die 'Failed to parse release tag.' }
} else {
    $tag = $Version
}
Write-Host "Target version: $tag"

$versionNumber = $tag.TrimStart('v')
$archiveName  = "solactl_${versionNumber}_windows_${arch}.zip"
$downloadUrl  = "https://github.com/$Repo/releases/download/$tag/$archiveName"
$checksumsUrl = "https://github.com/$Repo/releases/download/$tag/checksums.txt"

# --- 3. Download to a temp directory ----------------------------------------
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("solactl-install-" + [System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    $archivePath   = Join-Path $tmpDir $archiveName
    $checksumsPath = Join-Path $tmpDir 'checksums.txt'

    Write-Host "Downloading $archiveName..."
    Invoke-Download -Uri $downloadUrl -OutFile $archivePath

    Write-Host 'Downloading checksums...'
    Invoke-Download -Uri $checksumsUrl -OutFile $checksumsPath

    # --- 4. Verify SHA256 ----------------------------------------------------
    Write-Host 'Verifying checksum...'
    $expectedHash = $null
    foreach ($line in Get-Content -LiteralPath $checksumsPath) {
        $parts = ($line.Trim()) -split '\s+', 2
        if ($parts.Length -eq 2 -and $parts[1] -eq $archiveName) {
            $expectedHash = $parts[0].ToLowerInvariant()
            break
        }
    }
    if (-not $expectedHash) { Die "Checksum not found for $archiveName in checksums.txt" }

    $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expectedHash -ne $actualHash) {
        Die "Checksum mismatch: expected $expectedHash, got $actualHash. File may be tampered."
    }
    Write-Host 'Checksum verified.'

    # --- 5. Extract & install -----------------------------------------------
    Write-Host 'Extracting...'
    $extractDir = Join-Path $tmpDir 'extract'
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir -Force

    $binary = Join-Path $extractDir 'solactl.exe'
    if (-not (Test-Path -LiteralPath $binary)) {
        Die 'solactl.exe not found in archive.'
    }

    if (-not (Test-Path -LiteralPath $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    $dest = Join-Path $InstallDir 'solactl.exe'
    try {
        Move-Item -LiteralPath $binary -Destination $dest -Force
    } catch {
        # Existing binary is likely in use by a running process.
        # Park the old one so the install still succeeds.
        $stash = "$dest.old"
        if (Test-Path -LiteralPath $stash) { Remove-Item -LiteralPath $stash -Force }
        Move-Item -LiteralPath $dest -Destination $stash -Force
        Move-Item -LiteralPath $binary -Destination $dest -Force
        Write-Host "Previous binary was in use; moved aside to $stash"
    }

    Write-Host ''
    Write-Host "Installed solactl $tag"
    Write-Host "Location: $dest"
    Write-Host ''

    # --- 6. Ensure InstallDir is on the user PATH ---------------------------
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @()
    if ($userPath) { $entries = $userPath -split ';' | Where-Object { $_ -ne '' } }

    $alreadyOnPath = $false
    foreach ($entry in $entries) {
        if ([string]::Equals($entry.TrimEnd('\'), $InstallDir.TrimEnd('\'), [System.StringComparison]::OrdinalIgnoreCase)) {
            $alreadyOnPath = $true
            break
        }
    }

    if (-not $alreadyOnPath) {
        $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        # Update current session too so the user can run solactl without reopening the shell.
        $env:Path = "$env:Path;$InstallDir"
        Write-Host "Added $InstallDir to user PATH."
        Write-Host 'Open a new terminal for the PATH change to apply to other shells.'
    } else {
        Write-Host "$InstallDir is already on user PATH."
    }

    Write-Host ''
    Write-Host 'To upgrade later: solactl upgrade'
} finally {
    Remove-Item -LiteralPath $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
