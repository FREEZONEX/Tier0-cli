# Tier0 CLI one-command installer (Windows PowerShell).
# Usage: Invoke-RestMethod -Uri https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.ps1 | Invoke-Expression

$Repo = "FREEZONEX/Tier0-cli"
$InstallDir = if ($env:TIER0_INSTALL_DIR) { $env:TIER0_INSTALL_DIR } else { "$env:LOCALAPPDATA\tier0" }

function Detect-Platform {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLower()
    switch ($arch) {
        "x64"     { return "Windows-x86_64" }
        "arm64"   { return "Windows-arm64" }
        default   { throw "unsupported Windows arch: $arch" }
    }
}

$Platform = Detect-Platform

# Fetch latest version.
$LatestUrl = "https://api.github.com/repos/$Repo/releases/latest"
try {
    $Release = Invoke-RestMethod -Uri $LatestUrl -TimeoutSec 10
    $Version = $Release.tag_name
} catch {
    $Version = "v0.2.7"
}

# Download.
$PkgName = "tier0-cli-$Version-$Platform.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$PkgName"
$TempDir = [System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid().ToString()
New-Item -ItemType Directory -Path $TempDir | Out-Null

Invoke-RestMethod -Uri $DownloadUrl -OutFile "$TempDir\$PkgName"

# Extract.
Expand-Archive -Path "$TempDir\$PkgName" -DestinationPath $TempDir -Force

# Find binary.
$Binary = Get-ChildItem -Path $TempDir -Recurse -Filter "tier0.exe" | Select-Object -First 1
if (-not $Binary) {
    throw "binary not found in package"
}

# Install.
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}
Copy-Item -Path $Binary.FullName -Destination "$InstallDir\tier0.exe" -Force

# Add to PATH in the registry and current session.
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
}
# Make it available in the current session.
if ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$InstallDir;$env:Path"
}

Write-Host "tier0 $Version installed to $InstallDir\tier0.exe"
Write-Host ""
Write-Host "Next: tier0 login"

# Clean up.
Remove-Item -Recurse -Force $TempDir
