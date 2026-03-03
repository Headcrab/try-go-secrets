package config

import (
	"strings"
	"testing"
)

func TestLoadFromEnv_StrictModeRequiresCriticalEnv(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("STRICT_MODE", "")
	t.Setenv("TTS_ALLOW_FALLBACK", "true")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatalf("expected strict mode validation error")
	}
	if !strings.Contains(err.Error(), "ZAI_API_KEY") {
		t.Fatalf("expected missing ZAI_API_KEY error, got %v", err)
	}
}

func TestLoadFromEnv_ExplicitStrictOffAllowsFallback(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("STRICT_MODE", "false")
	t.Setenv("TTS_ALLOW_FALLBACK", "true")
	t.Setenv("LLM_MAX_RETRIES", "5")
	t.Setenv("LLM_REQUEST_TIMEOUT_SEC", "15")
	t.Setenv("YANDEX_TTS_SPEED", "1.2")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if cfg.StrictMode {
		t.Fatalf("expected strict mode to be disabled by STRICT_MODE=false")
	}
	if !cfg.TTSAllowFallback {
		t.Fatalf("expected explicit fallback in non-strict mode")
	}
	if cfg.LLMMaxRetries != 5 {
		t.Fatalf("unexpected llm max retries: %d", cfg.LLMMaxRetries)
	}
	if cfg.LLMTimeout.Seconds() != 15 {
		t.Fatalf("unexpected llm timeout: %v", cfg.LLMTimeout)
	}
	if cfg.TTSSpeed != 1.2 {
		t.Fatalf("unexpected tts speed: %v", cfg.TTSSpeed)
	}
}

func TestLoadFromEnv_StrictModeFromEnvDisablesFallback(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("APP_ENV", "staging")
	t.Setenv("STRICT_MODE", "true")
	t.Setenv("TTS_ALLOW_FALLBACK", "true")
	t.Setenv("ZAI_API_KEY", "zai-key")
	t.Setenv("YANDEX_API_KEY", "yandex-key")
	t.Setenv("YANDEX_FOLDER_ID", "folder-id")
	t.Setenv("PUPPETEER_SERVICE_URL", "http://localhost:3000")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if !cfg.StrictMode {
		t.Fatalf("expected strict mode enabled")
	}
	if cfg.TTSAllowFallback {
		t.Fatalf("expected fallback to be disabled in strict mode")
	}
}

func setBaseEnv(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("RAW_DIR", tempDir)
	t.Setenv("STATE_DIR", tempDir)
	t.Setenv("OUTPUT_DIR", tempDir)

	keys := []string{
		"APP_ENV",
		"STRICT_MODE",
		"STRICT_ENV",
		"STRICT_REQUIRE_RENDER",
		"PUPPETEER_SERVICE_URL",
		"PUPPETEER_BASE_URL",
		"ZAI_API_KEY",
		"ZAI_API_BASE_URL",
		"ZAI_MODEL",
		"LLM_REQUEST_TIMEOUT_SEC",
		"LLM_MAX_RETRIES",
		"LLM_RETRY_BACKOFF_MS",
		"YANDEX_API_KEY",
		"YANDEX_FOLDER_ID",
		"YANDEX_TTS_VOICE",
		"YANDEX_TTS_EMOTION",
		"YANDEX_TTS_SPEED",
		"YANDEX_TTS_FORMAT",
		"YANDEX_TTS_LANG",
		"YANDEX_TTS_SAMPLE_RATE_HERTZ",
		"TTS_REQUEST_TIMEOUT_SEC",
		"TTS_MAX_RETRIES",
		"TTS_RETRY_BACKOFF_MS",
		"TTS_ALLOW_FALLBACK",
		"VIDEO_REQUEST_TIMEOUT_SEC",
		"VIDEO_MAX_RETRIES",
		"VIDEO_RETRY_BACKOFF_MS",
		"IMAGE_API_KEY",
		"OPENAI_API_KEY",
		"IMAGE_API_BASE_URL",
		"OPENAI_API_BASE_URL",
		"IMAGE_MODEL",
		"OPENAI_IMAGE_MODEL",
		"IMAGE_SIZE",
		"IMAGE_REQUEST_TIMEOUT_SEC",
		"IMAGE_MAX_RETRIES",
		"IMAGE_RETRY_BACKOFF_MS",
		"SCENE_HERO_PROFILE",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}
