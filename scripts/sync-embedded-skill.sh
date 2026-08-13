#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_SRC="${SKILL_SRC:-${ROOT}/../Tier0-skill}"
TARGET="${ROOT}/internal/embeddedskill/content"
MODE="${1:-sync}"

if [[ ! -f "${SKILL_SRC}/SKILL.md" ]]; then
  echo "[embedded-skill] source not found: ${SKILL_SRC}" >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf -- "${tmp}"' EXIT
staged="${tmp}/content"
mkdir -p "${staged}"

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

if [[ "${MODE}" == "--check" ]]; then
  if [[ ! -d "${TARGET}" ]] || ! diff -qr "${staged}" "${TARGET}" >/dev/null; then
    echo "[embedded-skill] embedded content is out of sync" >&2
    echo "[embedded-skill] run: bash scripts/sync-embedded-skill.sh" >&2
    exit 1
  fi
  echo "[embedded-skill] content is in sync"
  exit 0
fi

if [[ "${MODE}" != "sync" ]]; then
  echo "usage: bash scripts/sync-embedded-skill.sh [--check]" >&2
  exit 2
fi

rm -rf -- "${TARGET}"
mkdir -p "$(dirname "${TARGET}")"
mv "${staged}" "${TARGET}"
echo "[embedded-skill] synced runtime Skill content into ${TARGET}"
