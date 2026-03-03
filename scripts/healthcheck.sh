#!/usr/bin/env bash
set -euo pipefail

url="${1:?health url is required}"
timeout="${2:-20}"

if command -v curl >/dev/null 2>&1; then
  curl -fsS --max-time "${timeout}" "${url}" >/dev/null
  echo "Healthy: ${url}"
  exit 0
fi

if command -v wget >/dev/null 2>&1; then
  wget -q --timeout="${timeout}" -O /dev/null "${url}"
  echo "Healthy: ${url}"
  exit 0
fi

echo "Neither curl nor wget is installed for health check." >&2
exit 1
