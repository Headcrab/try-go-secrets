package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"try-go-secrets/pkg/models"
	"try-go-secrets/pkg/services"
)

type ScriptWriter struct {
	Generator        services.ScriptGenerator
	MaxDurationSec   int
	CharactersPerSec float64
	ScriptOutputDir  string
	Now              func() time.Time
}

func NewScriptWriter(generator services.ScriptGenerator, maxDurationSec int, scriptOutputDir string) *ScriptWriter {
	return &ScriptWriter{
		Generator:        generator,
		MaxDurationSec:   maxDurationSec,
		CharactersPerSec: 8,
		ScriptOutputDir:  scriptOutputDir,
		Now:              func() time.Time { return time.Now().UTC() },
	}
}

func (w *ScriptWriter) Write(ctx context.Context, content models.Content) (models.Script, string, error) {
	text, err := w.Generator.GenerateNarration(ctx, content)
	if err != nil {
		return models.Script{}, "", fmt.Errorf("generate narration: %w", err)
	}
	segments, total := w.segmentAndTime(text)
	if total > float64(w.MaxDurationSec) {
		segments, total = w.trimToMaxDuration(segments, float64(w.MaxDurationSec))
	}
	if len(segments) == 0 {
		return models.Script{}, "", fmt.Errorf("generated script has no segments")
	}
	script := models.Script{
		ContentID:        content.ID,
		ContentSlug:      content.Slug,
		SourcePath:       content.FilePath,
		GeneratedBy:      "llm_service",
		Segments:         segments,
		TotalDurationSec: total,
		CreatedAt:        w.Now(),
	}

	if err := os.MkdirAll(w.ScriptOutputDir, 0o755); err != nil {
		return models.Script{}, "", fmt.Errorf("create script output dir: %w", err)
	}
	fileName := fmt.Sprintf("%s-%s.json", script.CreatedAt.Format("2006-01-02"), content.Slug)
	outputPath := filepath.Join(w.ScriptOutputDir, fileName)
	data, err := json.MarshalIndent(script, "", "  ")
	if err != nil {
		return models.Script{}, "", fmt.Errorf("encode script json: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return models.Script{}, "", fmt.Errorf("write script json: %w", err)
	}
	return script, outputPath, nil
}

func (w *ScriptWriter) segmentAndTime(text string) ([]models.ScriptSegment, float64) {
	parts := strings.Split(text, ".")
	segments := make([]models.ScriptSegment, 0, len(parts))
	var total float64
	order := 1
	for _, part := range parts {
		segmentText := strings.TrimSpace(part)
		if segmentText == "" {
			continue
		}
		segmentText += "."
		duration := float64(utf8.RuneCountInString(segmentText)) / w.CharactersPerSec
		if duration < 1 {
			duration = 1
		}
		segments = append(segments, models.ScriptSegment{
			Order:       order,
			Text:        segmentText,
			DurationSec: duration,
			ActionCue:   actionCueForOrder(order),
		})
		total += duration
		order++
	}
	return segments, total
}

func actionCueForOrder(order int) string {
	cues := []string{
		"герой врывается в кадр и формулирует проблему",
		"герой показывает код до исправления",
		"герой меняет подход и запускает решение",
		"герой объясняет выигрыш по скорости и стабильности",
		"герой завершает выводом и призывом попробовать",
	}
	if len(cues) == 0 {
		return ""
	}
	index := (order - 1) % len(cues)
	if index < 0 {
		index = 0
	}
	return cues[index]
}

func (w *ScriptWriter) trimToMaxDuration(segments []models.ScriptSegment, max float64) ([]models.ScriptSegment, float64) {
	out := make([]models.ScriptSegment, 0, len(segments))
	var total float64
	for _, segment := range segments {
		if total+segment.DurationSec > max {
			break
		}
		out = append(out, segment)
		total += segment.DurationSec
	}
	return out, total
}
