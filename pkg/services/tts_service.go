package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type TTSSynthesizer interface {
	Synthesize(ctx context.Context, text, outputPath string) error
}

type TTSServiceOptions struct {
	APIKey          string
	FolderID        string
	Voice           string
	Emotion         string
	Speed           float64
	Format          string
	Lang            string
	SampleRateHertz int
	Timeout         time.Duration
	MaxRetries      int
	RetryBackoff    time.Duration
	StrictMode      bool
	AllowFallback   bool
	Endpoint        string
	HTTPClient      *http.Client
}

type TTSService struct {
	apiKey          string
	folderID        string
	voice           string
	emotion         string
	speed           float64
	format          string
	lang            string
	sampleRateHertz int
	strictMode      bool
	allowFallback   bool
	endpoint        string
	maxRetries      int
	retryBackoff    time.Duration
	client          *http.Client
}

func NewTTSService(opts TTSServiceOptions) *TTSService {
	voice := strings.TrimSpace(opts.Voice)
	if voice == "" {
		voice = "alena"
	}
	format := strings.TrimSpace(opts.Format)
	if format == "" {
		format = "lpcm"
	}
	lang := strings.TrimSpace(opts.Lang)
	if lang == "" {
		lang = "ru-RU"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxRetries := opts.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	backoff := opts.RetryBackoff
	if backoff <= 0 {
		backoff = 700 * time.Millisecond
	}
	sampleRateHertz := opts.SampleRateHertz
	if sampleRateHertz <= 0 {
		sampleRateHertz = 48000
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 1.0
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint == "" {
		endpoint = "https://tts.api.cloud.yandex.net/speech/v1/tts:synthesize"
	}

	return &TTSService{
		apiKey:          strings.TrimSpace(opts.APIKey),
		folderID:        strings.TrimSpace(opts.FolderID),
		voice:           voice,
		emotion:         strings.TrimSpace(opts.Emotion),
		speed:           speed,
		format:          format,
		lang:            lang,
		sampleRateHertz: sampleRateHertz,
		strictMode:      opts.StrictMode,
		allowFallback:   opts.AllowFallback && !opts.StrictMode,
		endpoint:        endpoint,
		maxRetries:      maxRetries,
		retryBackoff:    backoff,
		client:          client,
	}
}

func (s *TTSService) Synthesize(ctx context.Context, text, outputPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("tts text cannot be empty")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create audio output dir: %w", err)
	}
	if s.apiKey == "" || s.folderID == "" {
		err := fmt.Errorf("YANDEX_API_KEY and YANDEX_FOLDER_ID are required for tts")
		return s.maybeFallback(outputPath, text, err)
	}

	audioBytes, err := s.sendWithRetry(ctx, text)
	if err != nil {
		return s.maybeFallback(outputPath, text, err)
	}
	if strings.EqualFold(s.format, "lpcm") && strings.EqualFold(filepath.Ext(outputPath), ".wav") {
		audioBytes, err = wrapLPCMAsWAV(audioBytes, s.sampleRateHertz)
		if err != nil {
			return s.maybeFallback(outputPath, text, err)
		}
	}
	if err := os.WriteFile(outputPath, audioBytes, 0o644); err != nil {
		return fmt.Errorf("write tts output: %w", err)
	}
	return nil
}

func (s *TTSService) sendWithRetry(ctx context.Context, text string) ([]byte, error) {
	payload := buildSpeechKitPayload(text, speechKitPayloadOptions{
		folderID:        s.folderID,
		voice:           s.voice,
		emotion:         s.emotion,
		speed:           s.speed,
		format:          s.format,
		lang:            s.lang,
		sampleRateHertz: s.sampleRateHertz,
	})

	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			s.endpoint,
			strings.NewReader(payload.Encode()),
		)
		if err != nil {
			return nil, fmt.Errorf("create tts request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Api-Key "+s.apiKey)

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send tts request: %w", err)
			if attempt < s.maxRetries {
				if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read tts response: %w", readErr)
			if attempt < s.maxRetries {
				if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode < 300 {
			return body, nil
		}

		lastErr = fmt.Errorf("tts error status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		if !isRetryableStatus(resp.StatusCode) || attempt == s.maxRetries {
			return nil, lastErr
		}
		if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

type speechKitPayloadOptions struct {
	folderID        string
	voice           string
	emotion         string
	speed           float64
	format          string
	lang            string
	sampleRateHertz int
}

func buildSpeechKitPayload(text string, opts speechKitPayloadOptions) url.Values {
	values := url.Values{}
	values.Set("text", text)
	values.Set("folderId", opts.folderID)
	values.Set("lang", opts.lang)
	values.Set("voice", opts.voice)
	values.Set("format", opts.format)
	values.Set("speed", strconv.FormatFloat(opts.speed, 'f', -1, 64))
	if strings.TrimSpace(opts.emotion) != "" {
		values.Set("emotion", opts.emotion)
	}
	if strings.EqualFold(opts.format, "lpcm") || strings.EqualFold(opts.format, "oggopus") {
		values.Set("sampleRateHertz", strconv.Itoa(opts.sampleRateHertz))
	}
	return values
}

func (s *TTSService) maybeFallback(outputPath, text string, cause error) error {
	if s.strictMode || !s.allowFallback {
		return cause
	}
	duration := estimateSpeechDuration(text)
	if err := writeSilentWAV(outputPath, duration); err != nil {
		return fmt.Errorf("tts failed (%v) and fallback failed: %w", cause, err)
	}
	return nil
}

func wrapLPCMAsWAV(rawPCM []byte, sampleRate int) ([]byte, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be > 0")
	}
	const (
		bitsPerSample = 16
		channels      = 1
	)
	dataSize := len(rawPCM)
	fileSize := 36 + dataSize

	buf := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	write := func(v any) error {
		return binary.Write(buf, binary.LittleEndian, v)
	}
	if _, err := buf.Write([]byte("RIFF")); err != nil {
		return nil, err
	}
	if err := write(uint32(fileSize)); err != nil {
		return nil, err
	}
	if _, err := buf.Write([]byte("WAVEfmt ")); err != nil {
		return nil, err
	}
	if err := write(uint32(16)); err != nil {
		return nil, err
	}
	if err := write(uint16(1)); err != nil {
		return nil, err
	}
	if err := write(uint16(channels)); err != nil {
		return nil, err
	}
	if err := write(uint32(sampleRate)); err != nil {
		return nil, err
	}
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	if err := write(uint32(byteRate)); err != nil {
		return nil, err
	}
	blockAlign := channels * (bitsPerSample / 8)
	if err := write(uint16(blockAlign)); err != nil {
		return nil, err
	}
	if err := write(uint16(bitsPerSample)); err != nil {
		return nil, err
	}
	if _, err := buf.Write([]byte("data")); err != nil {
		return nil, err
	}
	if err := write(uint32(dataSize)); err != nil {
		return nil, err
	}
	if _, err := buf.Write(rawPCM); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func estimateSpeechDuration(text string) time.Duration {
	runes := utf8.RuneCountInString(text)
	seconds := float64(runes) / 8.0
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds * float64(time.Second))
}

func writeSilentWAV(path string, duration time.Duration) error {
	const (
		sampleRate    = 16000
		bitsPerSample = 16
		channels      = 1
	)
	totalSamples := int(duration.Seconds() * sampleRate)
	if totalSamples <= 0 {
		totalSamples = sampleRate
	}
	dataSize := totalSamples * channels * (bitsPerSample / 8)
	fileSize := 36 + dataSize

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create wav file: %w", err)
	}
	defer file.Close()

	write := func(v any) error {
		return binary.Write(file, binary.LittleEndian, v)
	}

	if _, err := file.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := write(uint32(fileSize)); err != nil {
		return err
	}
	if _, err := file.Write([]byte("WAVEfmt ")); err != nil {
		return err
	}
	if err := write(uint32(16)); err != nil {
		return err
	}
	if err := write(uint16(1)); err != nil { // PCM
		return err
	}
	if err := write(uint16(channels)); err != nil {
		return err
	}
	if err := write(uint32(sampleRate)); err != nil {
		return err
	}
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	if err := write(uint32(byteRate)); err != nil {
		return err
	}
	blockAlign := channels * (bitsPerSample / 8)
	if err := write(uint16(blockAlign)); err != nil {
		return err
	}
	if err := write(uint16(bitsPerSample)); err != nil {
		return err
	}
	if _, err := file.Write([]byte("data")); err != nil {
		return err
	}
	if err := write(uint32(dataSize)); err != nil {
		return err
	}

	silence := make([]byte, dataSize)
	if _, err := file.Write(silence); err != nil {
		return fmt.Errorf("write wav data: %w", err)
	}
	return nil
}
