package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"try-go-secrets/pkg/models"
)

type VideoRenderer interface {
	Render(ctx context.Context, spec models.VideoSpec, audioPaths []string, outputPath string) error
}

type VideoServiceOptions struct {
	PuppeteerURL string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	StrictMode   bool
	HTTPClient   *http.Client
}

type VideoService struct {
	puppeteerURL string
	strictMode   bool
	maxRetries   int
	retryBackoff time.Duration
	client       *http.Client
}

func NewVideoService(opts VideoServiceOptions) *VideoService {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	maxRetries := opts.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	retryBackoff := opts.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = time.Second
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &VideoService{
		puppeteerURL: normalizeRenderURL(opts.PuppeteerURL),
		strictMode:   opts.StrictMode,
		maxRetries:   maxRetries,
		retryBackoff: retryBackoff,
		client:       client,
	}
}

func (s *VideoService) Render(ctx context.Context, spec models.VideoSpec, audioPaths []string, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create video output dir: %w", err)
	}
	if s.puppeteerURL == "" {
		if s.strictMode {
			return fmt.Errorf("renderer is unavailable: PUPPETEER_SERVICE_URL is empty")
		}
		return s.writePlaceholderMP4(spec, outputPath)
	}
	template := "terminal"
	if len(spec.MermaidBlocks) > 0 {
		template = "diagram"
	}

	duration := 5
	if sceneDuration := totalSceneDuration(spec.Scenes); sceneDuration > 0 {
		duration = int(math.Ceil(sceneDuration))
	} else if len(audioPaths) > 0 {
		duration = len(audioPaths) * 4
	}
	if duration > 55 {
		duration = 55
	}
	if duration < 1 {
		duration = 1
	}

	payload, err := json.Marshal(map[string]any{
		"template":        template,
		"durationSeconds": duration,
		"width":           1080,
		"height":          1920,
		"fps":             30,
		"outputDir":       filepath.Dir(outputPath),
		"outputFileName":  filepath.Base(outputPath),
		"title":           spec.Title,
		"codeBlocks":      spec.CodeBlocks,
		"mermaidBlocks":   spec.MermaidBlocks,
		"scenes":          spec.Scenes,
		"audioPaths":      audioPaths,
	})
	if err != nil {
		return fmt.Errorf("encode video render request: %w", err)
	}

	body, contentType, statusCode, err := s.sendWithRetry(ctx, payload)
	if err != nil {
		if s.strictMode {
			return err
		}
		return s.writePlaceholderMP4(spec, outputPath)
	}
	if statusCode >= 300 {
		err := fmt.Errorf("video service status=%d body=%s", statusCode, string(body))
		if s.strictMode {
			return err
		}
		return s.writePlaceholderMP4(spec, outputPath)
	}

	if isJSONResponse(contentType, body) {
		var renderResp struct {
			OutputPath string `json:"outputPath"`
		}
		if err := json.Unmarshal(body, &renderResp); err != nil {
			return fmt.Errorf("decode render json response: %w", err)
		}
		if strings.TrimSpace(renderResp.OutputPath) == "" {
			return fmt.Errorf("video service returned JSON without outputPath")
		}
		if samePath(outputPath, renderResp.OutputPath) {
			return nil
		}
		if err := copyFile(renderResp.OutputPath, outputPath); err != nil {
			return fmt.Errorf("copy rendered video from %q to %q: %w", renderResp.OutputPath, outputPath, err)
		}
		return nil
	}

	if err := os.WriteFile(outputPath, body, 0o644); err != nil {
		return fmt.Errorf("write output video: %w", err)
	}
	return nil
}

func (s *VideoService) sendWithRetry(ctx context.Context, payload []byte) ([]byte, string, int, error) {
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.puppeteerURL, bytes.NewReader(payload))
		if err != nil {
			return nil, "", 0, fmt.Errorf("create video render request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send video render request: %w", err)
			if attempt < s.maxRetries {
				if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
					return nil, "", 0, err
				}
				continue
			}
			return nil, "", 0, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read video render response: %w", readErr)
			if attempt < s.maxRetries {
				if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
					return nil, "", 0, err
				}
				continue
			}
			return nil, "", 0, lastErr
		}

		if !isRetryableStatus(resp.StatusCode) || attempt == s.maxRetries {
			return body, resp.Header.Get("Content-Type"), resp.StatusCode, nil
		}
		lastErr = fmt.Errorf("video render temporary status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
			return nil, "", 0, err
		}
	}
	return nil, "", 0, lastErr
}

func (s *VideoService) writePlaceholderMP4(spec models.VideoSpec, outputPath string) error {
	content := fmt.Sprintf(
		"MVP placeholder MP4\nTitle: %s\nCode blocks: %d\nMermaid blocks: %d\nScenes: %d\n",
		spec.Title, len(spec.CodeBlocks), len(spec.MermaidBlocks), len(spec.Scenes),
	)
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write placeholder mp4: %w", err)
	}
	return nil
}

func normalizeRenderURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/render"
		return parsed.String()
	}
	return raw
}

func isJSONResponse(contentType string, body []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if strings.EqualFold(mediaType, "application/json") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func copyFile(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func totalSceneDuration(scenes []models.VideoScene) float64 {
	var total float64
	for _, scene := range scenes {
		if scene.DurationSec <= 0 {
			continue
		}
		total += scene.DurationSec
	}
	return total
}
