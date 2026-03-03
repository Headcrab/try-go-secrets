# try-go-secrets

Production-ready пайплайн для генерации вертикальных видео (YouTube Shorts) из Markdown-файлов с Go-секретами:

- сценарий (LLM),
- озвучка (TTS),
- сцены с героем и экшеном (AI images),
- финальный рендер MP4 через Puppeteer + FFmpeg.

Проект оптимизирован под ежедневный запуск, Docker-first workflow и повторные прогоны с кэшированием артефактов.

## Что умеет сейчас

- генерирует видео в формате `1080x1920` (`.mp4`, H.264/AAC);
- поддерживает запуск по номеру (`NUM=43`) или случайный необработанный файл;
- строит динамические сцены с камерой и подписями;
- ведет состояние обработки (`state/processed.json`) и расход TTS (`state/tts_usage.json`);
- переиспользует готовые артефакты (script/audio/images/video), если они уже существуют;
- работает кроссплатформенно через `Taskfile.yml` + POSIX shell scripts.

## Технологии

| Слой | Стек |
|---|---|
| Оркестрация | Go 1.22+ |
| LLM (сценарий) | z.ai / OpenAI-compatible Chat API |
| TTS | Yandex SpeechKit |
| Сцены | OpenAI-compatible Images API |
| Рендер | Node.js service + FFmpeg |
| Инфраструктура | Docker, Docker Compose, Taskfile |

## Структура проекта

```text
cmd/                 # entrypoint
pkg/                 # основная логика (agents, services, state, config)
puppeteer/           # Node.js render service
static/              # CSS/JS для шаблонов
scripts/             # setup/preflight/healthcheck/reset
raw/                 # входные markdown
output/              # scripts/audio/images/videos/logs
state/               # processed + tts usage
```

## Быстрый старт (Docker, рекомендовано)

1. Подготовка:

```bash
task setup
```

2. Заполнить `.env` (минимум: `ZAI_API_KEY`, `YANDEX_API_KEY`, `YANDEX_FOLDER_ID`, `IMAGE_API_KEY`/`OPENAI_API_KEY`).

3. Проверка окружения:

```bash
task preflight:strict:render
```

4. Запуск пайплайна:

```bash
task run:prod:docker NUM=43
```

## Основные команды

```bash
task deps                 # go mod tidy + npm ci
task test                 # go test ./...
task build                # go build ./...
task docker:build         # собрать все образы
task docker:up            # поднять render service
task run:docker NUM=43    # запуск в docker
task run:prod:docker      # strict/prod запуск
task smoke:docker         # smoke-check
task state:reset          # сброс состояния
```

## Ключевые переменные `.env`

- `APP_ENV`, `STRICT_ENV`, `STRICT_REQUIRE_RENDER`
- `RAW_DIR`, `OUTPUT_DIR`, `STATE_DIR`
- `ZAI_API_KEY`, `ZAI_API_BASE_URL`, `ZAI_MODEL`
- `YANDEX_API_KEY`, `YANDEX_FOLDER_ID`, `YANDEX_TTS_*`
- `IMAGE_API_KEY`/`OPENAI_API_KEY`, `IMAGE_API_BASE_URL`, `IMAGE_MODEL`, `IMAGE_SIZE`
- `VIDEO_REQUEST_TIMEOUT_SEC`, `VIDEO_MAX_RETRIES`
- `PUPPETEER_SERVICE_URL`

Актуальный шаблон см. в [.env.example](./.env.example).

## Кэш и повторные запуски

- При `NUM=<id>` контент можно гонять повторно.
- Script очищается от режиссерских/камерных команд для TTS.
- Если артефакты уже есть и валидны, они переиспользуются.
- Если артефакта нет, он создается заново.

## Частые проблемы

1. `llm error 429` (z.ai): лимит/баланс тарифа.
2. `tts error 401` (Yandex): `YANDEX_FOLDER_ID` не совпадает с ключом.
3. `image api invalid size`: используйте `IMAGE_SIZE=1024x1536|1024x1024|1536x1024|auto`.
4. `render timeout`: увеличьте `VIDEO_REQUEST_TIMEOUT_SEC` (например `600`).
5. “кривой” docker-лог: в задачах уже принудительно включен plain-вывод (`--ansi never`, `--progress plain`).

## Локальная разработка

```bash
task run:local NUM=43
task puppeteer:dev
```

Если нужен режим без рендера:

```bash
task run:local:no-render NUM=43
```

## Лицензия и безопасность

- Не коммитьте `.env`, `output/*`, приватные ключи.
- `state/*.json` — runtime-состояние, обычно не должно попадать в PR.
