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
    throw "unable to determine the latest Tier0 CLI version: $($_.Exception.Message)"
}
if (-not $Version) {
    throw "unable to determine the latest Tier0 CLI version"
}

# Download.
$PkgName = "tier0-cli-$Version-$Platform.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$PkgName"
$TempDir = [System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid().ToString()
New-Item -ItemType Directory -Path $TempDir | Out-Null

Invoke-RestMethod -Uri $DownloadUrl -OutFile "$TempDir\$PkgName"

# Verify the release artifact before executing it.
$SumsUrl = "https://github.com/$Repo/releases/download/$Version/sha256sums.txt"
$SumsPath = "$TempDir\sha256sums.txt"
Invoke-RestMethod -Uri $SumsUrl -OutFile $SumsPath
$ChecksumLine = Get-Content $SumsPath | Where-Object {
    $Parts = $_.Trim() -split '\s+'
    $Parts.Count -ge 2 -and $Parts[-1].TrimStart('*') -eq $PkgName
} | Select-Object -First 1
if (-not $ChecksumLine) {
    throw "checksum not found for $PkgName"
}
$ExpectedSha = ($ChecksumLine -split '\s+')[0].ToLowerInvariant()
$ActualSha = (Get-FileHash -Algorithm SHA256 -Path "$TempDir\$PkgName").Hash.ToLowerInvariant()
if ($ActualSha -ne $ExpectedSha) {
    throw "SHA256 verification failed for $PkgName"
}

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

# Materialize the trusted Skill compiled into the verified CLI binary.
& "$InstallDir\tier0.exe" skills install --no-sync
if ($LASTEXITCODE -ne 0) {
    throw "embedded Tier0 Skill installation failed"
}

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
if (Get-Command npx -ErrorAction SilentlyContinue) {
    & "$InstallDir\tier0.exe" skills sync
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "The local Tier0 Skill is ready, but Agent Skills sync failed. Retry with: tier0 skills sync"
    }
} else {
    Write-Warning "npx was not found. Run 'tier0 skills sync' after installing Node.js."
}
Write-Host ""
Write-Host "Next: tier0 login"

# Clean up.
Remove-Item -Recurse -Force $TempDir
