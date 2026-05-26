#!/usr/bin/env bash
set -euo pipefail

# Tier0 CLI 跨平台 Release 构建脚本
# 用法: bash scripts/release.sh [VERSION]
# 示例: bash scripts/release.sh v0.2.0
#
# 环境变量（推荐写入 .env 文件，脚本会自动加载）：
#   GITHUB_TOKEN  - GitHub Personal Access Token
#   PARALLEL      - 并发构建数（默认 8）

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 自动加载 .env 文件（如果存在）
if [[ -f "${ROOT}/.env" ]]; then
  # shellcheck source=/dev/null
  source "${ROOT}/.env"
fi

VERSION="${1:-v0.1.0}"
BUILD_DIR="${ROOT}/dist/release-${VERSION}"
RELEASE_DIR="${BUILD_DIR}/packages"
PARALLEL="${PARALLEL:-8}"

# Skill 源码目录（与 cli 同级的 skill 子模块）
SKILL_SRC="${ROOT}/../skill"

# 排除的平台（非原生或不需要）
EXCLUDE_PLATFORMS="js/wasm wasip1/wasm android/386 android/amd64 android/arm android/arm64 ios/amd64 ios/arm64"

# 平台 → 友好名称映射
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

# 检查是否需要排除
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

# 确保版本号以 v 开头
if [[ ! "$VERSION" =~ ^v ]]; then
  VERSION="v${VERSION}"
fi

# 创建目录
rm -rf "${BUILD_DIR}"
mkdir -p "${RELEASE_DIR}"

# 获取所有原生平台（排除 wasm/android/ios/js）
PLATFORMS=()
while IFS='/' read -r goos goarch; do
  platform="${goos}/${goarch}"
  if is_excluded "$platform"; then
    echo "[skip] ${platform} (excluded)"
    continue
  fi
  PLATFORMS+=("${goos}:${goarch}")
done < <(go tool dist list | sort)

echo ""
echo "准备构建 ${#PLATFORMS[@]} 个平台，并发数: ${PARALLEL}"
echo ""

