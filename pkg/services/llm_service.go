package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"try-go-secrets/pkg/models"
)

const defaultZAIBaseURL = "https://api.z.ai/v1"

type ScriptGenerator interface {
	GenerateNarration(ctx context.Context, content models.Content) (string, error)
}

type LLMServiceOptions struct {
	APIKey       string
	BaseURL      string
	Model        string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	HTTPClient   *http.Client
}

type LLMService struct {
	apiKey       string
	model        string
	endpoint     string
	maxRetries   int
	retryBackoff time.Duration
	client       *http.Client
}

func NewLLMService(opts LLMServiceOptions) *LLMService {
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = defaultZAIBaseURL
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "glm-coding-pro"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	maxRetries := opts.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	retryBackoff := opts.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = 500 * time.Millisecond
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &LLMService{
		apiKey:       strings.TrimSpace(opts.APIKey),
		model:        model,
		endpoint:     buildChatCompletionsURL(baseURL),
		maxRetries:   maxRetries,
		retryBackoff: retryBackoff,
		client:       client,
	}
}

func (s *LLMService) GenerateNarration(ctx context.Context, content models.Content) (string, error) {
	if strings.TrimSpace(content.Body) == "" {
		return "", fmt.Errorf("content body is empty")
	}
	if s.apiKey == "" {
		return "", fmt.Errorf("ZAI_API_KEY is empty")
	}
	requestBody := map[string]any{
		"model": s.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Ты создаешь короткий сценарий на русском для YouTube Shorts (<60 сек).",
			},
			{
				"role": "user",
				"content": fmt.Sprintf("Тема: %s\nКонтент:\n%s\nНапиши живой сценарий без markdown.",
					content.Title, content.Body),
			},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("encode llm request: %w", err)
	}
	body, statusCode, err := s.sendWithRetry(ctx, payload)
	if err != nil {
		return "", err
	}
	if statusCode >= 300 {
		return "", fmt.Errorf("llm error status=%d body=%s", statusCode, strings.TrimSpace(string(body)))
	}

	text, err := parseLLMText(body)
	if err != nil {
		return "", err
	}
	return text, nil
}

func (s *LLMService) sendWithRetry(ctx context.Context, payload []byte) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, 0, fmt.Errorf("create llm request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.apiKey)

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send llm request: %w", err)
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
			lastErr = fmt.Errorf("read llm response: %w", readErr)
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

		lastErr = fmt.Errorf("llm temporary status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		if err := sleepWithContext(ctx, backoffDuration(s.retryBackoff, attempt)); err != nil {
			return nil, 0, err
		}
	}
	return nil, 0, lastErr
}

func parseLLMText(raw []byte) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", fmt.Errorf("empty llm response body")
	}
	var response struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if response.Error != nil {
		return "", fmt.Errorf("llm api error: %s", strings.TrimSpace(response.Error.Message))
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("llm response has no choices")
	}
	choice := response.Choices[0]
	text := strings.TrimSpace(choice.Text)
	if text == "" {
		text = strings.TrimSpace(extractContentText(choice.Message.Content))
	}
	if text == "" {
		return "", fmt.Errorf("llm returned empty content")
	}
	return text, nil
}

func extractContentText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			switch typed := item.(type) {
			case string:
				parts = append(parts, typed)
			case map[string]any:
				if text, ok := typed["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	default:
		return ""
	}
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func buildChatCompletionsURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultZAIBaseURL
	}
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSuffix(base, "/") + "/chat/completions"
	}
	parsed.Path = path.Join(parsed.Path, "chat/completions")
	return parsed.String()
}

func backoffDuration(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	if attempt <= 0 {
		return base
	}
	maxShift := attempt
	if maxShift > 6 {
		maxShift = 6
	}
	return base * time.Duration(1<<maxShift)
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
