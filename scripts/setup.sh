#!/usr/bin/env bash
set -euo pipefail

mkdir -p raw output/scripts output/audio output/videos output/logs state

if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
fi

if [ ! -f state/processed.json ]; then
  printf '{"by_path":{}}' > state/processed.json
fi

if [ ! -f state/tts_usage.json ]; then
  printf '{"date":"","characters":0}' > state/tts_usage.json
fi

if [ ! -f raw/.gitkeep ]; then
  : > raw/.gitkeep
fi
