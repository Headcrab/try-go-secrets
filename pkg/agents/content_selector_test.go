package agents

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"try-go-secrets/pkg/services"
	"try-go-secrets/pkg/state"
)

func TestContentSelector_SelectByNumber(t *testing.T) {
	rawDir := t.TempDir()
	fileA := filepath.Join(rawDir, "topic__line-043.md")
	fileB := filepath.Join(rawDir, "topic__line-202.md")
	writeFile(t, fileA, "# A\ncontent")
	writeFile(t, fileB, "# B\ncontent")

	processed := &state.ProcessedState{ByPath: map[string]state.ProcessedRecord{}}
	selector := NewContentSelector(rawDir, processed, services.NewContentParser())

	target := 43
	content, err := selector.Select(context.Background(), &target)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if content.FilePath != fileA {
		t.Fatalf("expected file %s, got %s", fileA, content.FilePath)
	}
}

func TestContentSelector_SelectRandomSkipsProcessed(t *testing.T) {
	rawDir := t.TempDir()
	fileA := filepath.Join(rawDir, "topic__line-001.md")
	fileB := filepath.Join(rawDir, "topic__line-002.md")
	writeFile(t, fileA, "# A\ncontent")
	writeFile(t, fileB, "# B\ncontent")

	processed := &state.ProcessedState{ByPath: map[string]state.ProcessedRecord{
		fileA: {ContentPath: fileA},
	}}
	selector := NewContentSelector(rawDir, processed, services.NewContentParser())

	content, err := selector.Select(context.Background(), nil)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if content.FilePath != fileB {
		t.Fatalf("expected only unprocessed file %s, got %s", fileB, content.FilePath)
	}
}

func TestContentSelector_SelectByNumberAlreadyProcessed(t *testing.T) {
	rawDir := t.TempDir()
	fileA := filepath.Join(rawDir, "topic__line-043.md")
	writeFile(t, fileA, "# A\ncontent")

	processed := &state.ProcessedState{ByPath: map[string]state.ProcessedRecord{
		fileA: {ContentPath: fileA},
	}}
	selector := NewContentSelector(rawDir, processed, services.NewContentParser())

	target := 43
	_, err := selector.Select(context.Background(), &target)
	if err == nil {
		t.Fatalf("expected error for already processed content")
	}
}

func writeFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
