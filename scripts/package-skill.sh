#!/usr/bin/env bash
set -euo pipefail

# Tier0 Skills packaging script
# Usage: bash scripts/package-skill.sh [OUT_DIR] [--version VERSION]
# Example: bash scripts/package-skill.sh ./dist/skills --version v0.2.0

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_SRC="${SKILL_SRC:-${ROOT}/../Tier0-skill}"
if [[ ! -f "${SKILL_SRC}/SKILL.md" && -f "${ROOT}/../skill/SKILL.md" ]]; then
  SKILL_SRC="${ROOT}/../skill"
fi
OUT_DIR="${1:-${ROOT}/dist/skills}"
SKILL_VERSION="${SKILL_VERSION:-0.0.0-dev}"

# Parse arguments.
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      if [[ -z "${2:-}" ]]; then
        echo "[package-skill] error: --version requires a version" >&2
        exit 1
      fi
      SKILL_VERSION="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $(basename "$0") [OUT_DIR] [--version VERSION]"
      echo ""
      echo "Arguments:"
      echo "  OUT_DIR            Output directory (default: ./dist/skills)"
      echo ""
      echo "Options:"
      echo "  --version VERSION  Skills version (default: ${SKILL_VERSION})"
      echo "  -h, --help         Show this help"
      exit 0
      ;;
    -*)
      echo "[package-skill] error: unknown option: $1" >&2
      exit 1
      ;;
    *)
      OUT_DIR="$1"
      shift
      ;;
  esac
done

if [[ ! -f "${SKILL_SRC}/SKILL.md" ]]; then
  echo "[package-skill] error: skill source not found at ${SKILL_SRC}" >&2
  echo "[package-skill] check out FREEZONEX/Tier0-skill beside Tier0-cli or set SKILL_SRC" >&2
  exit 1
fi

echo "[package-skill] packaging skills from ${SKILL_SRC}"
echo "[package-skill] output: ${OUT_DIR}"
echo "[package-skill] version: ${SKILL_VERSION}"

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

# Copy all skill files, excluding .git and installer scripts.
for item in "${SKILL_SRC}"/*; do
  name=$(basename "$item")
  if [[ "$name" == ".git" || "$name" == "install-openclaw.sh" ||
        "$name" == "README.md" || "$name" == "CHANGELOG.md" ||
        "$name" == "_commit_msg.txt" ]]; then
    continue
  fi
  if [[ -d "$item" ]]; then
    cp -R "$item" "${OUT_DIR}/"
  else
    cp "$item" "${OUT_DIR}/"
  fi
done

# The Skill source checkout also contains complete protocol implementation
# snapshots for maintainer reference. They are not referenced by Skill docs and
# must not be shipped to users.
rm -rf -- "${OUT_DIR}/flow/references/protocal"

# Never ship nested Git repositories from reference projects.
find "${OUT_DIR}" -type d -name .git -prune -exec rm -rf {} +

# Write metadata.
cat > "${OUT_DIR}/_meta.json" << EOF
{
  "version": "${SKILL_VERSION}",
  "updatedAt": "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
}
EOF

echo "[package-skill] done: ${OUT_DIR}"
echo "[package-skill] contents:"
find "${OUT_DIR}" -type f | sort | sed 's/^/  /'
