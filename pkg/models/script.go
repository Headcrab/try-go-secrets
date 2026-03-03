package models

import "time"

type ScriptSegment struct {
	Order       int     `json:"order"`
	Text        string  `json:"text"`
	DurationSec float64 `json:"duration_sec"`
}

type Script struct {
	ContentID        int             `json:"content_id"`
	ContentSlug      string          `json:"content_slug"`
	SourcePath       string          `json:"source_path"`
	GeneratedBy      string          `json:"generated_by"`
	Segments         []ScriptSegment `json:"segments"`
	TotalDurationSec float64         `json:"total_duration_sec"`
	CreatedAt        time.Time       `json:"created_at"`
}
