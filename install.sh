#!/usr/bin/env bash
set -euo pipefail

# Tier0 CLI one-command installer.
# Usage: curl -sL https://raw.githubusercontent.com/FREEZONEX/Tier0-cli/main/install.sh | bash

REPO="FREEZONEX/Tier0-cli"

# Install into the user directory by default; sudo is not required.
INSTALL_DIR="${INSTALL_DIR:-$HOME/.tier0/bin}"

# Detect platform.
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

# Fetch latest version.
LATEST_URL="https://api.github.com/repos/$REPO/releases/latest"
VERSION="$(curl -sL "$LATEST_URL" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4)"
if [ -z "$VERSION" ]; then
  echo "error: unable to determine the latest Tier0 CLI version" >&2
  exit 1
fi

# Download.
PKG_NAME="tier0-cli-${VERSION}-${PLATFORM}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$PKG_NAME"
TMP_DIR="$(mktemp -d)"
trap "rm -rf $TMP_DIR" EXIT

curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$PKG_NAME"

# Verify the release artifact before executing it.
SUMS_URL="https://github.com/$REPO/releases/download/$VERSION/sha256sums.txt"
curl -fsSL "$SUMS_URL" -o "$TMP_DIR/sha256sums.txt"
EXPECTED_SHA="$(awk -v name="$PKG_NAME" '$2 == name || $2 == "*" name { print $1; exit }' "$TMP_DIR/sha256sums.txt")"
if [ -z "$EXPECTED_SHA" ]; then
  echo "error: checksum not found for $PKG_NAME" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SHA="$(sha256sum "$TMP_DIR/$PKG_NAME" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL_SHA="$(shasum -a 256 "$TMP_DIR/$PKG_NAME" | awk '{print $1}')"
else
  echo "error: sha256sum or shasum is required to verify the download" >&2
  exit 1
fi
if [ "$ACTUAL_SHA" != "$EXPECTED_SHA" ]; then
  echo "error: SHA256 verification failed for $PKG_NAME" >&2
  exit 1
fi

# Extract.
tar -xzf "$TMP_DIR/$PKG_NAME" -C "$TMP_DIR"

# Find binary.
BINARY=""
for f in tier0 tier0.exe; do
  if [ -f "$TMP_DIR/$f" ]; then
    BINARY="$TMP_DIR/$f"
    break
  fi
  found="$(find "$TMP_DIR" -maxdepth 2 -name "$f" -print -quit 2>/dev/null || true)"
  if [ -n "$found" ]; then
    BINARY="$found"
    break
  fi
done

if [ -z "$BINARY" ]; then
  echo "error: binary not found in package" >&2
  exit 1
fi

# Install into the user directory.
mkdir -p "$INSTALL_DIR"
cp "$BINARY" "$INSTALL_DIR/tier0"
chmod +x "$INSTALL_DIR/tier0"

# Materialize the trusted Skill compiled into the verified CLI binary. This is
# required even though release archives no longer contain a duplicate skill/.
"$INSTALL_DIR/tier0" skills install --no-sync

# PATH configuration.
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  # Detect current shell.
  CURRENT_SHELL="${SHELL##*/}"
  SHELL_RC=""
  case "$CURRENT_SHELL" in
    zsh)  SHELL_RC="$HOME/.zshrc" ;;
    bash) SHELL_RC="$HOME/.bashrc" ;;
    *)    SHELL_RC="$HOME/.profile" ;;
  esac

  # Append to shell profile.
  if [ -f "$SHELL_RC" ]; then
    if ! grep -q "export PATH=.*$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
      echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$SHELL_RC"
    fi
  fi

  # Make it available in the current shell.
  export PATH="$INSTALL_DIR:$PATH"
fi

echo "tier0 $VERSION installed to $INSTALL_DIR/tier0"
if command -v npx >/dev/null 2>&1; then
  if ! "$INSTALL_DIR/tier0" skills sync; then
    echo "warning: the local Tier0 Skill is ready, but Agent Skills sync failed" >&2
    echo "         retry with: tier0 skills sync" >&2
  fi
else
  echo "warning: npx was not found; run 'tier0 skills sync' after installing Node.js" >&2
fi
echo ""
echo "Next: tier0 login"
