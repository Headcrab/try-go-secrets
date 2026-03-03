package services

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"try-go-secrets/pkg/models"
)

var lineNumberPattern = regexp.MustCompile(`(?i)line-(\d+)`)

type ContentParser struct{}

func NewContentParser() *ContentParser {
	return &ContentParser{}
}

func (p *ContentParser) ParseFile(path string) (models.Content, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return models.Content{}, fmt.Errorf("read markdown file %q: %w", path, err)
	}
	content, err := p.ParseMarkdown(string(data))
	if err != nil {
		return models.Content{}, err
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	content.FilePath = path
	content.SourceName = base
	content.Slug = strings.TrimSuffix(base, ext)
	content.ID = extractIDFromFilename(base)
	content.ParsedAt = time.Now().UTC()
	return content, nil
}

func (p *ContentParser) ParseMarkdown(markdown string) (models.Content, error) {
	if strings.TrimSpace(markdown) == "" {
		return models.Content{}, fmt.Errorf("markdown content is empty")
	}

	scanner := bufio.NewScanner(strings.NewReader(markdown))
	blocks := make([]models.FencedBlock, 0)
	bodyLines := make([]string, 0)
	title := ""

	var inFence bool
	currentLang := ""
	currentBlock := make([]string, 0)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence = true
				currentLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				currentBlock = currentBlock[:0]
				continue
			}
			blocks = append(blocks, models.FencedBlock{
				Language: strings.ToLower(currentLang),
				Content:  strings.Join(currentBlock, "\n"),
			})
			inFence = false
			currentLang = ""
			currentBlock = currentBlock[:0]
			continue
		}

		if inFence {
			currentBlock = append(currentBlock, line)
			continue
		}

		if title == "" && strings.HasPrefix(trimmed, "#") {
			title = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		}
		bodyLines = append(bodyLines, line)
	}

	if err := scanner.Err(); err != nil {
		return models.Content{}, fmt.Errorf("scan markdown: %w", err)
	}

	if inFence {
		blocks = append(blocks, models.FencedBlock{
			Language: strings.ToLower(currentLang),
			Content:  strings.Join(currentBlock, "\n"),
		})
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return models.Content{
		Title:  title,
		Body:   body,
		Blocks: blocks,
	}, nil
}

func extractIDFromFilename(filename string) int {
	match := lineNumberPattern.FindStringSubmatch(filename)
	if len(match) != 2 {
		return 0
	}
	value := strings.TrimLeft(match[1], "0")
	if value == "" {
		value = "0"
	}
	var id int
	_, _ = fmt.Sscanf(value, "%d", &id)
	return id
}
