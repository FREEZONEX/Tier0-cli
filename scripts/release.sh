#!/usr/bin/env bash
set -euo pipefail

# Tier0 CLI cross-platform release build script
# Usage: bash scripts/release.sh [VERSION]
# Example: bash scripts/release.sh v0.2.0
#
# Environment variables (recommended in .env; loaded automatically):
#   GITHUB_TOKEN  - GitHub Personal Access Token
#   PARALLEL      - parallel build count (default 8)
#   BUILD_ONLY    - set to 1 to build and verify packages without publishing
#   TARGET_PLATFORMS - optional space-separated GOOS/GOARCH list for local tests

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Load .env automatically when present.
if [[ -f "${ROOT}/.env" ]]; then
  # shellcheck source=/dev/null
  source "${ROOT}/.env"
fi

VERSION="${1:-v0.1.0}"
BUILD_DIR="${ROOT}/dist/release-${VERSION}"
RELEASE_DIR="${BUILD_DIR}/packages"
PARALLEL="${PARALLEL:-8}"
BUILD_ONLY="${BUILD_ONLY:-0}"
TARGET_PLATFORMS="${TARGET_PLATFORMS:-}"

# Skill source directory, normally the sibling Tier0-skill checkout.
# SKILL_SRC can override this in release automation.
SKILL_SRC="${SKILL_SRC:-${ROOT}/../Tier0-skill}"
if [[ ! -f "${SKILL_SRC}/SKILL.md" && -f "${ROOT}/../skill/SKILL.md" ]]; then
  SKILL_SRC="${ROOT}/../skill"
fi
if [[ -f "${SKILL_SRC}/SKILL.md" ]]; then
  # When the source checkout is available, refuse to release stale embedded
  # content. Sync and review it explicitly before running this script.
  SKILL_SRC="${SKILL_SRC}" bash "${ROOT}/scripts/sync-embedded-skill.sh" --check
elif [[ -f "${ROOT}/internal/embeddedskill/content/SKILL.md" ]]; then
  echo "[release] external Skill source not found; using checked-in embedded baseline"
else
  echo "[release] error: neither external nor embedded Tier0 Skill content was found" >&2
  exit 1
fi

# Excluded platforms that are not native or not needed.
EXCLUDE_PLATFORMS="js/wasm wasip1/wasm android/386 android/amd64 android/arm android/arm64 ios/amd64 ios/arm64"

# Platform to release asset name mapping.
platform_name() {
  local goos="$1" goarch="$2"
  case "${goos}/${goarch}" in
    linux/amd64)   echo "Linux-x86_64" ;;
    linux/arm64)   echo "Linux-aarch64" ;;
    linux/386)     echo "Linux-i386" ;;
    linux/arm)     echo "Linux-armv7" ;;
    darwin/amd64)  echo "macOS-x86_64" ;;
    darwin/arm64)  echo "macOS-arm64" ;;
    windows/amd64) echo "Windows-x86_64" ;;
    windows/arm64) echo "Windows-arm64" ;;
    freebsd/amd64) echo "FreeBSD-x86_64" ;;
    freebsd/arm64) echo "FreeBSD-aarch64" ;;
    *)             echo "${goos}-${goarch}" ;;
  esac
}

# Check whether the platform should be excluded.
is_excluded() {
  local platform="$1"
  for ex in $EXCLUDE_PLATFORMS; do
    if [[ "$platform" == "$ex" ]]; then
      return 0
    fi
  done
  return 1
}

echo "========================================"
echo "  Tier0 CLI Release Builder"
echo "  Version: ${VERSION}"
echo "  Parallel: ${PARALLEL}"
echo "========================================"
echo ""

# Ensure the version starts with v.
if [[ ! "$VERSION" =~ ^v ]]; then
  VERSION="v${VERSION}"
fi

# Create output directory.
rm -rf "${BUILD_DIR}"
mkdir -p "${RELEASE_DIR}"

# Get requested or native platforms, excluding wasm/android/ios/js.
PLATFORMS=()
platform_source="$(go tool dist list | sort)"
if [[ -n "${TARGET_PLATFORMS}" ]]; then
  platform_source="$(printf '%s\n' ${TARGET_PLATFORMS})"
fi
while IFS='/' read -r goos goarch; do
	[[ -z "${goos}" || -z "${goarch}" ]] && continue
  platform="${goos}/${goarch}"
  if is_excluded "$platform"; then
    echo "[skip] ${platform} (excluded)"
    continue
  fi
  PLATFORMS+=("${goos}:${goarch}")
