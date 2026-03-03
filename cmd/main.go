package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"try-go-secrets/pkg/agents"
	"try-go-secrets/pkg/config"
	"try-go-secrets/pkg/orchestrator"
	"try-go-secrets/pkg/services"
	"try-go-secrets/pkg/state"
)

func main() {
	if err := run(); err != nil {
		log.Printf("pipeline failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	processed, err := state.LoadProcessed(cfg.ProcessedStatePath)
	if err != nil {
		return fmt.Errorf("load processed state: %w", err)
	}
	ttsUsage, err := state.LoadTTSUsage(cfg.TTSUsageStatePath, nowUTC())
	if err != nil {
		return fmt.Errorf("load tts usage state: %w", err)
	}
	requestedNumber, showHelp, err := parseRequestedNumber(os.Args[1:])
	if err != nil {
		return err
	}
	if showHelp {
		printUsage()
		return nil
	}

	parser := services.NewContentParser()
	selector := agents.NewContentSelector(cfg.RawDir, processed, parser)
	writer := agents.NewScriptWriter(
		services.NewLLMService(services.LLMServiceOptions{
			APIKey:       cfg.ZAIAPIKey,
			BaseURL:      cfg.ZAIAPIBaseURL,
			Model:        cfg.ZAIModel,
			Timeout:      cfg.LLMTimeout,
			MaxRetries:   cfg.LLMMaxRetries,
			RetryBackoff: cfg.LLMRetryBackoff,
		}),
		cfg.MaxVideoDurationSec,
		cfg.OutputScriptsDir,
	)
	extractor := agents.NewCodeExtractor()
	generator := agents.NewVideoGenerator(
		services.NewTTSService(services.TTSServiceOptions{
			APIKey:          cfg.YandexAPIKey,
			FolderID:        cfg.YandexFolderID,
			Voice:           cfg.TTSVoice,
			Emotion:         cfg.TTSEmotion,
			Speed:           cfg.TTSSpeed,
			Format:          cfg.TTSFormat,
			Lang:            cfg.TTSLang,
			SampleRateHertz: cfg.TTSSampleRateHertz,
			Timeout:         cfg.TTSTimeout,
			MaxRetries:      cfg.TTSMaxRetries,
			RetryBackoff:    cfg.TTSRetryBackoff,
			StrictMode:      cfg.StrictMode,
			AllowFallback:   cfg.TTSAllowFallback,
		}),
		services.NewVideoService(services.VideoServiceOptions{
			PuppeteerURL: cfg.PuppeteerServiceURL,
			Timeout:      cfg.VideoTimeout,
			MaxRetries:   cfg.VideoMaxRetries,
			RetryBackoff: cfg.VideoRetryBackoff,
			StrictMode:   cfg.StrictMode,
		}),
		ttsUsage,
		cfg.TTSDailyCharacterLimit,
		cfg.OutputAudioDir,
		cfg.OutputVideosDir,
	)
	checker := agents.NewQualityChecker(cfg.MaxVideoDurationSec, processed)

	orch := orchestrator.Orchestrator{
		Selector:           selector,
		Writer:             writer,
		Extractor:          extractor,
		Generator:          generator,
		Checker:            checker,
		Processed:          processed,
		TTSUsage:           ttsUsage,
		ProcessedStatePath: cfg.ProcessedStatePath,
		TTSUsageStatePath:  cfg.TTSUsageStatePath,
	}

	result, err := orch.Run(context.Background(), requestedNumber)
	if err != nil {
		return err
	}

	log.Printf("content: %s", result.ContentPath)
	log.Printf("script: %s", result.ScriptPath)
	log.Printf("video: %s", result.VideoPath)
	return nil
}

func parseRequestedNumber(args []string) (*int, bool, error) {
	if len(args) == 0 {
		return nil, false, nil
	}
	if len(args) > 1 {
		return nil, false, fmt.Errorf("expected at most one argument: content number")
	}
	switch args[0] {
	case "-h", "--help":
		return nil, true, nil
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, false, fmt.Errorf("invalid content number %q: %w", args[0], err)
	}
	if n < 0 {
		return nil, false, fmt.Errorf("content number must be non-negative, got %d", n)
	}
	return &n, false, nil
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: go run ./cmd [CONTENT_NUM]")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "CONTENT_NUM is optional. If omitted, a random unprocessed markdown file is selected.")
}

func nowUTC() (now time.Time) {
	return time.Now().UTC()
}
