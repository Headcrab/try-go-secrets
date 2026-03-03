package agents

import (
	"strings"

	"try-go-secrets/pkg/models"
)

type CodeExtractor struct{}

func NewCodeExtractor() *CodeExtractor {
	return &CodeExtractor{}
}

func (e *CodeExtractor) Extract(content models.Content) models.VideoSpec {
	spec := models.VideoSpec{
		Title:         content.Title,
		CodeBlocks:    make([]string, 0),
		MermaidBlocks: make([]string, 0),
	}
	if spec.Title == "" {
		spec.Title = content.Slug
	}
	for _, block := range content.Blocks {
		lang := strings.ToLower(strings.TrimSpace(block.Language))
		switch lang {
		case "go":
			spec.CodeBlocks = append(spec.CodeBlocks, block.Content)
		case "mermaid":
			spec.MermaidBlocks = append(spec.MermaidBlocks, block.Content)
		}
	}
	return spec
}
