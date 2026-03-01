# daimon installer for Windows (PowerShell)
# Usage: irm https://raw.githubusercontent.com/Kishanmp3/daimon/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo       = "Kishanmp3/daimon"
$Binary     = "daimon.exe"
$InstallDir = "$env:LOCALAPPDATA\daimon"

# ── Create install directory ─────────────────────────────────────────────────
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# ── Fetch latest release ─────────────────────────────────────────────────────
Write-Host "-> Fetching latest daimon release..."

$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Tag     = $Release.tag_name
$Asset   = $Release.assets | Where-Object { $_.name -eq $Binary } | Select-Object -First 1

if (-not $Asset) {
    Write-Error "Could not find '$Binary' in release $Tag. Check https://github.com/$Repo/releases"
    exit 1
}

# ── Download ─────────────────────────────────────────────────────────────────
Write-Host "-> Downloading daimon $Tag..."

$Dest = "$InstallDir\$Binary"
Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $Dest

Write-Host "-> daimon $Tag installed at $Dest"

# ── Add to PATH (current user, permanent) ────────────────────────────────────
$CurrentPath = [System.Environment]::GetEnvironmentVariable("Path", "User")

if ($CurrentPath -notlike "*$InstallDir*") {
    Write-Host "-> Adding $InstallDir to your PATH..."
    [System.Environment]::SetEnvironmentVariable(
        "Path",
        "$CurrentPath;$InstallDir",
        "User"
    )
    # Also update the current session so `daimon` works immediately.
    $env:Path += ";$InstallDir"
}

Write-Host ""

# ── First-time setup ─────────────────────────────────────────────────────────
& $Dest summon
