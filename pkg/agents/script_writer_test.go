package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"try-go-secrets/pkg/models"
)

type fakeScriptGenerator struct {
	calls int
	text  string
	err   error
}

func (f *fakeScriptGenerator) GenerateNarration(_ context.Context, _ models.Content) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

func TestScriptWriterWriteReusesExistingScript(t *testing.T) {
	scriptDir := t.TempDir()
	existingPath := filepath.Join(scriptDir, "2026-03-03-120000.000-topic.json")
	existing := models.Script{
		ContentID:        43,
		ContentSlug:      "topic",
		SourcePath:       "raw/topic__line-043.md",
		GeneratedBy:      "llm_service",
		TotalDurationSec: 8,
		CreatedAt:        time.Now().UTC(),
		Segments: []models.ScriptSegment{
			{Order: 1, Text: "hello world.", DurationSec: 2},
			{Order: 2, Text: "second line.", DurationSec: 6},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing script: %v", err)
	}
	if err := os.WriteFile(existingPath, data, 0o644); err != nil {
		t.Fatalf("write existing script: %v", err)
	}

	gen := &fakeScriptGenerator{text: "new text should not be used"}
	writer := NewScriptWriter(gen, 60, scriptDir)

	content := models.Content{
		ID:       43,
		Slug:     "topic",
		FilePath: "raw/topic__line-043.md",
	}
	gotScript, gotPath, err := writer.Write(context.Background(), content)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if gen.calls != 0 {
		t.Fatalf("generator should not be called when script cache exists, calls=%d", gen.calls)
	}
	if gotPath != existingPath {
		t.Fatalf("expected existing script path %q, got %q", existingPath, gotPath)
	}
	if gotScript.ContentSlug != "topic" || len(gotScript.Segments) != 2 {
		t.Fatalf("unexpected script loaded from cache: %+v", gotScript)
	}
}

func TestScriptWriterWriteSanitizesDirectorCommandsFromGeneratedText(t *testing.T) {
	scriptDir := t.TempDir()
	gen := &fakeScriptGenerator{text: "[Видео начинается с анимации]\nНа фоне появляются заголовки.\n[Ведущий:] Привет! Секрет в том, что map работает быстро.\nСмена кадра на график.\nПоявляются визуализации коллизий."}
	writer := NewScriptWriter(gen, 60, scriptDir)

	content := models.Content{
		ID:       43,
		Slug:     "topic",
		FilePath: "raw/topic__line-043.md",
	}
	gotScript, _, err := writer.Write(context.Background(), content)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(gotScript.Segments) == 0 {
		t.Fatalf("expected at least one sanitized segment")
	}
	joined := strings.ToLower(joinSegmentTexts(gotScript.Segments))
	if strings.Contains(joined, "видео начинается") || strings.Contains(joined, "смена кадра") {
		t.Fatalf("director commands should be removed from narration: %q", joined)
	}
	if strings.Contains(joined, "на фоне появляются") || strings.Contains(joined, "появляются визуализации") {
		t.Fatalf("visual control phrases should be removed from narration: %q", joined)
	}
	if !strings.Contains(joined, "привет") {
		t.Fatalf("expected spoken phrase to stay in narration: %q", joined)
	}
}

func TestScriptWriterWriteSanitizesCachedScriptSegments(t *testing.T) {
	scriptDir := t.TempDir()
	existingPath := filepath.Join(scriptDir, "2026-03-03-120000.000-topic.json")
	existing := models.Script{
		ContentID:        43,
		ContentSlug:      "topic",
		SourcePath:       "raw/topic__line-043.md",
		GeneratedBy:      "llm_service",
		TotalDurationSec: 20,
		CreatedAt:        time.Now().UTC(),
		Segments: []models.ScriptSegment{
			{Order: 1, Text: "[Видео начинается с заставки.", DurationSec: 6},
			{Order: 2, Text: "На фоне появляются заголовки.", DurationSec: 6},
			{Order: 3, Text: "] [Ведущий:] Важный момент про map.", DurationSec: 6},
			{Order: 4, Text: "[Смена кадра на диаграмму.", DurationSec: 6},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing script: %v", err)
	}
	if err := os.WriteFile(existingPath, data, 0o644); err != nil {
		t.Fatalf("write existing script: %v", err)
	}

	gen := &fakeScriptGenerator{text: "must not be called"}
	writer := NewScriptWriter(gen, 60, scriptDir)

	content := models.Content{ID: 43, Slug: "topic", FilePath: "raw/topic__line-043.md"}
	gotScript, _, err := writer.Write(context.Background(), content)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if gen.calls != 0 {
		t.Fatalf("generator should not be called when script cache exists, calls=%d", gen.calls)
	}
	if len(gotScript.Segments) != 1 {
		t.Fatalf("expected only spoken segment to remain, got %d", len(gotScript.Segments))
	}
	segmentText := strings.ToLower(gotScript.Segments[0].Text)
	if strings.Contains(segmentText, "смена кадра") || strings.Contains(segmentText, "на фоне появляются") {
		t.Fatalf("cached directives should be removed: %+v", gotScript.Segments[0])
	}

	rewrittenData, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read rewritten cached script: %v", err)
	}
	rewritten := strings.ToLower(string(rewrittenData))
	if strings.Contains(rewritten, "на фоне появляются") {
		t.Fatalf("cached script json should be rewritten without directives: %s", rewritten)
	}
}

func joinSegmentTexts(segments []models.ScriptSegment) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		parts = append(parts, segment.Text)
	}
	return strings.Join(parts, " ")
}
