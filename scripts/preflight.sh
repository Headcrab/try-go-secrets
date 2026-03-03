#!/usr/bin/env bash
set -euo pipefail

strict_raw="${1:-${STRICT_MODE:-false}}"
require_render_raw="${2:-${REQUIRE_RENDER_MODE:-false}}"

load_dotenv() {
  local file="${1:-.env}"
  [ -f "${file}" ] || return 0

  while IFS= read -r line || [ -n "${line}" ]; do
    line="${line%$'\r'}"
    case "${line}" in
      ""|\#*) continue ;;
    esac

    local key="${line%%=*}"
    local value="${line#*=}"
    key="$(printf '%s' "${key}" | tr -d '[:space:]')"

    case "${key}" in
      ''|*[!A-Za-z0-9_]*)
        continue
        ;;
    esac

    if [ "${value#\"}" != "${value}" ] && [ "${value%\"}" != "${value}" ]; then
      value="${value#\"}"
      value="${value%\"}"
    elif [ "${value#\'}" != "${value}" ] && [ "${value%\'}" != "${value}" ]; then
      value="${value#\'}"
      value="${value%\'}"
    fi

    export "${key}=${value}"
  done < "${file}"
}

load_dotenv .env

to_bool() {
  case "$(printf '%s' "${1:-}" | tr -d '\r' | tr '[:upper:]' '[:lower:]' | xargs)" in
    1|true|yes|on) printf 'true' ;;
    *) printf 'false' ;;
  esac
}

is_empty() {
  [ -z "${1:-}" ]
}

is_placeholder() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    replace_me|changeme|your_api_key|your_folder_id) return 0 ;;
    *) return 1 ;;
  esac
}

strict_mode="$(to_bool "${strict_raw}")"
require_render="$(to_bool "${require_render_raw}")"

recommended=(
  APP_ENV
  RAW_DIR
  OUTPUT_DIR
  STATE_DIR
  MAX_VIDEO_DURATION_SEC
  TTS_DAILY_LIMIT
)

for name in "${recommended[@]}"; do
  value="${!name-}"
  if is_empty "${value}"; then
    echo "WARN: missing optional env var in dev mode: ${name}"
  fi
done

missing=()

append_missing() {
  local item="$1"
  for existing in "${missing[@]:-}"; do
    if [ "${existing}" = "${item}" ]; then
      return 0
    fi
  done
  missing+=("${item}")
}

if [ "${strict_mode}" = "true" ]; then
  required=(
    APP_ENV
    RAW_DIR
    OUTPUT_DIR
    STATE_DIR
    MAX_VIDEO_DURATION_SEC
    TTS_DAILY_LIMIT
    ZAI_API_KEY
    YANDEX_API_KEY
    YANDEX_FOLDER_ID
    PUPPETEER_SERVICE_URL
  )
  for name in "${required[@]}"; do
    value="${!name-}"
    if is_empty "${value}"; then
      missing+=("${name}")
    fi
  done

  for name in ZAI_API_KEY YANDEX_API_KEY YANDEX_FOLDER_ID; do
    value="${!name-}"
    if [ -n "${value}" ] && is_placeholder "${value}"; then
      append_missing "${name} (placeholder value)"
    fi
  done
fi

if [ "${require_render}" = "true" ] && is_empty "${PUPPETEER_SERVICE_URL-}"; then
  append_missing "PUPPETEER_SERVICE_URL"
fi

if [ "${#missing[@]}" -gt 0 ]; then
  echo "Preflight failed: $(IFS=', '; printf '%s' "${missing[*]}")" >&2
  exit 1
fi

echo "Preflight passed (strict=${strict_mode}, require_render=${require_render})"
