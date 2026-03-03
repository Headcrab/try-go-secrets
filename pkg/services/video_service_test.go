package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"try-go-secrets/pkg/models"
)

func TestVideoServiceRender_StrictModeRequiresRenderer(t *testing.T) {
	service := NewVideoService(VideoServiceOptions{
		PuppeteerURL: "",
		StrictMode:   true,
	})
	outputPath := filepath.Join(t.TempDir(), "video.mp4")

	err := service.Render(context.Background(), models.VideoSpec{Title: "x"}, nil, outputPath)
	if err == nil {
		t.Fatalf("expected strict mode render error")
	}
	if _, statErr := os.Stat(outputPath); statErr == nil {
		t.Fatalf("video file should not be created in strict mode when renderer is missing")
	}
}

func TestVideoServiceRender_NonStrictWritesPlaceholder(t *testing.T) {
	service := NewVideoService(VideoServiceOptions{
		PuppeteerURL: "",
		StrictMode:   false,
	})
	outputPath := filepath.Join(t.TempDir(), "video.mp4")

	if err := service.Render(context.Background(), models.VideoSpec{Title: "x"}, nil, outputPath); err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read placeholder output: %v", err)
	}
	if !strings.Contains(string(data), "placeholder") {
		t.Fatalf("expected placeholder content")
	}
}
