package services

import (
	"strings"
	"testing"
)

func TestContentParser_ParseMarkdown(t *testing.T) {
	input := `# Defer Secret
Some explanation before code.

` + "```go" + `
package main
func main() { println("hi") }
` + "```" + `

Diagram:
` + "```mermaid" + `
graph TD
A-->B
` + "```" + `
`

	parser := NewContentParser()
	content, err := parser.ParseMarkdown(input)
	if err != nil {
		t.Fatalf("ParseMarkdown returned error: %v", err)
	}
	if content.Title != "Defer Secret" {
		t.Fatalf("unexpected title: %q", content.Title)
	}
	if len(content.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(content.Blocks))
	}
	if content.Blocks[0].Language != "go" {
		t.Fatalf("expected first block language go, got %q", content.Blocks[0].Language)
	}
	if content.Blocks[1].Language != "mermaid" {
		t.Fatalf("expected second block language mermaid, got %q", content.Blocks[1].Language)
	}
	if containsFence(content.Body) {
		t.Fatalf("expected body without fenced markers, got %q", content.Body)
	}
}

func TestContentParser_ParseMarkdownUnclosedFence(t *testing.T) {
	input := "# Title\n\n```go\nfmt.Println(\"x\")\n"
	parser := NewContentParser()
	content, err := parser.ParseMarkdown(input)
	if err != nil {
		t.Fatalf("ParseMarkdown returned error: %v", err)
	}
	if len(content.Blocks) != 1 {
		t.Fatalf("expected unclosed fence to be captured as 1 block, got %d", len(content.Blocks))
	}
}

func containsFence(s string) bool {
	return strings.Contains(s, "```")
}
