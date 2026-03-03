package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type ProcessedRecord struct {
	ContentID   int       `json:"content_id"`
	ContentPath string    `json:"content_path"`
	VideoPath   string    `json:"video_path"`
	ProcessedAt time.Time `json:"processed_at"`
}

type ProcessedState struct {
	ByPath map[string]ProcessedRecord `json:"by_path"`
}

func LoadProcessed(path string) (*ProcessedState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ProcessedState{ByPath: map[string]ProcessedRecord{}}, nil
		}
		return nil, fmt.Errorf("read processed state: %w", err)
	}
	state := &ProcessedState{}
	if len(data) == 0 {
		state.ByPath = map[string]ProcessedRecord{}
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("decode processed state: %w", err)
	}
	if state.ByPath == nil {
		state.ByPath = map[string]ProcessedRecord{}
	}
	return state, nil
}

func (s *ProcessedState) IsProcessed(path string) bool {
	if s == nil {
		return false
	}
	_, ok := s.ByPath[path]
	return ok
}

func (s *ProcessedState) Mark(record ProcessedRecord) {
	if s.ByPath == nil {
		s.ByPath = map[string]ProcessedRecord{}
	}
	s.ByPath[record.ContentPath] = record
}

func (s *ProcessedState) Save(path string) error {
	if s == nil {
		return errors.New("processed state is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode processed state: %w", err)
	}
	return writeFileAtomically(path, data, 0o644)
}

func (s *ProcessedState) Paths() []string {
	if s == nil {
		return nil
	}
	items := make([]string, 0, len(s.ByPath))
	for path := range s.ByPath {
		items = append(items, path)
	}
	sort.Strings(items)
	return items
}
