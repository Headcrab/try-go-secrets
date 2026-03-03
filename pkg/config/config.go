package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRawDir           = "raw"
	defaultStateDir         = "state"
	defaultOutputDir        = "output"
	defaultDailyTTSLimit    = 2000
	defaultMaxVideoDuration = 60
	defaultAppEnv           = "development"
	defaultHeroProfile      = "харизматичный инженер в бирюзовой худи, главный герой техно-истории"

	defaultZAIBaseURL          = "https://api.z.ai/v1"
	defaultZAIModel            = "glm-4.7"
	defaultLLMTimeoutSec       = 20
	defaultLLMMaxRetries       = 3
	defaultLLMRetryBackoffMs   = 500
	defaultTTSTimeoutSec       = 30
	defaultTTSMaxRetries       = 3
	defaultTTSRetryBackoffMs   = 700
	defaultTTSVoice            = "alena"
	defaultTTSEmotion          = "good"
	defaultTTSSpeed            = 1.0
	defaultTTSFormat           = "lpcm"
	defaultTTSLang             = "ru-RU"
	defaultTTSSampleRateHertz  = 48000
	defaultVideoTimeoutSec     = 60
	defaultVideoMaxRetries     = 2
	defaultVideoRetryBackoffMs = 1000
	defaultImageBaseURL        = "https://api.openai.com/v1"
	defaultImageModel          = "gpt-image-1"
	defaultImageSize           = "1024x1792"
	defaultImageTimeoutSec     = 45
	defaultImageMaxRetries     = 2
	defaultImageRetryBackoffMs = 1000
)

type Config struct {
	AppEnv     string
	StrictMode bool

	RawDir                 string
	StateDir               string
	OutputDir              string
	ProcessedStatePath     string
	TTSUsageStatePath      string
	OutputScriptsDir       string
	OutputAudioDir         string
	OutputImagesDir        string
	OutputVideosDir        string
	OutputLogsDir          string
	MaxVideoDurationSec    int
	TTSDailyCharacterLimit int
	HeroProfile            string

	PuppeteerServiceURL string

	ZAIAPIKey       string
	ZAIAPIBaseURL   string
	ZAIModel        string
	LLMTimeout      time.Duration
	LLMMaxRetries   int
	LLMRetryBackoff time.Duration

	YandexAPIKey       string
	YandexFolderID     string
	TTSVoice           string
	TTSEmotion         string
	TTSSpeed           float64
	TTSFormat          string
	TTSLang            string
	TTSSampleRateHertz int
	TTSTimeout         time.Duration
	TTSMaxRetries      int
	TTSRetryBackoff    time.Duration
	TTSAllowFallback   bool

	VideoTimeout      time.Duration
	VideoMaxRetries   int
	VideoRetryBackoff time.Duration

	ImageAPIKey       string
	ImageAPIBaseURL   string
	ImageModel        string
	ImageSize         string
	ImageTimeout      time.Duration
	ImageMaxRetries   int
	ImageRetryBackoff time.Duration
}

