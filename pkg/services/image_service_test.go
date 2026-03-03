package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildImagesGenerationsURL(t *testing.T) {
	got := buildImagesGenerationsURL("https://api.openai.com/v1")
	if got != "https://api.openai.com/v1/images/generations" {
		t.Fatalf("unexpected url: %s", got)
	}

	got = buildImagesGenerationsURL("https://api.openai.com/v1/images/generations")
	if got != "https://api.openai.com/v1/images/generations" {
		t.Fatalf("unexpected already-complete url: %s", got)
	}
}

func TestImageServiceGenerate_NonStrictFallback(t *testing.T) {
	service := NewImageService(ImageServiceOptions{
		APIKey:     "",
		StrictMode: false,
	})
	outputPath := filepath.Join(t.TempDir(), "scene.png")
	if err := service.Generate(context.Background(), "hero runs into a server room", outputPath); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("expected generated fallback image: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("fallback image is empty")
	}
}

func TestImageServiceGenerate_StrictWithoutKeyFails(t *testing.T) {
	service := NewImageService(ImageServiceOptions{
		APIKey:     "",
		StrictMode: true,
	})
	outputPath := filepath.Join(t.TempDir(), "scene.png")
	if err := service.Generate(context.Background(), "hero runs into a server room", outputPath); err == nil {
		t.Fatalf("expected strict mode error without key")
	}
}
