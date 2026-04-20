package productenrich

import (
	"context"
	"strings"
	"time"
)

type PipelineState struct {
	Task        *Task
	ParsedInput *ParsedInput
	Strategy    ProcessingStrategy
	Validation  *ValidationResult
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
	strategy, validation, err := s.validateAndSelectStrategy(ctx, state.Task, state.ParsedInput)
	if err != nil {
		return err
	}
	state.Strategy = strategy
	state.Validation = validation
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
	attachProductEvidence(productJSON, state.ParsedInput)
	productJSON.QualityScoring = buildQualityScoringMetadata(state.Validation)
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

func attachProductEvidence(productJSON *ProductJSON, parsedInput *ParsedInput) {
	if productJSON == nil || parsedInput == nil || parsedInput.ScrapedData == nil {
		return
	}
	if productJSON.Evidence == nil {
		productJSON.Evidence = map[string][]CanonicalSource{}
	}
	scraped := parsedInput.ScrapedData
	if title := truncateEvidenceDetail(scraped.Title); title != "" {
		productJSON.Evidence["title"] = append(productJSON.Evidence["title"], CanonicalSource{
			Type:   CanonicalSourceScrapedData,
			Detail: `scraped title: "` + title + `"`,
		})
	}
	if description := truncateEvidenceDetail(scraped.Description); description != "" {
		productJSON.Evidence["description"] = append(productJSON.Evidence["description"], CanonicalSource{
			Type:   CanonicalSourceScrapedData,
			Detail: `scraped description: "` + description + `"`,
		})
	}
	for key, value := range scraped.Specs {
		key = strings.TrimSpace(key)
		value = truncateEvidenceDetail(value)
		if key == "" || value == "" {
			continue
		}
		source := CanonicalSource{
			Type:   CanonicalSourceScrapedData,
			Detail: "scraped spec " + key + ": " + value,
		}
		productJSON.Evidence["attributes."+key] = append(productJSON.Evidence["attributes."+key], source)
		productJSON.Evidence["specifications.technical."+key] = append(productJSON.Evidence["specifications.technical."+key], source)
	}
}

func truncateEvidenceDetail(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if value == "" {
		return ""
	}
	if len(value) <= 120 {
		return value
	}
	return value[:117] + "..."
}
