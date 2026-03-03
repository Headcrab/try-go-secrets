package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTTSUsage_ConsumeAndReset(t *testing.T) {
	now := time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)
	u := &TTSUsage{Date: now.Format(time.DateOnly), Characters: 100}

	if err := u.Consume(50, 200, now); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	if u.Characters != 150 {
		t.Fatalf("expected 150 chars, got %d", u.Characters)
	}
	nextDay := now.Add(24 * time.Hour)
	if err := u.Consume(30, 200, nextDay); err != nil {
		t.Fatalf("Consume next day returned error: %v", err)
	}
	if u.Characters != 30 {
		t.Fatalf("expected reset and consume to 30 chars, got %d", u.Characters)
	}
}

func TestTTSUsage_ConsumeOverLimit(t *testing.T) {
	now := time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)
	u := &TTSUsage{Date: now.Format(time.DateOnly), Characters: 190}
	if err := u.Consume(20, 200, now); err == nil {
		t.Fatalf("expected over-limit error")
	}
}

func TestTTSUsage_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tts_usage.json")
	now := time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)
	u := &TTSUsage{
		Date:       now.Format(time.DateOnly),
		Characters: 123,
	}
	if err := u.Save(path); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	loaded, err := LoadTTSUsage(path, now)
	if err != nil {
		t.Fatalf("LoadTTSUsage returned error: %v", err)
	}
	if loaded.Characters != 123 {
		t.Fatalf("expected 123 chars, got %d", loaded.Characters)
	}
}
