#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK_PATH="${SCRIPT_DIR}/hooks/devlog.zsh"

if [[ ! -f "${HOOK_PATH}" ]]; then
  echo "Hook not found: ${HOOK_PATH}" >&2
  exit 1
fi

ZSHRC="${HOME}/.zshrc"
LINE="source \"${HOOK_PATH}\""

if [[ -f "${ZSHRC}" ]] && grep -Fq "${LINE}" "${ZSHRC}"; then
  echo "Already installed in ${ZSHRC}"
  exit 0
fi

{
  echo ""
  echo "# DevLog Report hook"
  echo "${LINE}"
} >> "${ZSHRC}"

echo "Installed to ${ZSHRC}"
