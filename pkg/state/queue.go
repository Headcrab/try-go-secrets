package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func BuildUnprocessedQueue(rawDir string, processed *ProcessedState) ([]string, error) {
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return nil, fmt.Errorf("read raw dir: %w", err)
	}
	queue := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		fullPath := filepath.Join(rawDir, entry.Name())
		if processed != nil && processed.IsProcessed(fullPath) {
			continue
		}
		queue = append(queue, fullPath)
	}
	sort.Strings(queue)
	return queue, nil
}
