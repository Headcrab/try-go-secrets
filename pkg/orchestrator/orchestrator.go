package orchestrator

import (
	"context"
	"fmt"

	"try-go-secrets/pkg/agents"
	"try-go-secrets/pkg/models"
	"try-go-secrets/pkg/state"
)

type Orchestrator struct {
	Selector  *agents.ContentSelector
	Writer    *agents.ScriptWriter
	Extractor *agents.CodeExtractor
	Generator *agents.VideoGenerator
	Checker   *agents.QualityChecker
	Processed *state.ProcessedState
	TTSUsage  *state.TTSUsage

	ProcessedStatePath string
	TTSUsageStatePath  string
}

func (o *Orchestrator) Run(ctx context.Context, requestedNumber *int) (models.RunResult, error) {
	content, err := o.Selector.Select(ctx, requestedNumber)
	if err != nil {
		return models.RunResult{}, fmt.Errorf("select content: %w", err)
	}
	script, scriptPath, err := o.Writer.Write(ctx, content)
	if err != nil {
		return models.RunResult{}, fmt.Errorf("write script: %w", err)
	}
	videoSpec := o.Extractor.Extract(content)
	videoPath, audioPaths, err := o.Generator.Generate(ctx, script, videoSpec)
	if err != nil {
		return models.RunResult{}, fmt.Errorf("generate video: %w", err)
	}
	if err := o.Checker.CheckAndMark(content, script, videoPath, audioPaths); err != nil {
		return models.RunResult{}, fmt.Errorf("quality check: %w", err)
	}
	if err := o.Processed.Save(o.ProcessedStatePath); err != nil {
		return models.RunResult{}, fmt.Errorf("persist processed state: %w", err)
	}
	if err := o.TTSUsage.Save(o.TTSUsageStatePath); err != nil {
		return models.RunResult{}, fmt.Errorf("persist tts usage: %w", err)
	}
	return models.RunResult{
		ContentPath: content.FilePath,
		ScriptPath:  scriptPath,
		VideoPath:   videoPath,
	}, nil
}
