package models

import "time"

type FencedBlock struct {
	Language string `json:"language"`
	Content  string `json:"content"`
}

type Content struct {
	ID         int           `json:"id"`
	Slug       string        `json:"slug"`
	Title      string        `json:"title"`
	FilePath   string        `json:"file_path"`
	Body       string        `json:"body"`
	Blocks     []FencedBlock `json:"blocks"`
	ParsedAt   time.Time     `json:"parsed_at"`
	SourceName string        `json:"source_name"`
}
