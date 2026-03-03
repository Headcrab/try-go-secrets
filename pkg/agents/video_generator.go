package agents

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"try-go-secrets/pkg/models"
	"try-go-secrets/pkg/services"
	"try-go-secrets/pkg/state"
)

type VideoGenerator struct {
	TTS            services.TTSSynthesizer
	ImageGenerator services.SceneImageGenerator
	Renderer       services.VideoRenderer
	TTSUsage       *state.TTSUsage
	TTSDailyLimit  int
	AudioOutputDir string
	ImageOutputDir string
	VideoOutputDir string
	HeroProfile    string
	Now            func() time.Time
}

func NewVideoGenerator(
	tts services.TTSSynthesizer,
	imageGenerator services.SceneImageGenerator,
	renderer services.VideoRenderer,
	usage *state.TTSUsage,
	ttsDailyLimit int,
	audioOutputDir, imageOutputDir, videoOutputDir, heroProfile string,
) *VideoGenerator {
	return &VideoGenerator{
		TTS:            tts,
		ImageGenerator: imageGenerator,
		Renderer:       renderer,
		TTSUsage:       usage,
		TTSDailyLimit:  ttsDailyLimit,
		AudioOutputDir: audioOutputDir,
		ImageOutputDir: imageOutputDir,
		VideoOutputDir: videoOutputDir,
		HeroProfile:    heroProfile,
		Now:            func() time.Time { return time.Now().UTC() },
	}
}

func (g *VideoGenerator) Generate(ctx context.Context, script models.Script, spec models.VideoSpec) (string, []string, error) {
	audioPaths := make([]string, 0, len(script.Segments))
	runStamp := g.Now().Format("2006-01-02-150405.000")

	for _, segment := range script.Segments {
		chars := utf8.RuneCountInString(segment.Text)
		if err := g.TTSUsage.Consume(chars, g.TTSDailyLimit, g.Now()); err != nil {
			return "", nil, fmt.Errorf("tts quota check for segment %d: %w", segment.Order, err)
		}
		audioPath := filepath.Join(g.AudioOutputDir, fmt.Sprintf("%s-%s-%02d.wav", runStamp, script.ContentSlug, segment.Order))
		if err := g.TTS.Synthesize(ctx, segment.Text, audioPath); err != nil {
			return "", nil, fmt.Errorf("synthesize segment %d: %w", segment.Order, err)
		}
		audioPaths = append(audioPaths, audioPath)
	}
	scenes, err := g.generateScenes(ctx, script, spec.Title, runStamp)
	if err != nil {
		return "", nil, fmt.Errorf("generate scenes: %w", err)
	}
	spec.Scenes = scenes

	videoPath := filepath.Join(g.VideoOutputDir, fmt.Sprintf("%s-%s.mp4", runStamp, script.ContentSlug))
	if err := g.Renderer.Render(ctx, spec, audioPaths, videoPath); err != nil {
		return "", nil, fmt.Errorf("render video: %w", err)
	}
	return videoPath, audioPaths, nil
}

func (g *VideoGenerator) generateScenes(ctx context.Context, script models.Script, title, datePrefix string) ([]models.VideoScene, error) {
	if len(script.Segments) == 0 {
		return nil, nil
	}
	scenes := make([]models.VideoScene, 0, len(script.Segments))
	var start float64
	for _, segment := range script.Segments {
		duration := segment.DurationSec
		if duration < 1 {
			duration = 1
		}
		caption := compactText(segment.Text, 180)
		action := strings.TrimSpace(segment.ActionCue)
		if action == "" {
			action = defaultSceneAction(segment.Order)
		}
		prompt := g.buildScenePrompt(title, caption, action, segment.Order, len(script.Segments))
		imagePath := filepath.Join(
			g.ImageOutputDir,
			fmt.Sprintf("%s-%s-scene-%02d.png", datePrefix, script.ContentSlug, segment.Order),
		)
		if g.ImageGenerator != nil {
			if err := g.ImageGenerator.Generate(ctx, prompt, imagePath); err != nil {
				return nil, fmt.Errorf("generate scene %d image: %w", segment.Order, err)
			}
		}

		scenes = append(scenes, models.VideoScene{
			Order:       segment.Order,
			StartSec:    start,
			DurationSec: duration,
			Caption:     caption,
			Action:      action,
			Motion:      motionForScene(segment.Order),
			ImagePath:   imagePath,
			Prompt:      prompt,
		})
		start += duration
	}
	return scenes, nil
}

func (g *VideoGenerator) buildScenePrompt(title, caption, action string, order, total int) string {
	hero := strings.TrimSpace(g.HeroProfile)
	if hero == "" {
		hero = "харизматичный инженер в бирюзовой худи"
	}
	return fmt.Sprintf(
		"Vertical cinematic illustration 9:16. Consistent hero: %s. Scene %d of %d. Action: %s. Topic: %s. Context: %s. Dynamic motion, dramatic lighting, modern software lab, no text, no logos, no watermark.",
		hero,
		order,
		total,
		action,
		compactText(title, 120),
		compactText(caption, 200),
	)
}

func defaultSceneAction(order int) string {
	actions := []string{
		"hero detects a dangerous bug and freezes the frame",
		"hero compares broken and fixed code on holographic screens",
		"hero refactors concurrency flow under time pressure",
		"hero launches benchmark and celebrates massive speedup",
		"hero points to the final rule and calls to apply it today",
	}
	index := (order - 1) % len(actions)
	if index < 0 {
		index = 0
	}
	return actions[index]
}

func motionForScene(order int) string {
	motions := []string{"push-in", "pan-left", "pan-right", "tilt-up"}
	index := (order - 1) % len(motions)
	if index < 0 {
		index = 0
	}
	return motions[index]
}

func compactText(value string, maxRunes int) string {
	trimmed := strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	runes := []rune(trimmed)
	if len(runes) <= maxRunes || maxRunes <= 0 {
		return trimmed
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}