func LoadFromEnv() (Config, error) {
	rawDir := getEnv("RAW_DIR", defaultRawDir)
	stateDir := getEnv("STATE_DIR", defaultStateDir)
	outputDir := getEnv("OUTPUT_DIR", defaultOutputDir)
	appEnv := normalizeAppEnv(getEnv("APP_ENV", defaultAppEnv))
	strictMode, err := parseStrictMode(appEnv)
	if err != nil {
		return Config{}, err
	}
	maxDur, err := getEnvInt("MAX_VIDEO_DURATION_SEC", defaultMaxVideoDuration)
	if err != nil {
		return Config{}, fmt.Errorf("parse MAX_VIDEO_DURATION_SEC: %w", err)
	}
	if maxDur <= 0 {
		return Config{}, fmt.Errorf("MAX_VIDEO_DURATION_SEC must be > 0")
	}
	ttsLimit, err := getEnvInt("TTS_DAILY_LIMIT", defaultDailyTTSLimit)
	if err != nil {
		return Config{}, fmt.Errorf("parse TTS_DAILY_LIMIT: %w", err)
	}
	if ttsLimit <= 0 {
		return Config{}, fmt.Errorf("TTS_DAILY_LIMIT must be > 0")
	}
	llmTimeoutSec, err := getEnvInt("LLM_REQUEST_TIMEOUT_SEC", defaultLLMTimeoutSec)
	if err != nil {
		return Config{}, fmt.Errorf("parse LLM_REQUEST_TIMEOUT_SEC: %w", err)
	}
	llmRetries, err := getEnvInt("LLM_MAX_RETRIES", defaultLLMMaxRetries)
	if err != nil {
		return Config{}, fmt.Errorf("parse LLM_MAX_RETRIES: %w", err)
	}
	llmBackoffMS, err := getEnvInt("LLM_RETRY_BACKOFF_MS", defaultLLMRetryBackoffMs)
	if err != nil {
		return Config{}, fmt.Errorf("parse LLM_RETRY_BACKOFF_MS: %w", err)
	}
	ttsTimeoutSec, err := getEnvInt("TTS_REQUEST_TIMEOUT_SEC", defaultTTSTimeoutSec)
	if err != nil {
		return Config{}, fmt.Errorf("parse TTS_REQUEST_TIMEOUT_SEC: %w", err)
	}
	ttsRetries, err := getEnvInt("TTS_MAX_RETRIES", defaultTTSMaxRetries)
	if err != nil {
		return Config{}, fmt.Errorf("parse TTS_MAX_RETRIES: %w", err)
	}
	ttsBackoffMS, err := getEnvInt("TTS_RETRY_BACKOFF_MS", defaultTTSRetryBackoffMs)
	if err != nil {
		return Config{}, fmt.Errorf("parse TTS_RETRY_BACKOFF_MS: %w", err)
	}
	videoTimeoutSec, err := getEnvInt("VIDEO_REQUEST_TIMEOUT_SEC", defaultVideoTimeoutSec)
	if err != nil {
		return Config{}, fmt.Errorf("parse VIDEO_REQUEST_TIMEOUT_SEC: %w", err)
	}
	videoRetries, err := getEnvInt("VIDEO_MAX_RETRIES", defaultVideoMaxRetries)
	if err != nil {
		return Config{}, fmt.Errorf("parse VIDEO_MAX_RETRIES: %w", err)
	}
	videoBackoffMS, err := getEnvInt("VIDEO_RETRY_BACKOFF_MS", defaultVideoRetryBackoffMs)
	if err != nil {
		return Config{}, fmt.Errorf("parse VIDEO_RETRY_BACKOFF_MS: %w", err)
	}
	imageTimeoutSec, err := getEnvInt("IMAGE_REQUEST_TIMEOUT_SEC", defaultImageTimeoutSec)
	if err != nil {
		return Config{}, fmt.Errorf("parse IMAGE_REQUEST_TIMEOUT_SEC: %w", err)
	}
	imageRetries, err := getEnvInt("IMAGE_MAX_RETRIES", defaultImageMaxRetries)
	if err != nil {
		return Config{}, fmt.Errorf("parse IMAGE_MAX_RETRIES: %w", err)
	}
	imageBackoffMS, err := getEnvInt("IMAGE_RETRY_BACKOFF_MS", defaultImageRetryBackoffMs)
	if err != nil {
		return Config{}, fmt.Errorf("parse IMAGE_RETRY_BACKOFF_MS: %w", err)
	}
	ttsSpeed, err := getEnvFloat64("YANDEX_TTS_SPEED", defaultTTSSpeed)
	if err != nil {
		return Config{}, fmt.Errorf("parse YANDEX_TTS_SPEED: %w", err)
	}
	ttsAllowFallback, err := getEnvBool("TTS_ALLOW_FALLBACK", false)
	if err != nil {
		return Config{}, fmt.Errorf("parse TTS_ALLOW_FALLBACK: %w", err)
	}
	ttsSampleRate, err := getEnvInt("YANDEX_TTS_SAMPLE_RATE_HERTZ", defaultTTSSampleRateHertz)
	if err != nil {
		return Config{}, fmt.Errorf("parse YANDEX_TTS_SAMPLE_RATE_HERTZ: %w", err)
	}

	cfg := Config{
		AppEnv:                 appEnv,
		StrictMode:             strictMode,
		RawDir:                 rawDir,
		StateDir:               stateDir,
		OutputDir:              outputDir,
		ProcessedStatePath:     filepath.Join(stateDir, "processed.json"),
		TTSUsageStatePath:      filepath.Join(stateDir, "tts_usage.json"),
		OutputScriptsDir:       filepath.Join(outputDir, "scripts"),
		OutputAudioDir:         filepath.Join(outputDir, "audio"),
		OutputImagesDir:        filepath.Join(outputDir, "images"),
		OutputVideosDir:        filepath.Join(outputDir, "videos"),
		OutputLogsDir:          filepath.Join(outputDir, "logs"),
		MaxVideoDurationSec:    maxDur,
		TTSDailyCharacterLimit: ttsLimit,
		HeroProfile:            getEnv("SCENE_HERO_PROFILE", defaultHeroProfile),
		PuppeteerServiceURL:    firstNonEmpty(os.Getenv("PUPPETEER_SERVICE_URL"), os.Getenv("PUPPETEER_BASE_URL")),
		ZAIAPIKey:              os.Getenv("ZAI_API_KEY"),
		ZAIAPIBaseURL:          getEnv("ZAI_API_BASE_URL", defaultZAIBaseURL),
		ZAIModel:               getEnv("ZAI_MODEL", defaultZAIModel),
		LLMTimeout:             time.Duration(llmTimeoutSec) * time.Second,
		LLMMaxRetries:          llmRetries,
		LLMRetryBackoff:        time.Duration(llmBackoffMS) * time.Millisecond,
		YandexAPIKey:           os.Getenv("YANDEX_API_KEY"),
		YandexFolderID:         os.Getenv("YANDEX_FOLDER_ID"),
		TTSVoice:               getEnv("YANDEX_TTS_VOICE", defaultTTSVoice),
		TTSEmotion:             getEnv("YANDEX_TTS_EMOTION", defaultTTSEmotion),
		TTSSpeed:               ttsSpeed,
		TTSFormat:              getEnv("YANDEX_TTS_FORMAT", defaultTTSFormat),
		TTSLang:                getEnv("YANDEX_TTS_LANG", defaultTTSLang),
		TTSSampleRateHertz:     ttsSampleRate,
		TTSTimeout:             time.Duration(ttsTimeoutSec) * time.Second,
		TTSMaxRetries:          ttsRetries,
		TTSRetryBackoff:        time.Duration(ttsBackoffMS) * time.Millisecond,
		TTSAllowFallback:       ttsAllowFallback && !strictMode,
		VideoTimeout:           time.Duration(videoTimeoutSec) * time.Second,
		VideoMaxRetries:        videoRetries,
		VideoRetryBackoff:      time.Duration(videoBackoffMS) * time.Millisecond,
		ImageAPIKey: firstNonEmpty(
			os.Getenv("IMAGE_API_KEY"),
			os.Getenv("OPENAI_API_KEY"),
			os.Getenv("ZAI_API_KEY"),
		),
		ImageAPIBaseURL: firstNonEmpty(
			os.Getenv("IMAGE_API_BASE_URL"),
			os.Getenv("OPENAI_API_BASE_URL"),
			os.Getenv("ZAI_API_BASE_URL"),
			defaultImageBaseURL,
		),
		ImageModel: firstNonEmpty(
			os.Getenv("IMAGE_MODEL"),
			os.Getenv("OPENAI_IMAGE_MODEL"),
			defaultImageModel,
		),
		ImageSize:         getEnv("IMAGE_SIZE", defaultImageSize),
		ImageTimeout:      time.Duration(imageTimeoutSec) * time.Second,
		ImageMaxRetries:   imageRetries,
		ImageRetryBackoff: time.Duration(imageBackoffMS) * time.Millisecond,
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	if err := cfg.ensureDirs(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) ensureDirs() error {
	dirs := []string{
		c.StateDir,
		c.OutputDir,
		c.OutputScriptsDir,
		c.OutputAudioDir,
		c.OutputImagesDir,
		c.OutputVideosDir,
		c.OutputLogsDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %q: %w", dir, err)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return number, nil
}

func getEnvFloat64(key string, fallback float64) (float64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return number, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, err
	}
	return parsed, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseStrictMode(appEnv string) (bool, error) {
	strictRaw := strings.TrimSpace(firstNonEmpty(os.Getenv("STRICT_MODE"), os.Getenv("STRICT_ENV")))
	if strictRaw != "" {
		parsed, err := strconv.ParseBool(strictRaw)
		if err != nil {
			return false, fmt.Errorf("parse STRICT_MODE/STRICT_ENV: %w", err)
		}
		return parsed, nil
	}
	return appEnv == "production", nil
}

func normalizeAppEnv(value string) string {
	env := strings.TrimSpace(strings.ToLower(value))
	switch env {
	case "", "dev":
		return "development"
	case "prod":
		return "production"
	default:
		return env
	}
}

func (c Config) validate() error {
	switch c.AppEnv {
	case "development", "production", "staging", "test":
	default:
		return fmt.Errorf("APP_ENV must be one of development|production|staging|test, got %q", c.AppEnv)
	}
	if c.LLMTimeout <= 0 {
		return fmt.Errorf("LLM_REQUEST_TIMEOUT_SEC must be > 0")
	}
	if c.LLMMaxRetries < 0 {
		return fmt.Errorf("LLM_MAX_RETRIES must be >= 0")
	}
	if c.LLMRetryBackoff <= 0 {
		return fmt.Errorf("LLM_RETRY_BACKOFF_MS must be > 0")
	}
	if strings.TrimSpace(c.ZAIModel) == "" {
		return fmt.Errorf("ZAI_MODEL cannot be empty")
	}
	if strings.TrimSpace(c.ZAIAPIBaseURL) == "" {
		return fmt.Errorf("ZAI_API_BASE_URL cannot be empty")
	}
	if c.TTSTimeout <= 0 {
		return fmt.Errorf("TTS_REQUEST_TIMEOUT_SEC must be > 0")
	}
	if c.TTSMaxRetries < 0 {
		return fmt.Errorf("TTS_MAX_RETRIES must be >= 0")
	}
	if c.TTSRetryBackoff <= 0 {
		return fmt.Errorf("TTS_RETRY_BACKOFF_MS must be > 0")
	}
	if c.TTSSpeed < 0.1 || c.TTSSpeed > 3.0 {
		return fmt.Errorf("YANDEX_TTS_SPEED must be between 0.1 and 3.0")
	}
	if strings.TrimSpace(c.TTSVoice) == "" {
		return fmt.Errorf("YANDEX_TTS_VOICE cannot be empty")
	}
	if strings.TrimSpace(c.TTSFormat) == "" {
		return fmt.Errorf("YANDEX_TTS_FORMAT cannot be empty")
	}
	if strings.TrimSpace(c.TTSLang) == "" {
		return fmt.Errorf("YANDEX_TTS_LANG cannot be empty")
	}
	if c.TTSSampleRateHertz <= 0 {
		return fmt.Errorf("YANDEX_TTS_SAMPLE_RATE_HERTZ must be > 0")
	}
	if c.VideoTimeout <= 0 {
		return fmt.Errorf("VIDEO_REQUEST_TIMEOUT_SEC must be > 0")
	}
	if c.VideoMaxRetries < 0 {
		return fmt.Errorf("VIDEO_MAX_RETRIES must be >= 0")
	}
	if c.VideoRetryBackoff <= 0 {
		return fmt.Errorf("VIDEO_RETRY_BACKOFF_MS must be > 0")
	}
	if c.ImageTimeout <= 0 {
		return fmt.Errorf("IMAGE_REQUEST_TIMEOUT_SEC must be > 0")
	}
	if c.ImageMaxRetries < 0 {
		return fmt.Errorf("IMAGE_MAX_RETRIES must be >= 0")
	}
	if c.ImageRetryBackoff <= 0 {
		return fmt.Errorf("IMAGE_RETRY_BACKOFF_MS must be > 0")
	}
	if strings.TrimSpace(c.ImageAPIBaseURL) == "" {
		return fmt.Errorf("IMAGE_API_BASE_URL cannot be empty")
	}
	if strings.TrimSpace(c.ImageModel) == "" {
		return fmt.Errorf("IMAGE_MODEL cannot be empty")
	}
	if strings.TrimSpace(c.ImageSize) == "" {
		return fmt.Errorf("IMAGE_SIZE cannot be empty")
	}
	if strings.TrimSpace(c.HeroProfile) == "" {
		return fmt.Errorf("SCENE_HERO_PROFILE cannot be empty")
	}
	if c.StrictMode {
		if strings.TrimSpace(c.ZAIAPIKey) == "" {
			return fmt.Errorf("ZAI_API_KEY is required in strict mode")
		}
		if strings.TrimSpace(c.YandexAPIKey) == "" {
			return fmt.Errorf("YANDEX_API_KEY is required in strict mode")
		}
		if strings.TrimSpace(c.YandexFolderID) == "" {
			return fmt.Errorf("YANDEX_FOLDER_ID is required in strict mode")
		}
		if strings.TrimSpace(c.PuppeteerServiceURL) == "" {
			return fmt.Errorf("PUPPETEER_SERVICE_URL is required in strict mode")
		}
		if strings.TrimSpace(c.ImageAPIKey) == "" {
			return fmt.Errorf("IMAGE_API_KEY (or OPENAI_API_KEY) is required in strict mode")
		}
	}
	return nil
}
