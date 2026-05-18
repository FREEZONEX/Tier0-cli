#!/usr/bin/env bash
set -euo pipefail

# Tier0 CLI 一键安装脚本
# 用法: curl -sL https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.sh | bash

REPO="FREEZONEX/Tier0-cli"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# 检测平台
 detect_platform() {
  local os arch
  os="$(uname -s)"
  arch="$(uname -m)"

  case "$os" in
    Linux)
      case "$arch" in
        x86_64)  echo "Linux-x86_64" ;;
        aarch64|arm64) echo "Linux-aarch64" ;;
        *) echo "unsupported Linux arch: $arch" >&2; exit 1 ;;
      esac
      ;;
    Darwin)
      case "$arch" in
        x86_64)  echo "macOS-x86_64" ;;
        arm64)   echo "macOS-arm64" ;;
        *) echo "unsupported macOS arch: $arch" >&2; exit 1 ;;
      esac
      ;;
    *)
      echo "unsupported OS: $os" >&2
      exit 1
      ;;
  esac
}

PLATFORM="$(detect_platform)"
echo "[install] detected platform: $PLATFORM"

# 获取最新版本
LATEST_URL="https://api.github.com/repos/$REPO/releases/latest"
echo "[install] fetching latest release..."
VERSION="$(curl -sL "$LATEST_URL" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4)"
if [ -z "$VERSION" ]; then
  echo "[install] failed to fetch latest release, using fallback version v0.2.2"
  VERSION="v0.2.2"
fi
echo "[install] latest version: $VERSION"

# 下载
PKG_NAME="tier0-cli-${VERSION}-${PLATFORM}"
if [[ "$PLATFORM" == Windows* ]]; then
  PKG_NAME="${PKG_NAME}.zip"
else
  PKG_NAME="${PKG_NAME}.tar.gz"
fi

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$PKG_NAME"
TMP_DIR="$(mktemp -d)"
trap "rm -rf $TMP_DIR" EXIT

echo "[install] downloading $PKG_NAME..."
curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/$PKG_NAME"

# 解压
echo "[install] extracting..."
cd "$TMP_DIR"
if [[ "$PKG_NAME" == *.zip ]]; then
  unzip -q "$PKG_NAME"
else
  tar -xzf "$PKG_NAME"
fi

# 查找二进制
BINARY=""
for f in tier0 tier0.exe; do
  if [ -f "$f" ]; then
    BINARY="$f"
    break
  fi
  # 可能在子目录中
  if [ -f "*/$f" ]; then
    BINARY="*/$f"
    break
  fi
done

if [ -z "$BINARY" ]; then
  echo "[install] error: binary not found in package" >&2
  ls -la "$TMP_DIR"
  exit 1
fi

# 安装
if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "$INSTALL_DIR/tier0"
else
  echo "[install] sudo required to install to $INSTALL_DIR"
  sudo mv "$BINARY" "$INSTALL_DIR/tier0"
fi
chmod +x "$INSTALL_DIR/tier0"

echo "[install] ✓ tier0 installed to $INSTALL_DIR/tier0"
echo "[install] version: $($INSTALL_DIR/tier0 version)"
echo ""
echo "Next steps:"
echo "  tier0 config --base-url https://your-domain.com   # (私有化部署)"
echo "  tier0 login                                        # 登录授权"
