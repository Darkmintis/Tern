# Tern install script — downloads the latest GitHub Release binary.
# Usage: iwr -useb https://raw.githubusercontent.com/darkmintis/Tern/main/install/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = if ($env:TERN_REPO) { $env:TERN_REPO } else { "darkmintis/Tern" }
$installDir = if ($env:TERN_INSTALL_DIR) { $env:TERN_INSTALL_DIR } else { "$env:LOCALAPPDATA\Tern\bin" }
$version = if ($env:TERN_VERSION) { $env:TERN_VERSION } else { "latest" }

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "amd64" }
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { $arch = "arm64" }

$asset = "tern-windows-${arch}.exe"

if ($version -eq "latest") {
    $url = "https://github.com/$repo/releases/latest/download/$asset"
} else {
    $url = "https://github.com/$repo/releases/download/$version/$asset"
}

# Create install directory
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

# Download
Write-Host "Downloading $url"
$dest = "$installDir\tern.exe"
Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing

# Add to PATH if not already there
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
    Write-Host "Added $installDir to PATH"
    Write-Host "Restart your terminal or run: `$env:Path += ';$installDir'"
}

Write-Host "Installed tern to $dest"
Write-Host "Run 'tern version' to verify"
