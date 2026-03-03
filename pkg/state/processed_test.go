package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestProcessedState_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "processed.json")
	now := time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)

	s := &ProcessedState{ByPath: map[string]ProcessedRecord{}}
	s.Mark(ProcessedRecord{
		ContentID:   43,
		ContentPath: "raw/topic__line-043.md",
		VideoPath:   "output/videos/2026-03-03-topic.mp4",
		ProcessedAt: now,
	})
	if err := s.Save(path); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := LoadProcessed(path)
	if err != nil {
		t.Fatalf("LoadProcessed returned error: %v", err)
	}
	if !loaded.IsProcessed("raw/topic__line-043.md") {
		t.Fatalf("expected path to be marked processed")
	}
	record := loaded.ByPath["raw/topic__line-043.md"]
	if record.ContentID != 43 {
		t.Fatalf("unexpected content id: %d", record.ContentID)
	}
}

func TestLoadProcessedMissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	state, err := LoadProcessed(path)
	if err != nil {
		t.Fatalf("LoadProcessed returned error: %v", err)
	}
	if len(state.ByPath) != 0 {
		t.Fatalf("expected empty state, got %d records", len(state.ByPath))
	}
}