done <<< "${platform_source}"

echo ""
echo "Preparing ${#PLATFORMS[@]} platform builds, parallelism: ${PARALLEL}"
echo ""

# Build one platform.
build_one() {
  local goos="$1" goarch="$2"
  local platform="${goos}/${goarch}"
  local name="$(platform_name "$goos" "$goarch")"
  local platform_dir="${BUILD_DIR}/${goos}-${goarch}"

  local exe_suffix=""
  if [[ "$goos" == "windows" ]]; then
    exe_suffix=".exe"
  fi

  mkdir -p "${platform_dir}"

  local build_err=0
  local commit_sha build_date
  commit_sha="$(git -C "${ROOT}" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
  build_date="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

  echo "[building] ${platform} ..." >&2

  local build_log
  build_log=$(cd "${ROOT}" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
        -ldflags "-s -w -X github.com/FREEZONEX/Tier0-cli/internal/version.BuildVersion=${VERSION} -X github.com/FREEZONEX/Tier0-cli/internal/version.BuildCommit=${commit_sha} -X github.com/FREEZONEX/Tier0-cli/internal/version.BuildDate=${build_date}" \
        -o "${platform_dir}/tier0${exe_suffix}" . 2>&1) || build_err=1

  if [[ $build_err -ne 0 ]]; then
    echo "[FAILED]   ${platform}${build_log:+: ${build_log}}" >&2
    echo "FAILED:${platform}"
    rm -rf "${platform_dir}"
    return 1
  fi

  echo "[OK]       ${platform} (${name})" >&2
  echo "OK:${platform}:${name}"
}
export -f build_one platform_name
export ROOT BUILD_DIR VERSION SKILL_SRC

# Parallel build.
BUILD_OK=0
BUILD_FAIL=0
FAILED_PLATFORMS=()

results=$(printf '%s\n' "${PLATFORMS[@]}" | xargs -P"${PARALLEL}" -I{} bash -c 'IFS=: read -r goos goarch <<< "$1"; build_one "$goos" "$goarch"' _ {})

while IFS= read -r line; do
  if [[ "$line" == OK:* ]]; then
    platform="${line#OK:}"
    platform="${platform%%:*}"
    name="${line##*:}"
    goos="${platform%%/*}"
    goarch="${platform#*/}"

    pkg_name="tier0-cli-${VERSION}-${name}"
    platform_dir="${BUILD_DIR}/${goos}-${goarch}"
    if [[ "$goos" == "windows" ]]; then
      (cd "${BUILD_DIR}" && zip -rq "${RELEASE_DIR}/${pkg_name}.zip" "${goos}-${goarch}")
    else
      (cd "${BUILD_DIR}" && tar -czf "${RELEASE_DIR}/${pkg_name}.tar.gz" "${goos}-${goarch}")
    fi
    echo "[build] ${platform} (${name}) ... OK"
    BUILD_OK=$((BUILD_OK + 1))
  elif [[ "$line" == FAILED:* ]]; then
    platform="${line#FAILED:}"
    echo "[build] ${platform} ... FAILED"
    BUILD_FAIL=$((BUILD_FAIL + 1))
    FAILED_PLATFORMS+=("$platform")
  fi
done <<< "$results"

