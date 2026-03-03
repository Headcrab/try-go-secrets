package agents

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
	"unicode/utf8"

	"try-go-secrets/pkg/models"
	"try-go-secrets/pkg/services"
	"try-go-secrets/pkg/state"
)

type VideoGenerator struct {
	TTS            services.TTSSynthesizer
	Renderer       services.VideoRenderer
	TTSUsage       *state.TTSUsage
	TTSDailyLimit  int
	AudioOutputDir string
	VideoOutputDir string
	Now            func() time.Time
}

func NewVideoGenerator(
	tts services.TTSSynthesizer,
	renderer services.VideoRenderer,
	usage *state.TTSUsage,
	ttsDailyLimit int,
	audioOutputDir, videoOutputDir string,
) *VideoGenerator {
	return &VideoGenerator{
		TTS:            tts,
		Renderer:       renderer,
		TTSUsage:       usage,
		TTSDailyLimit:  ttsDailyLimit,
		AudioOutputDir: audioOutputDir,
		VideoOutputDir: videoOutputDir,
		Now:            func() time.Time { return time.Now().UTC() },
	}
}

func (g *VideoGenerator) Generate(ctx context.Context, script models.Script, spec models.VideoSpec) (string, []string, error) {
	audioPaths := make([]string, 0, len(script.Segments))
	datePrefix := g.Now().Format("2006-01-02")

	for _, segment := range script.Segments {
		chars := utf8.RuneCountInString(segment.Text)
		if err := g.TTSUsage.Consume(chars, g.TTSDailyLimit, g.Now()); err != nil {
			return "", nil, fmt.Errorf("tts quota check for segment %d: %w", segment.Order, err)
		}
		audioPath := filepath.Join(g.AudioOutputDir, fmt.Sprintf("%s-%s-%02d.wav", datePrefix, script.ContentSlug, segment.Order))
		if err := g.TTS.Synthesize(ctx, segment.Text, audioPath); err != nil {
			return "", nil, fmt.Errorf("synthesize segment %d: %w", segment.Order, err)
		}
		audioPaths = append(audioPaths, audioPath)
	}

	videoPath := filepath.Join(g.VideoOutputDir, fmt.Sprintf("%s-%s.mp4", datePrefix, script.ContentSlug))
	if err := g.Renderer.Render(ctx, spec, audioPaths, videoPath); err != nil {
		return "", nil, fmt.Errorf("render video: %w", err)
	}
	return videoPath, audioPaths, nil
}
