#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

MODE="${RUN_MODE:-docker}"
PROFILE="${RUN_PROFILE:-dev}"
BUILD_IMAGES=0
START_PUPPETEER=1
FORCE_STRICT=0
CONTENT_NUM=""
TASK_BIN=""

log() {
  printf '[run.sh] %s\n' "$*"
}

fail() {
  printf '[run.sh] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  ./scripts/run.sh [CONTENT_NUM] [--docker|--local] [--prod|--dev] [--strict] [--build] [--skip-puppeteer]

Examples:
  ./scripts/run.sh
  ./scripts/run.sh 43
  ./scripts/run.sh 43 --build
  ./scripts/run.sh 43 --prod
  ./scripts/run.sh --local

Notes:
  - CONTENT_NUM must be digits only.
  - Default mode is docker (override with RUN_MODE=local or --local).
  - Default profile is dev (override with RUN_PROFILE=prod or --prod).
  - --strict forces STRICT_ENV=true for the delegated task run.
  - --build rebuilds Docker images before docker runs.
EOF
}

require_task() {
  if command -v task >/dev/null 2>&1; then
    TASK_BIN="task"
    return
  fi
  fail "task is not installed or not in PATH"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --docker)
      MODE="docker"
      ;;
    --local)
      MODE="local"
      ;;
    --prod)
      PROFILE="prod"
      ;;
    --dev)
      PROFILE="dev"
      ;;
    --strict)
      FORCE_STRICT=1
      ;;
    --build)
      BUILD_IMAGES=1
      ;;
    --skip-puppeteer)
      START_PUPPETEER=0
      ;;
    --*)
      fail "unknown flag: $1"
      ;;
    *)
      if [ -n "${CONTENT_NUM}" ]; then
        fail "only one CONTENT_NUM argument is allowed"
      fi
      CONTENT_NUM="$1"
      ;;
  esac
  shift
done

if [ -n "${CONTENT_NUM}" ] && [[ ! "${CONTENT_NUM}" =~ ^[0-9]+$ ]]; then
  fail "CONTENT_NUM must contain digits only"
fi

require_task

if [ "${BUILD_IMAGES}" -eq 1 ]; then
  if [ "${MODE}" != "docker" ]; then
    log "Ignoring --build for local mode"
  else
    "${TASK_BIN}" docker:build
  fi
fi

TASK_NAME=""
if [ "${PROFILE}" = "prod" ]; then
  if [ "${START_PUPPETEER}" -eq 0 ]; then
    fail "--skip-puppeteer is not allowed in prod profile (strict mode requires renderer)"
  fi
  if [ "${MODE}" = "docker" ]; then
    TASK_NAME="run:prod:docker"
  else
    TASK_NAME="run:prod:local"
  fi
else
  if [ "${MODE}" = "docker" ]; then
    TASK_NAME="run:docker"
    if [ "${START_PUPPETEER}" -eq 0 ]; then
      TASK_NAME="run:docker:no-render"
    fi
  else
    TASK_NAME="run:local"
    if [ "${START_PUPPETEER}" -eq 0 ]; then
      TASK_NAME="run:local:no-render"
    fi
  fi
fi

TASK_ARGS=("${TASK_NAME}")
if [ -n "${CONTENT_NUM}" ]; then
  TASK_ARGS+=("NUM=${CONTENT_NUM}")
fi

if [ "${PROFILE}" = "prod" ] || [ "${FORCE_STRICT}" -eq 1 ]; then
  STRICT_ENV=true "${TASK_BIN}" "${TASK_ARGS[@]}"
else
  "${TASK_BIN}" "${TASK_ARGS[@]}"
fi
