package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
