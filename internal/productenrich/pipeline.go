package productenrich

import (
	"context"
	"time"
)

type PipelineState struct {
	Task        *Task
	ParsedInput *ParsedInput
	Strategy    ProcessingStrategy
	Analysis    *ProductAnalysis
	ProductJSON *ProductJSON
}

type ProductStage interface {
	Name() string
	Run(ctx context.Context, state *PipelineState) error
}

type productStageFunc struct {
	name string
	run  func(ctx context.Context, state *PipelineState) error
}

func (f productStageFunc) Name() string {
	return f.name
}

func (f productStageFunc) Run(ctx context.Context, state *PipelineState) error {
	return f.run(ctx, state)
}

func (s *productService) buildProcessStages() []ProductStage {
	return []ProductStage{
		productStageFunc{name: "parse_input", run: s.runParseStage},
		productStageFunc{name: "validate_strategy", run: s.runValidateStage},
		productStageFunc{name: "analyze_product", run: s.runAnalyzeStage},
		productStageFunc{name: "generate_json", run: s.runGenerateStage},
		productStageFunc{name: "validate_result", run: s.runResultValidationStage},
	}
}

func (s *productService) runPipeline(ctx context.Context, state *PipelineState) error {
	for _, stage := range s.buildProcessStages() {
		stageLog := loggerForServiceProcess(state.Task.ID).WithField("stage", stage.Name())
		startedAt := time.Now()
		stageLog.Info("stage started")

		if err := stage.Run(ctx, state); err != nil {
			stageLog.WithError(err).WithFields(stageFailureFields(err, startedAt)).Error("stage failed")
			return err
		}

		stageLog.WithFields(stageSuccessFields(startedAt)).Info("stage completed")
	}
	return nil
}

func (s *productService) runParseStage(ctx context.Context, state *PipelineState) error {
	parsedInput, err := s.parseInput(ctx, state.Task)
	if err != nil {
		if dbErr := s.taskRepo.MarkFailed(ctx, state.Task.ID, "input parsing failed: "+err.Error()); dbErr != nil {
			loggerForServiceProcess(state.Task.ID).WithField("stage", "parse_input").WithError(dbErr).Error("failed to persist task error")
		}
		return err
	}
	state.ParsedInput = parsedInput
	return nil
}

func (s *productService) runValidateStage(ctx context.Context, state *PipelineState) error {
	strategy, err := s.validateAndSelectStrategy(ctx, state.Task, state.ParsedInput)
	if err != nil {
		return err
	}
	state.Strategy = strategy
	return nil
}

func (s *productService) runAnalyzeStage(ctx context.Context, state *PipelineState) error {
	analysis, err := s.analyzeProduct(ctx, state.Task, state.ParsedInput)
	if err != nil {
		if dbErr := s.taskRepo.MarkFailed(ctx, state.Task.ID, "product analysis failed: "+err.Error()); dbErr != nil {
			loggerForServiceProcess(state.Task.ID).WithField("stage", "analyze_product").WithError(dbErr).Error("failed to persist task error")
		}
		return err
	}
	state.Analysis = analysis
	return nil
}

func (s *productService) runGenerateStage(ctx context.Context, state *PipelineState) error {
	productJSON, err := s.generateProductJSON(ctx, state.Task, state.Analysis, state.Strategy)
	if err != nil {
		if dbErr := s.taskRepo.MarkFailed(ctx, state.Task.ID, "JSON generation failed: "+err.Error()); dbErr != nil {
			loggerForServiceProcess(state.Task.ID).WithField("stage", "generate_json").WithError(dbErr).Error("failed to persist task error")
		}
		return err
	}
	productJSON.Images = state.ParsedInput.Images
	state.ProductJSON = productJSON
	return nil
}

func (s *productService) runResultValidationStage(ctx context.Context, state *PipelineState) error {
	if err := s.validateResult(ctx, state.Task, state.ParsedInput, state.ProductJSON); err != nil {
		if dbErr := s.taskRepo.MarkFailed(ctx, state.Task.ID, err.Error()); dbErr != nil {
			loggerForServiceProcess(state.Task.ID).WithField("stage", "validate_result").WithError(dbErr).Error("failed to persist task error")
		}
		return err
	}
	return nil
}

func stageSuccessFields(startedAt time.Time) map[string]any {
	return map[string]any{
		"outcome":     "success",
		"duration_ms": time.Since(startedAt).Milliseconds(),
	}
}

func stageFailureFields(err error, startedAt time.Time) map[string]any {
	return map[string]any{
		"outcome":             "failed",
		"duration_ms":         time.Since(startedAt).Milliseconds(),
		"failure_disposition": ClassifyProcessFailure(err),
	}
}
