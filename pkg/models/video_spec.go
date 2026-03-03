package models

type VideoSpec struct {
	Title         string   `json:"title"`
	CodeBlocks    []string `json:"code_blocks"`
	MermaidBlocks []string `json:"mermaid_blocks"`
}

type RunResult struct {
	ContentPath string `json:"content_path"`
	ScriptPath  string `json:"script_path"`
	VideoPath   string `json:"video_path"`
}
