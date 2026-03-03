package agents

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"try-go-secrets/pkg/models"
	"try-go-secrets/pkg/services"
	"try-go-secrets/pkg/state"
)

var selectorLinePattern = regexp.MustCompile(`(?i)line-(\d+)`)

type ContentSelector struct {
	RawDir    string
	Processed *state.ProcessedState
	Parser    *services.ContentParser
	rng       *rand.Rand
}

func NewContentSelector(rawDir string, processed *state.ProcessedState, parser *services.ContentParser) *ContentSelector {
	return &ContentSelector{
		RawDir:    rawDir,
		Processed: processed,
		Parser:    parser,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *ContentSelector) Select(ctx context.Context, requestedNumber *int) (models.Content, error) {
	if err := ctx.Err(); err != nil {
		return models.Content{}, err
	}
	candidates, err := s.listMarkdownFiles()
	if err != nil {
		return models.Content{}, err
	}
	if len(candidates) == 0 {
		return models.Content{}, fmt.Errorf("no markdown files found in %q", s.RawDir)
	}

	var chosen string
	if requestedNumber != nil {
		chosen, err = s.pickByNumber(candidates, *requestedNumber)
	} else {
		chosen, err = s.pickRandomUnprocessed(candidates)
	}
	if err != nil {
		return models.Content{}, err
	}
	content, err := s.Parser.ParseFile(chosen)
	if err != nil {
		return models.Content{}, fmt.Errorf("parse selected content: %w", err)
	}
	return content, nil
}

func (s *ContentSelector) listMarkdownFiles() ([]string, error) {
	entries, err := os.ReadDir(s.RawDir)
	if err != nil {
		return nil, fmt.Errorf("read raw dir: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		files = append(files, filepath.Join(s.RawDir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func (s *ContentSelector) pickByNumber(files []string, target int) (string, error) {
	for _, file := range files {
		number, ok := extractLineNumber(filepath.Base(file))
		if !ok || number != target {
			continue
		}
		return file, nil
	}
	return "", fmt.Errorf("content with number %d not found", target)
}

func (s *ContentSelector) pickRandomUnprocessed(files []string) (string, error) {
	unprocessed := make([]string, 0, len(files))
	for _, file := range files {
		if s.Processed != nil && s.Processed.IsProcessed(file) {
			continue
		}
		unprocessed = append(unprocessed, file)
	}
	if len(unprocessed) == 0 {
		return "", fmt.Errorf("no unprocessed markdown files available")
	}
	return unprocessed[s.rng.Intn(len(unprocessed))], nil
}

func extractLineNumber(name string) (int, bool) {
	match := selectorLinePattern.FindStringSubmatch(name)
	if len(match) != 2 {
		return 0, false
	}
	number, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return number, true
}