echo ""
echo "========================================"
echo "  Build results"
echo "========================================"
echo "  Success: ${BUILD_OK}"
echo "  Failed: ${BUILD_FAIL}"
if [[ ${#FAILED_PLATFORMS[@]} -gt 0 ]]; then
  echo "  Failed platforms:"
  for fp in "${FAILED_PLATFORMS[@]}"; do
    echo "    - ${fp}"
  done
fi
echo ""

# Generate checksums.
echo "[checksum] Generating SHA256 checksums..."
cd "${RELEASE_DIR}"
sha256sum * > "sha256sums.txt"
cd - >/dev/null

echo ""
echo "Release packages generated: ${RELEASE_DIR}"
ls -lh "${RELEASE_DIR}"
echo ""

# ========================================
# GitHub Release
# ========================================
release_github() {
  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "[github] GITHUB_TOKEN is not set; skipping GitHub Release"
    echo "         To publish, set the environment variable and rerun:"
    echo "         export GITHUB_TOKEN=ghp_xxxxxxxx"
    return 1
  fi

  local repo="FREEZONEX/Tier0-cli"
  echo "[github] Creating Release: ${VERSION} ..."

  # Build JSON payload with Node.js to guarantee valid JSON encoding
  # (avoids bash quoting / \n expansion / backtick issues on all platforms)
  local payload_file
  payload_file=$(mktemp)
  (node -e "
    const body = [
      '## Tier0 CLI ${VERSION}',
      '',
      'See [CHANGELOG](https://github.com/FREEZONEX/Tier0-cli/blob/main/CHANGELOG.md) for full release notes.',
      '',
      '**Install**',
      '',
      '\`\`\`bash',
      'npx @tier0/cli@latest install',
      '\`\`\`',
      '',
      '**Upgrade**',
      '',
      '\`\`\`bash',
      'tier0 upgrade',
      '\`\`\`'
    ].join('\n');
    process.stdout.write(JSON.stringify({
      tag_name: '${VERSION}',
      name: 'tier0-cli ${VERSION}',
      body: body
    }));
  " > "$payload_file") || {
    echo "[github] Failed to generate JSON payload (is node unavailable?)"
    rm -f "$payload_file"
    return 1
  }

  # Preflight: check api.github.com connectivity with an 8-second timeout.
  local ping_code
  ping_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 8 \
    -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/rate_limit" 2>/dev/null || echo "000")
  if [[ "$ping_code" == "000" ]]; then
    echo "[github] Failed to create Release (HTTP 000)"
    echo "[github] Common causes:"
    echo "         000 - cannot connect to api.github.com (network, firewall, or proxy issue)"
    echo "               If using a proxy, set: export https_proxy=http://host:port"
    rm -f "$payload_file"
    return 1
  fi

  local release_resp http_code
  release_resp=$(curl -s -w "\n__HTTP_CODE__:%{http_code}" -X POST \
    -H "Authorization: token ${GITHUB_TOKEN}" \
    -H "Accept: application/vnd.github.v3+json" \
    -H "Content-Type: application/json" \
    "https://api.github.com/repos/${repo}/releases" \
    --data-binary "@${payload_file}")
  rm -f "$payload_file"

  http_code=$(echo "$release_resp" | grep '__HTTP_CODE__:' | cut -d: -f2)
  release_resp=$(echo "$release_resp" | grep -v '__HTTP_CODE__:')

  local upload_url
  upload_url=$(echo "$release_resp" | grep -o '"upload_url":"[^"]*' | cut -d'"' -f4 | sed 's/{?name,label}//')
  # Compatible with space-separated format.
  if [[ -z "$upload_url" ]]; then
    upload_url=$(echo "$release_resp" | grep -o '"upload_url": "[^"]*' | cut -d'"' -f4 | sed 's/{?name,label}//')
  fi

  # Release already exists (422); fetch the existing upload_url.
  if [[ -z "$upload_url" && "$http_code" == "422" ]]; then
    echo "[github] Release ${VERSION} already exists (HTTP 422); fetching existing Release..."
    local existing_resp
    existing_resp=$(curl -s \
      -H "Authorization: token ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github.v3+json" \
      "https://api.github.com/repos/${repo}/releases/tags/${VERSION}")
    upload_url=$(echo "$existing_resp" | grep -o '"upload_url":"[^"]*' | cut -d'"' -f4 | sed 's/{?name,label}//')
    if [[ -z "$upload_url" ]]; then
      upload_url=$(echo "$existing_resp" | grep -o '"upload_url": "[^"]*' | cut -d'"' -f4 | sed 's/{?name,label}//')
    fi
    if [[ -n "$upload_url" ]]; then
      echo "[github] Found existing Release; uploading additional assets..."
    fi
  fi

  if [[ -z "$upload_url" ]]; then
    echo "[github] Failed to create Release (HTTP ${http_code})"
    # Extract and show the API error message.
    local api_msg
    api_msg=$(echo "$release_resp" | grep -o '"message":"[^"]*' | cut -d'"' -f4)
    [[ -z "$api_msg" ]] && api_msg=$(echo "$release_resp" | grep -o '"message": "[^"]*' | cut -d'"' -f4)
    [[ -n "$api_msg" ]] && echo "[github] Error message: ${api_msg}"
    echo "[github] Common causes:"
    echo "         401 - GITHUB_TOKEN is invalid or expired"
    echo "         403 - token lacks repo/workflow permissions"
    echo "         404 - repository does not exist or token has no access"
    echo "         422 - tag already has a Release and fetching it failed; check manually"
    return 1
  fi

  echo "[github] Release ready; uploading assets..."

  for asset in "${RELEASE_DIR}"/*; do
    local fname
    fname=$(basename "$asset")
    echo -n "[github] Uploading ${fname} ... "
    local upload_code
    if ! upload_code=$(curl -sS -o /dev/null -w "%{http_code}" -X POST \
      -H "Authorization: token ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github.v3+json" \
      -H "Content-Type: application/octet-stream" \
      "${upload_url}?name=${fname}" \
      --data-binary "@$asset" 2>/dev/null); then
      upload_code="000"
    fi
    if [[ "${upload_code}" =~ ^2[0-9][0-9]$ ]]; then
      echo "OK"
    else
      echo "FAILED (HTTP ${upload_code})"
      return 1
    fi
  done

  echo "[github] Published: https://github.com/${repo}/releases/tag/${VERSION}"
}

# ========================================
# npm publish
# ========================================
npm_with_auth() {
  local npm_dir="$1"
  shift

  if [[ -z "${NPM_TOKEN:-}" ]]; then
    (cd "${npm_dir}" && npm "$@")
    return
  fi

  local userconfig
  userconfig=$(mktemp)
  printf '%s\n' "//registry.npmjs.org/:_authToken=${NPM_TOKEN}" > "${userconfig}"
  (
    cd "${npm_dir}"
    NPM_CONFIG_USERCONFIG="${userconfig}" npm "$@"
  )
  local status=$?
  rm -f "${userconfig}"
  return "${status}"
}

prepare_npm() {
  local npm_dir="${ROOT}/npm-wrapper"

  if [[ ! -f "${npm_dir}/package.json" ]]; then
    echo "[npm] npm-wrapper/package.json does not exist"
    return 1
  fi

  if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    echo "[npm] node and npm are required for release preflight"
    return 1
  fi

  # Sync package.json version without the v prefix.
  # Use cd plus relative paths to avoid Git Bash Unix path parsing issues on Windows Node.js.
  local semver="${VERSION#v}"
  (cd "${npm_dir}" && node -e "
    const fs = require('fs');
    const pkg = JSON.parse(fs.readFileSync('./package.json', 'utf8'));
    pkg.version = '${semver}';
    fs.writeFileSync('./package.json', JSON.stringify(pkg, null, 2) + '\n');
    console.log('[npm] package.json version -> ' + pkg.version);
  ")

  echo "[npm] Running tests..."
  (cd "${npm_dir}" && npm test)

  echo "[npm] Validating package contents..."
  (cd "${npm_dir}" && npm pack --dry-run --json >/dev/null)

  echo "[npm] Verifying publish authentication..."
  if ! npm_with_auth "${npm_dir}" whoami >/dev/null; then
    echo "[npm] Authentication check failed. Run npm login or set NPM_TOKEN."
    return 1
  fi
  echo "[npm] Preflight passed for @tier0/cli@${semver}"
}

release_npm() {
  local npm_dir="${ROOT}/npm-wrapper"
  local semver="${VERSION#v}"

  echo "[npm] Publishing @tier0/cli@${semver} ..."
  if npm_with_auth "${npm_dir}" publish --access public; then
    echo "[npm] Published: https://www.npmjs.com/package/@tier0/cli"
  else
    echo "[npm] Publish failed; check:"
    echo "      1. npm is logged in (npm whoami) or NPM_TOKEN is set"
    echo "      2. publish permission for @tier0/cli; @tier0 org membership is required"
    return 1
  fi
}

# Build-only mode supports local packaging and CI artifact verification without
# requiring GitHub or npm credentials.
if [[ "${BUILD_ONLY}" == "1" ]]; then
  echo "[release] BUILD_ONLY=1; packages verified, publish skipped"
  exit 0
fi

# Try publishing.
echo "========================================"
echo "  Publish stage"
echo "========================================"
echo ""

if ! prepare_npm; then
  echo ""
  echo "[release] npm preflight failed; nothing was published"
  exit 1
fi

GITHUB_OK=0
if release_github; then
  GITHUB_OK=1
else
  echo ""
  echo "[release] GitHub Release failed; skipping npm publish because the version is not ready"
  exit 1
fi

echo ""

# Run npm publish only after GitHub Release succeeds.
if [[ "${GITHUB_OK}" == "1" ]]; then
  if ! release_npm; then
    echo "[release] npm publish failed; GitHub assets exist but the release is incomplete"
    exit 1
  fi
fi

echo ""
echo "========================================"
echo "  Done"
echo "========================================"
echo "Build artifacts: ${RELEASE_DIR}"
