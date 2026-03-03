package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"try-go-secrets/pkg/models"
	"try-go-secrets/pkg/state"
)

type QualityChecker struct {
	MaxDurationSec int
	Processed      *state.ProcessedState
	Now            func() time.Time
}

func NewQualityChecker(maxDurationSec int, processed *state.ProcessedState) *QualityChecker {
	return &QualityChecker{
		MaxDurationSec: maxDurationSec,
		Processed:      processed,
		Now:            func() time.Time { return time.Now().UTC() },
	}
}

func (q *QualityChecker) CheckAndMark(content models.Content, script models.Script, videoPath string, audioPaths []string) error {
	if script.TotalDurationSec >= float64(q.MaxDurationSec) {
		return fmt.Errorf("script duration %.2fs exceeds limit %ds", script.TotalDurationSec, q.MaxDurationSec)
	}
	if !strings.EqualFold(filepath.Ext(videoPath), ".mp4") {
		return fmt.Errorf("video file must have .mp4 extension: %s", videoPath)
	}
	info, err := os.Stat(videoPath)
	if err != nil {
		return fmt.Errorf("video file not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("video file is empty: %s", videoPath)
	}
	for _, audioPath := range audioPaths {
		audioInfo, err := os.Stat(audioPath)
		if err != nil {
			return fmt.Errorf("audio file %s not found: %w", audioPath, err)
		}
		if audioInfo.Size() == 0 {
			return fmt.Errorf("audio file %s is empty", audioPath)
		}
	}

	q.Processed.Mark(state.ProcessedRecord{
		ContentID:   content.ID,
		ContentPath: content.FilePath,
		VideoPath:   videoPath,
		ProcessedAt: q.Now(),
	})
	return nil
}
