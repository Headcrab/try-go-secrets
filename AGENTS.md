# Repository Guidelines

## Project Structure & Module Organization
This repository currently contains the implementation spec in `TRY-GO-SECRETS.md`. Build the codebase to match that plan and keep responsibilities separated:

- `cmd/` - application entrypoints (for example `cmd/main.go`).
- `pkg/` - core Go logic (`agents/`, `services/`, `orchestrator/`, `models/`, `state/`).
- `puppeteer/` - Node.js video-rendering service and HTML templates.
- `static/` - frontend assets used by templates (CSS, JS, fonts).
- `scripts/` - local automation helpers (for example `run.sh`).
- `tests/` - integration/end-to-end tests.
- `output/` and `.env` are runtime artifacts/secrets and must stay uncommitted.

## Build, Test, and Development Commands
Use `Taskfile.yml` as the primary workflow entrypoint:

- `task setup` - create runtime dirs/files and bootstrap `.env`.
- `task preflight` - validate env/runtime requirements (strict checks disabled by default).
- `task preflight:strict:render` - strict checks for secrets + render endpoint.
- `task deps` - sync Go and Node dependencies.
- `task test` - run all Go unit/integration tests.
- `task build` - build Go packages.
- `task run NUM=43` - run Docker pipeline for specific content.
- `task run` - run Docker pipeline for random unprocessed content.
- `task run:local NUM=43` - run locally against local Puppeteer service.
- `task run:prod:local NUM=43` - production-mode local run with strict checks.
- `task run:prod:docker NUM=43` - production-mode Docker run with strict checks.
- `task smoke:local` / `task smoke:docker` - smoke checks for local and Docker workflows.
- `task smoke:prod:docker` - strict production smoke validation before long runs.
- `scripts/run.sh` - compatibility wrapper; keep it thin and delegate execution to Task tasks.

## Coding Style & Naming Conventions
- Go: always format with `gofmt` and keep imports clean (`go fmt ./...`).
- Package names: short, lowercase, no underscores (`orchestrator`, `contentparser`).
- Files: snake_case for multiword files (`tts_service.go`, `content_selector.go`).
- Tests: table-driven style for parser/selector logic where possible.
- Node service (`puppeteer/`): prefer modern JS syntax, small modules, and explicit error handling around browser/FFmpeg calls.

## Testing Guidelines
- Unit tests live next to Go code as `*_test.go`.
- Integration tests live in `tests/` and validate end-to-end generation boundaries (duration < 60s, valid MP4, state updates).
- Run `go test ./...` before every PR; include failing-case tests for bug fixes.

## Commit & Pull Request Guidelines
Repository history is currently empty (no commits yet), so use Conventional Commits from the first commit onward:

- `feat: add yandex speechkit client`
- `fix: handle already-processed content id`

PRs should include:
- clear summary and scope,
- linked issue/task,
- test evidence (command output),
- sample output path(s) when behavior changes (for example `output/videos/...mp4`).

## Security & Configuration Tips
- Keep secrets only in `.env`; commit `.env.example` with placeholders.
- Never commit generated media, logs, or API keys.
- Validate daily TTS quota usage before long runs and persist state in `state/*.json`.
