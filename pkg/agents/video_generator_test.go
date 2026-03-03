package agents

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"try-go-secrets/pkg/models"
	"try-go-secrets/pkg/state"
)

type fakeTTS struct {
	calls int
}

func (f *fakeTTS) Synthesize(_ context.Context, _ string, outputPath string) error {
	f.calls++
	return os.WriteFile(outputPath, []byte("audio"), 0o644)
}

type fakeImageGenerator struct {
	calls int
}

func (f *fakeImageGenerator) Generate(_ context.Context, _ string, outputPath string) error {
	f.calls++
	return os.WriteFile(outputPath, []byte("image"), 0o644)
}

type fakeRenderer struct {
	calls     int
	lastSpec  models.VideoSpec
	lastAudio []string
}

func (f *fakeRenderer) Render(_ context.Context, spec models.VideoSpec, audioPaths []string, outputPath string) error {
	f.calls++
	f.lastSpec = spec
	f.lastAudio = append([]string(nil), audioPaths...)
	return os.WriteFile(outputPath, []byte("video"), 0o644)
}

func TestVideoGeneratorGenerateReusesExistingVideo(t *testing.T) {
	baseDir := t.TempDir()
	audioDir := filepath.Join(baseDir, "audio")
	imageDir := filepath.Join(baseDir, "images")
	videoDir := filepath.Join(baseDir, "videos")
	for _, dir := range []string{audioDir, imageDir, videoDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	tts := &fakeTTS{}
	images := &fakeImageGenerator{}
	renderer := &fakeRenderer{}
	g := NewVideoGenerator(
		tts,
		images,
		renderer,
		&state.TTSUsage{Date: time.Now().Format(time.DateOnly)},
		5000,
		audioDir,
		imageDir,
		videoDir,
		"hero",
	)

	script := models.Script{
		ContentSlug: "topic",
		Segments: []models.ScriptSegment{
			{Order: 1, Text: "a", DurationSec: 1},
		},
	}
	artifactKey := scriptArtifactKey(script)
	existingVideo := filepath.Join(videoDir, "topic-"+artifactKey+".mp4")
	if err := os.WriteFile(existingVideo, []byte("already rendered"), 0o644); err != nil {
		t.Fatalf("write existing video: %v", err)
	}

	videoPath, audioPaths, err := g.Generate(context.Background(), script, models.VideoSpec{Title: "x"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if videoPath != existingVideo {
		t.Fatalf("expected reuse of existing video %q, got %q", existingVideo, videoPath)
	}
	if len(audioPaths) != 0 {
		t.Fatalf("expected empty audio paths on full video reuse, got %d", len(audioPaths))
	}
	if tts.calls != 0 || images.calls != 0 || renderer.calls != 0 {
		t.Fatalf("expected no generation calls, got tts=%d images=%d render=%d", tts.calls, images.calls, renderer.calls)
	}
}

func TestVideoGeneratorGenerateReusesHashedAudioAndImage(t *testing.T) {
	baseDir := t.TempDir()
	audioDir := filepath.Join(baseDir, "audio")
	imageDir := filepath.Join(baseDir, "images")
	videoDir := filepath.Join(baseDir, "videos")
	for _, dir := range []string{audioDir, imageDir, videoDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	tts := &fakeTTS{}
	images := &fakeImageGenerator{}
	renderer := &fakeRenderer{}
	g := NewVideoGenerator(
		tts,
		images,
		renderer,
		&state.TTSUsage{Date: time.Now().Format(time.DateOnly)},
		5000,
		audioDir,
		imageDir,
		videoDir,
		"hero",
	)

	script := models.Script{
		ContentSlug: "topic",
		Segments: []models.ScriptSegment{
			{Order: 1, Text: "segment", DurationSec: 1.5, ActionCue: "do action"},
		},
	}
	artifactKey := scriptArtifactKey(script)
	hashedAudio := filepath.Join(audioDir, "topic-"+artifactKey+"-01.wav")
	hashedImage := filepath.Join(imageDir, "topic-"+artifactKey+"-scene-01.png")
	if err := os.WriteFile(hashedAudio, []byte("audio"), 0o644); err != nil {
		t.Fatalf("write hashed audio: %v", err)
	}
	if err := os.WriteFile(hashedImage, []byte("image"), 0o644); err != nil {
		t.Fatalf("write hashed image: %v", err)
	}

	videoPath, audioPaths, err := g.Generate(context.Background(), script, models.VideoSpec{Title: "title"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if _, err := os.Stat(videoPath); err != nil {
		t.Fatalf("expected rendered video file, stat error: %v", err)
	}
	if len(audioPaths) != 1 || audioPaths[0] != hashedAudio {
		t.Fatalf("expected hashed audio reuse, got %v", audioPaths)
	}
	if renderer.calls != 1 {
		t.Fatalf("expected one render call, got %d", renderer.calls)
	}
	if len(renderer.lastSpec.Scenes) != 1 || renderer.lastSpec.Scenes[0].ImagePath != hashedImage {
		t.Fatalf("expected hashed image in scene, got %+v", renderer.lastSpec.Scenes)
	}
	if tts.calls != 0 || images.calls != 0 {
		t.Fatalf("expected no new tts/image generation, got tts=%d images=%d", tts.calls, images.calls)
	}
}

func TestVideoGeneratorGenerateDoesNotReuseLegacyAudioAndImage(t *testing.T) {
	baseDir := t.TempDir()
	audioDir := filepath.Join(baseDir, "audio")
	imageDir := filepath.Join(baseDir, "images")
	videoDir := filepath.Join(baseDir, "videos")
	for _, dir := range []string{audioDir, imageDir, videoDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	legacyAudio := filepath.Join(audioDir, "2026-03-03-topic-01.wav")
	legacyImage := filepath.Join(imageDir, "2026-03-03-topic-scene-01.png")
	if err := os.WriteFile(legacyAudio, []byte("legacy-audio"), 0o644); err != nil {
		t.Fatalf("write legacy audio: %v", err)
	}
	if err := os.WriteFile(legacyImage, []byte("legacy-image"), 0o644); err != nil {
		t.Fatalf("write legacy image: %v", err)
	}

	tts := &fakeTTS{}
	images := &fakeImageGenerator{}
	renderer := &fakeRenderer{}
	g := NewVideoGenerator(
		tts,
		images,
		renderer,
		&state.TTSUsage{Date: time.Now().Format(time.DateOnly)},
		5000,
		audioDir,
		imageDir,
		videoDir,
		"hero",
	)

	script := models.Script{
		ContentSlug: "topic",
		Segments: []models.ScriptSegment{
			{Order: 1, Text: "clean spoken line", DurationSec: 1.5, ActionCue: "do action"},
		},
	}
	_, audioPaths, err := g.Generate(context.Background(), script, models.VideoSpec{Title: "title"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(audioPaths) != 1 {
		t.Fatalf("expected one audio path, got %d", len(audioPaths))
	}
	if audioPaths[0] == legacyAudio {
		t.Fatalf("legacy audio must not be reused")
	}
	if tts.calls != 1 || images.calls != 1 {
		t.Fatalf("expected new tts/image generation, got tts=%d images=%d", tts.calls, images.calls)
	}
}
