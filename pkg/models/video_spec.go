package models

type VideoSpec struct {
	Title         string       `json:"title"`
	CodeBlocks    []string     `json:"code_blocks"`
	MermaidBlocks []string     `json:"mermaid_blocks"`
	Scenes        []VideoScene `json:"scenes,omitempty"`
}

type VideoScene struct {
	Order       int     `json:"order"`
	StartSec    float64 `json:"startSec"`
	DurationSec float64 `json:"durationSec"`
	Caption     string  `json:"caption"`
	Action      string  `json:"action"`
	Motion      string  `json:"motion"`
	ImagePath   string  `json:"imagePath"`
	Prompt      string  `json:"prompt,omitempty"`
}

type RunResult struct {
	ContentPath string `json:"content_path"`
	ScriptPath  string `json:"script_path"`
	VideoPath   string `json:"video_path"`
}