# 构建单个平台的函数
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

  if ! (cd "${ROOT}" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
        -ldflags "-s -w -X github.com/FREEZONEX/Tier0-cli/internal/version.BuildVersion=${VERSION} -X github.com/FREEZONEX/Tier0-cli/internal/version.BuildCommit=${commit_sha} -X github.com/FREEZONEX/Tier0-cli/internal/version.BuildDate=${build_date}" \
        -o "${platform_dir}/tier0${exe_suffix}" . 2>/dev/null); then
    build_err=1
  fi

  if [[ $build_err -ne 0 ]]; then
    echo "FAILED:${platform}"
    rm -rf "${platform_dir}"
    return 1
  fi

  # 复制 skill 资源到发布包
  if [[ -d "${SKILL_SRC}" ]]; then
    mkdir -p "${platform_dir}/skill"
    # 复制 SKILL.md、uns/ 目录、LICENSE 等
    for item in "${SKILL_SRC}"/*; do
      local skill_name
      skill_name=$(basename "$item")
      # 跳过 .git 和脚本
      if [[ "$skill_name" == ".git" || "$skill_name" == "install-openclaw.sh" ]]; then
        continue
      fi
      if [[ -d "$item" ]]; then
        cp -R "$item" "${platform_dir}/skill/"
      else
        cp "$item" "${platform_dir}/skill/"
      fi
    done

    # 写入 skills 版本元数据
    cat > "${platform_dir}/skill/_meta.json" << EOF
{
  "version": "${VERSION}",
  "updatedAt": "${build_date}"
}
EOF
  fi

  echo "OK:${platform}:${name}"
}
export -f build_one platform_name
export ROOT BUILD_DIR VERSION SKILL_SRC

# 并行构建
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
echo "  构建结果"
echo "========================================"
echo "  成功: ${BUILD_OK}"
echo "  失败: ${BUILD_FAIL}"
if [[ ${#FAILED_PLATFORMS[@]} -gt 0 ]]; then
  echo "  失败平台:"
  for fp in "${FAILED_PLATFORMS[@]}"; do
    echo "    - ${fp}"
  done
fi
echo ""

# 生成 checksums
echo "[checksum] 生成 SHA256 校验和..."
cd "${RELEASE_DIR}"
sha256sum * > "sha256sums.txt"
cd - >/dev/null

echo ""
echo "发布包已生成: ${RELEASE_DIR}"
ls -lh "${RELEASE_DIR}"
echo ""

# ========================================
# GitHub Release
# ========================================
release_github() {
  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "[github] 未设置 GITHUB_TOKEN，跳过 GitHub Release"
    echo "         如需发布，请设置环境变量后重新运行:"
    echo "         export GITHUB_TOKEN=ghp_xxxxxxxx"
    return 1
  fi

  local repo="FREEZONEX/Tier0-cli"
  echo "[github] 创建 Release: ${VERSION} ..."

  # Build JSON payload via a temp file to avoid shell quoting / backtick issues
  local payload_file
  payload_file=$(mktemp)
  # Use printf so no shell substitution occurs inside the body string
  printf '{"tag_name":"%s","name":"tier0-cli %s","body":"## Tier0 CLI %s\n\nSee [CHANGELOG](https://github.com/FREEZONEX/Tier0-cli/blob/main/CHANGELOG.md) for full release notes.\n\n**Install**\n\nnpx @tier0/cli@latest install\n\n**Upgrade**\n\ntier0 upgrade"}' \
    "${VERSION}" "${VERSION}" "${VERSION}" > "$payload_file"

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
  # 兼容带空格格式
  if [[ -z "$upload_url" ]]; then
    upload_url=$(echo "$release_resp" | grep -o '"upload_url": "[^"]*' | cut -d'"' -f4 | sed 's/{?name,label}//')
  fi

  # Release 已存在（422）→ 获取已有 Release 的 upload_url
  if [[ -z "$upload_url" && "$http_code" == "422" ]]; then
    echo "[github] Release ${VERSION} 已存在（HTTP 422），获取已有 Release..."
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
      echo "[github] 已找到现有 Release，追加上传资产..."
    fi
  fi

  if [[ -z "$upload_url" ]]; then
    echo "[github] 创建 Release 失败（HTTP ${http_code}）"
    # 提取并显示 API 错误信息
    local api_msg
    api_msg=$(echo "$release_resp" | grep -o '"message":"[^"]*' | cut -d'"' -f4)
    [[ -z "$api_msg" ]] && api_msg=$(echo "$release_resp" | grep -o '"message": "[^"]*' | cut -d'"' -f4)
    [[ -n "$api_msg" ]] && echo "[github] 错误信息: ${api_msg}"
    echo "[github] 常见原因："
    echo "         401 — GITHUB_TOKEN 无效或过期"
    echo "         403 — Token 缺少 repo/workflow 权限"
    echo "         404 — 仓库不存在或 Token 无访问权限"
    echo "         422 — 标签已存在 Release 且获取失败（请手动检查）"
    return 1
  fi

  echo "[github] Release 就绪，开始上传资产..."

  for asset in "${RELEASE_DIR}"/*; do
    local fname
    fname=$(basename "$asset")
    echo -n "[github] 上传 ${fname} ... "
    if curl -s -X POST \
      -H "Authorization: token ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github.v3+json" \
      -H "Content-Type: application/octet-stream" \
      "${upload_url}?name=${fname}" \
      --data-binary "@$asset" >/dev/null 2>&1; then
      echo "OK"
    else
      echo "FAILED"
    fi
  done

  echo "[github] 发布完成: https://github.com/${repo}/releases/tag/${VERSION}"
}

# ========================================
# npm publish
# ========================================
release_npm() {
  local npm_dir="${ROOT}/npm-wrapper"

  if [[ ! -f "${npm_dir}/package.json" ]]; then
    echo "[npm] npm-wrapper/package.json 不存在，跳过 npm 发布"
    return 1
  fi

  # 同步 package.json 版本号（去掉 v 前缀）
  # 注意：用 cd + 相对路径，避免 Git Bash 的 Unix 路径在 Windows Node.js 上解析错误
  local semver="${VERSION#v}"
  if command -v node >/dev/null 2>&1; then
    (cd "${npm_dir}" && node -e "
      const fs = require('fs');
      const pkg = JSON.parse(fs.readFileSync('./package.json', 'utf8'));
      pkg.version = '${semver}';
      fs.writeFileSync('./package.json', JSON.stringify(pkg, null, 2) + '\n');
      console.log('[npm] package.json version → ' + pkg.version);
    ")
  else
    echo "[npm] 未找到 node，跳过版本号同步"
  fi

  # 检查 npm 登录态（需提前 npm login 或设置 NPM_TOKEN）
  # 全部操作在 cd 后的子 shell 里执行，避免路径转换问题
  echo "[npm] 发布 @tier0/cli@${semver} ..."
  if (
    cd "${npm_dir}"
    if [[ -n "${NPM_TOKEN:-}" ]]; then
      echo "[npm] 使用 NPM_TOKEN 认证..."
      echo "//registry.npmjs.org/:_authToken=${NPM_TOKEN}" > .npmrc
    fi
    trap 'rm -f .npmrc' EXIT
    npm publish --access public
  ); then
    echo "[npm] 发布成功: https://www.npmjs.com/package/@tier0/cli"
  else
    echo "[npm] 发布失败，请检查："
    echo "      1. npm 已登录（npm whoami）或已设置 NPM_TOKEN"
    echo "      2. 包名 @tier0/cli 的发布权限（需 @tier0 org 成员）"
    return 1
  fi
}

# 尝试发布
echo "========================================"
echo "  发布阶段"
echo "========================================"
echo ""

release_github || true

echo ""

# npm publish 与 GitHub Release 强制同步，每次发版都执行
# 需要提前 npm login 或设置 NPM_TOKEN
release_npm || true

echo ""
echo "========================================"
echo "  Done"
echo "========================================"
echo "构建产物: ${RELEASE_DIR}"
