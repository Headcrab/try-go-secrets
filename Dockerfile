# syntax=docker/dockerfile:1.7

FROM golang:1.23-bookworm AS base

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        ffmpeg \
        git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    APP_ENV=development \
    LOG_LEVEL=info \
    STRICT_ENV=false \
    STRICT_REQUIRE_RENDER=false \
    MAX_VIDEO_DURATION_SEC=60 \
    TTS_DAILY_LIMIT=2000

FROM base AS deps
COPY go.mod go.sum* ./
RUN mkdir -p /go/pkg /go/pkg/mod /root/.cache/go-build \
    && if [ -f go.mod ]; then go mod download; fi

FROM base AS dev
COPY --from=deps /go/pkg /go/pkg
COPY . /workspace

CMD ["go", "run", "./cmd"]

FROM base AS builder
COPY --from=deps /go/pkg /go/pkg
COPY . /workspace
RUN go build -o /out/try-go-secrets ./cmd

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        ffmpeg \
    && rm -rf /var/lib/apt/lists/*

ENV APP_ENV=production \
    LOG_LEVEL=info \
    STRICT_ENV=true

WORKDIR /workspace
COPY --from=builder /out/try-go-secrets /usr/local/bin/try-go-secrets
ENTRYPOINT ["try-go-secrets"]
