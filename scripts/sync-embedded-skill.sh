#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${ROOT}/internal/embeddedskill/content"
REPO="FREEZONEX/Tier0-skill"
MODE="sync"
SOURCE_MODE="github"
COMMIT=""
LOCAL_SOURCE=""

usage() {
  cat <<'EOF'
Usage: bash scripts/sync-embedded-skill.sh [options]

By default, downloads the latest Tier0-skill main commit from GitHub.

Options:
  --check           Compare the embedded snapshot without changing it
  --commit <sha>    Download a specific GitHub commit instead of latest main
  --local [path]    Use a local Tier0-skill checkout (default: ../Tier0-skill)
  -h, --help        Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      MODE="check"
      shift
      ;;
    --commit)
      [[ $# -ge 2 ]] || { echo "[embedded-skill] --commit requires a SHA" >&2; exit 2; }
      COMMIT="$2"
      shift 2
      ;;
    --local)
      SOURCE_MODE="local"
      if [[ $# -ge 2 && "$2" != --* ]]; then
        LOCAL_SOURCE="$2"
        shift 2
      else
        shift
      fi
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[embedded-skill] unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

tmp="$(mktemp -d)"
trap 'rm -rf -- "${tmp}"' EXIT
staged="${tmp}/content"
mkdir -p "${staged}"

resolve_latest_commit() {
  git ls-remote "https://github.com/${REPO}.git" refs/heads/main 2>/dev/null | awk 'NR == 1 { print $1 }'
}

if [[ "${SOURCE_MODE}" == "github" ]]; then
  if [[ -z "${COMMIT}" ]]; then
    COMMIT="$(resolve_latest_commit)"
  fi
  if [[ ! "${COMMIT}" =~ ^[0-9a-fA-F]{40}$ ]]; then
    echo "[embedded-skill] invalid Git commit SHA: ${COMMIT:-<empty>}" >&2
    exit 1
  fi

  archive="${tmp}/tier0-skill.tar.gz"
  echo "[embedded-skill] downloading ${REPO}@${COMMIT}"
  curl -fsSL \
    -H "User-Agent: tier0-cli-release" \
    "https://codeload.github.com/${REPO}/tar.gz/${COMMIT}" \
    -o "${archive}"
  mkdir -p "${tmp}/source"
  tar -xzf "${archive}" -C "${tmp}/source"
  SKILL_SRC="$(find "${tmp}/source" -mindepth 1 -maxdepth 1 -type d -print -quit)"
  [[ -n "${SKILL_SRC}" ]] || { echo "[embedded-skill] downloaded archive is empty" >&2; exit 1; }
  SOURCE_LABEL="https://github.com/${REPO}@${COMMIT}"
else
  SKILL_SRC="${LOCAL_SOURCE:-${SKILL_SRC:-${ROOT}/../Tier0-skill}}"
	if [[ -e "${SKILL_SRC}/.git" ]]; then
		resolved_source="$(cd "${SKILL_SRC}" && pwd)"
		COMMIT="$(git -c safe.directory="${resolved_source}" -C "${SKILL_SRC}" rev-parse HEAD 2>/dev/null || true)"
  fi
  COMMIT="${COMMIT:-local}"
  SOURCE_LABEL="local:${SKILL_SRC}@${COMMIT}"
fi

if [[ ! -f "${SKILL_SRC}/SKILL.md" ]]; then
  echo "[embedded-skill] source not found: ${SKILL_SRC}" >&2
  exit 1
fi

for item in "${SKILL_SRC}"/*; do
  name="$(basename "${item}")"
  case "${name}" in
    .git|README.md|CHANGELOG.md|_commit_msg.txt|install-openclaw.sh)
      continue
      ;;
  esac
  cp -R "${item}" "${staged}/"
done

rm -rf -- "${staged}/flow/references/protocal"
find "${staged}" -type d -name .git -prune -exec rm -rf {} +

printf '{\n  "repository": "https://github.com/%s",\n  "commit": "%s"\n}\n' \
  "${REPO}" "${COMMIT}" > "${staged}/_source.json"

if [[ "${MODE}" == "check" ]]; then
  if [[ ! -d "${TARGET}" ]] || ! diff -qr "${staged}" "${TARGET}" >/dev/null; then
    echo "[embedded-skill] embedded content is out of sync with ${SOURCE_LABEL}" >&2
    echo "[embedded-skill] run: bash scripts/sync-embedded-skill.sh" >&2
    exit 1
  fi
  echo "[embedded-skill] content is in sync with ${SOURCE_LABEL}"
  exit 0
fi

rm -rf -- "${TARGET}"
mkdir -p "$(dirname "${TARGET}")"
mv "${staged}" "${TARGET}"
echo "[embedded-skill] synced runtime Skill from ${SOURCE_LABEL}"
