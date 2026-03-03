package services

import "testing"

func TestParseLLMText_StringContent(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"content":"Привет, мир"}}]}`)

	text, err := parseLLMText(raw)
	if err != nil {
		t.Fatalf("parseLLMText returned error: %v", err)
	}
	if text != "Привет, мир" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestParseLLMText_ArrayContent(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"content":[{"type":"text","text":"Часть 1"},{"type":"text","text":"Часть 2"}]}}]}`)

	text, err := parseLLMText(raw)
	if err != nil {
		t.Fatalf("parseLLMText returned error: %v", err)
	}
	if text != "Часть 1 Часть 2" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestParseLLMText_ErrorEnvelope(t *testing.T) {
	raw := []byte(`{"error":{"message":"quota exceeded"}}`)

	_, err := parseLLMText(raw)
	if err == nil {
		t.Fatalf("expected error for api envelope")
	}
}

func TestBuildChatCompletionsURL(t *testing.T) {
	got := buildChatCompletionsURL("https://api.z.ai/v1")
	if got != "https://api.z.ai/v1/chat/completions" {
		t.Fatalf("unexpected url: %s", got)
	}

	got = buildChatCompletionsURL("https://api.z.ai/v1/chat/completions")
	if got != "https://api.z.ai/v1/chat/completions" {
		t.Fatalf("unexpected already-complete url: %s", got)
	}
}
