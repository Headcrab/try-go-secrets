package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const defaultImageBaseURL = "https://api.openai.com/v1"

type SceneImageGenerator interface {
	Generate(ctx context.Context, prompt, outputPath string) error
}

type ImageServiceOptions struct {
	APIKey       string
	BaseURL      string
	Model        string
	Size         string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	StrictMode   bool
	HTTPClient   *http.Client
}

type ImageService struct {
	apiKey       string
	model        string
	size         string
	endpoint     string
	maxRetries   int
	retryBackoff time.Duration
	strictMode   bool
	client       *http.Client
}

func NewImageService(opts ImageServiceOptions) *ImageService {
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = defaultImageBaseURL
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "gpt-image-1"
	}
	size := strings.TrimSpace(opts.Size)
	if size == "" {
		size = "1024x1792"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
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

	return &ImageService{
		apiKey:       strings.TrimSpace(opts.APIKey),
		model:        model,
		size:         size,
		endpoint:     buildImagesGenerationsURL(baseURL),
		maxRetries:   maxRetries,
		retryBackoff: retryBackoff,
		strictMode:   opts.StrictMode,
		client:       client,
	}
}

func (s *ImageService) Generate(ctx context.Context, prompt, outputPath string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("image prompt is empty")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create image output dir: %w", err)
	}
	if s.apiKey == "" {
		if s.strictMode {
			return fmt.Errorf("image generation key is empty (set IMAGE_API_KEY or OPENAI_API_KEY)")
		}
		return writeFallbackSceneImage(outputPath, prompt)
	}

	requestBody := map[string]any{
		"model":  s.model,
		"prompt": prompt,
		"size":   s.size,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encode image request: %w", err)
	}

	body, statusCode, err := s.sendWithRetry(ctx, payload)
	if err != nil {
		if s.strictMode {
			return err
		}
		return writeFallbackSceneImage(outputPath, prompt)
	}
	if statusCode >= 300 {
		if s.strictMode {
			return fmt.Errorf("image api error status=%d body=%s", statusCode, strings.TrimSpace(string(body)))
		}
		return writeFallbackSceneImage(outputPath, prompt)
	}

	imageBytes, err := s.parseImageBody(ctx, body)
	if err != nil {
		if s.strictMode {
			return err
		}
		return writeFallbackSceneImage(outputPath, prompt)
	}
	if err := os.WriteFile(outputPath, imageBytes, 0o644); err != nil {
		return fmt.Errorf("write scene image: %w", err)
	}
	return nil
}

func (s *ImageService) sendWithRetry(ctx context.Context, payload []byte) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, 0, fmt.Errorf("create image request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.apiKey)

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send image request: %w", err)
			if attempt < s.maxRetries {
				if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
					return nil, 0, err
				}
				continue
			}
			return nil, 0, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read image response: %w", readErr)
			if attempt < s.maxRetries {
				if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
					return nil, 0, err
				}
				continue
			}
			return nil, 0, lastErr
		}

		if !isRetryableStatus(resp.StatusCode) || attempt == s.maxRetries {
			return body, resp.StatusCode, nil
		}
		lastErr = fmt.Errorf("image temporary status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
			return nil, 0, err
		}
	}
	return nil, 0, lastErr
}

func (s *ImageService) parseImageBody(ctx context.Context, raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty image response body")
	}

	var response struct {
		Error *struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode image response: %w", err)
	}
	if response.Error != nil {
		return nil, fmt.Errorf("image api error: %s", strings.TrimSpace(response.Error.Message))
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("image response has no data")
	}

	item := response.Data[0]
	if strings.TrimSpace(item.B64JSON) != "" {
		decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return nil, fmt.Errorf("decode image b64_json: %w", err)
		}
		return decoded, nil
	}

	imageURL := strings.TrimSpace(item.URL)
	if imageURL == "" {
		return nil, fmt.Errorf("image response item has no b64_json/url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create image download request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download image status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read downloaded image: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("downloaded image is empty")
	}
	return data, nil
}

func buildImagesGenerationsURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultImageBaseURL
	}
	if strings.HasSuffix(base, "/images/generations") {
		return base
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSuffix(base, "/") + "/images/generations"
	}
	parsed.Path = path.Join(parsed.Path, "images", "generations")
	return parsed.String()
}

func writeFallbackSceneImage(outputPath, prompt string) error {
	const (
		width  = 1024
		height = 1792
	)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	seed := uint8(len(prompt)%255 + 1)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x*int(seed) + y) % 255)
			g := uint8((y*2 + int(seed)*5) % 255)
			b := uint8((x + y + int(seed)*13) % 255)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create fallback scene image: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode fallback scene image: %w", err)
	}
	return nil
}
