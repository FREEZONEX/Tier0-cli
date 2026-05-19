# Tier0 CLI 一键安装脚本 (Windows PowerShell)
# 用法: Invoke-RestMethod -Uri https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.ps1 | Invoke-Expression

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

# 获取最新版本
$LatestUrl = "https://api.github.com/repos/$Repo/releases/latest"
try {
    $Release = Invoke-RestMethod -Uri $LatestUrl -TimeoutSec 10
    $Version = $Release.tag_name
} catch {
    $Version = "v0.2.6"
}

# 下载
$PkgName = "tier0-cli-$Version-$Platform.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$PkgName"
$TempDir = [System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid().ToString()
New-Item -ItemType Directory -Path $TempDir | Out-Null

Invoke-RestMethod -Uri $DownloadUrl -OutFile "$TempDir\$PkgName"

# 解压
Expand-Archive -Path "$TempDir\$PkgName" -DestinationPath $TempDir -Force

# 查找二进制
$Binary = Get-ChildItem -Path $TempDir -Recurse -Filter "tier0.exe" | Select-Object -First 1
if (-not $Binary) {
    throw "binary not found in package"
}

# 安装
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}
Copy-Item -Path $Binary.FullName -Destination "$InstallDir\tier0.exe" -Force

# 添加到 PATH（注册表 + 当前 session）
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
}
# 当前 session 立即生效
if ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$InstallDir;$env:Path"
}

Write-Host "tier0 $Version installed to $InstallDir\tier0.exe"
Write-Host ""
Write-Host "Next: tier0 login"

# 清理
Remove-Item -Recurse -Force $TempDir
