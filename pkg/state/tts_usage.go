package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type TTSUsage struct {
	Date       string `json:"date"`
	Characters int    `json:"characters"`
}

func LoadTTSUsage(path string, now time.Time) (*TTSUsage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &TTSUsage{Date: now.Format(time.DateOnly)}, nil
		}
		return nil, fmt.Errorf("read tts usage: %w", err)
	}
	usage := &TTSUsage{}
	if len(data) == 0 {
		usage.Date = now.Format(time.DateOnly)
		return usage, nil
	}
	if err := json.Unmarshal(data, usage); err != nil {
		return nil, fmt.Errorf("decode tts usage: %w", err)
	}
	usage.ResetIfNewDay(now)
	return usage, nil
}

func (u *TTSUsage) ResetIfNewDay(now time.Time) {
	if u == nil {
		return
	}
	today := now.Format(time.DateOnly)
	if u.Date != today {
		u.Date = today
		u.Characters = 0
	}
}

func (u *TTSUsage) Consume(chars, dailyLimit int, now time.Time) error {
	if u == nil {
		return errors.New("tts usage is nil")
	}
	if chars < 0 {
		return fmt.Errorf("chars cannot be negative: %d", chars)
	}
	u.ResetIfNewDay(now)
	if u.Characters+chars > dailyLimit {
		return fmt.Errorf("tts quota exceeded: requested=%d, current=%d, limit=%d", chars, u.Characters, dailyLimit)
	}
	u.Characters += chars
	return nil
}

func (u *TTSUsage) Save(path string) error {
	if u == nil {
		return errors.New("tts usage is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tts usage: %w", err)
	}
	return writeFileAtomically(path, data, 0o644)
}
