# try-go-secrets bootstrap

This repository is bootstrapped for local/dev execution of a Go pipeline plus a Puppeteer rendering service.

## What is wired

- `Dockerfile`: Go runtime image with FFmpeg.
- `Dockerfile.puppeteer`: Node + Chromium + FFmpeg image for video rendering.
- `docker-compose.yml`: two services (`app`, `puppeteer`) with shared runtime mounts.
- `Taskfile.yml`: primary command entrypoint for setup, preflight validation, smoke checks, local run, and docker run.
- `scripts/run.sh`: thin compatibility shim that delegates to Task tasks (no duplicated run logic).
- `.env.example`: environment template for API/config values.

## Volume mappings

- `./raw -> /workspace/raw` (read-only in `app`)
- `./output -> /workspace/output` (read-write in both services)
- `./state -> /workspace/state` (read-write in `app`)

## Quick start

Prerequisites:
- `bash` (all Task automation scripts are POSIX shell-based).
- `curl` (used for smoke/health checks).

1. Bootstrap:
   - `task setup`
2. Install dependencies:
   - `task deps`
3. Put source markdown files into `raw/` (pattern `*-line-043.md` etc.).
4. Validate environment:
   - Dev defaults: `task preflight`
   - Strict/prod check: `task preflight:strict:render`
5. Run full docker pipeline:
   - `task run NUM=43`
   - or random: `task run`

## Common tasks

- `task preflight` - validate env/runtime prerequisites (`STRICT_ENV=true` enables strict secret checks).
- `task preflight:strict:render` - strict mode and required render endpoint.
- `task test` - run all Go tests.
- `task build` - build all Go packages.
- `task ci` - format + test + build.
- `task puppeteer:dev` - start local Puppeteer service (foreground).
- `task run:local NUM=43` - local run using `PUPPETEER_SERVICE_URL` (defaults to `http://127.0.0.1:3000/render`).
- `task run:local:no-render NUM=43` - force placeholder renderer.
- `task run:docker NUM=43` - run in Docker with Puppeteer.
- `task run:docker:no-render NUM=43` - run in Docker without Puppeteer.
- `task run:prod:local NUM=43` - local production-mode run with strict env checks.
- `task run:prod:docker NUM=43` - Docker production-mode run with strict env checks.
- `task smoke:local` - local smoke check (`go run ./cmd --help`).
- `task smoke:docker` - Docker smoke check (Puppeteer health + app help).
- `task smoke:prod:docker` - strict production-mode Docker smoke check.
- `task state:reset` - reset `state/processed.json` and `state/tts_usage.json`.

## Production run flow

1. Configure secrets in `.env` (replace placeholder values for `ZAI_API_KEY` and `YANDEX_API_KEY`).
2. Validate strict prerequisites:
   - `task preflight:strict:render`
3. Run smoke check:
   - `task smoke:prod:docker`
4. Start production pipeline:
   - `task run:prod:docker NUM=43`
