#!/usr/bin/env bash
set -euo pipefail

mkdir -p state
printf '{"by_path":{}}' > state/processed.json
printf '{"date":"","characters":0}' > state/tts_usage.json
