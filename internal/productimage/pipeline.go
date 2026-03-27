package productimage

import (
	"context"
	"fmt"
	"strings"
	"time"

	productenrich "task-processor/internal/productenrich"
)

type PipelineState struct {
	Task       *Task
	Source     *SourceBundle
	Analysis   *productenrich.ProductAnalysis
	Audits     []ImageAudit
	Candidates *ImageCandidateSet
	Result     *ImageProcessResult
}

type Stage interface {
	Name() string
	Run(ctx context.Context, state *PipelineState) error
}

type stageFunc struct {
	name string
	run  func(ctx context.Context, state *PipelineState) error
}

func (f stageFunc) Name() string { return f.name }

func (f stageFunc) Run(ctx context.Context, state *PipelineState) error {
	return f.run(ctx, state)
}

func (s *service) buildStages() []Stage {
	return []Stage{
		stageFunc{name: "parse_source", run: s.runParseStage},
		stageFunc{name: "analyze_context", run: s.runAnalyzeContextStage},
		stageFunc{name: "audit_images", run: s.runAuditStage},
		stageFunc{name: "rank_candidates", run: s.runRankStage},
		stageFunc{name: "extract_subject", run: s.runSubjectStage},
		stageFunc{name: "cleanup_image", run: s.runCleanupStage},
		stageFunc{name: "render_white_bg", run: s.runWhiteBgStage},
		stageFunc{name: "render_gallery", run: s.runGalleryStage},
		stageFunc{name: "assess_quality", run: s.runQualityStage},
		stageFunc{name: "assess_ip_risk", run: s.runIPRiskStage},
		stageFunc{name: "validate_marketplace", run: s.runValidateStage},
		stageFunc{name: "assess_review", run: s.runReviewStage},
		stageFunc{name: "publish_assets", run: s.runPublishStage},
	}
}

func (s *service) runPipeline(ctx context.Context, state *PipelineState) error {
	for _, stage := range s.buildStages() {
		startedAt := time.Now()
		if err := stage.Run(ctx, state); err != nil {
			durationMS := time.Since(startedAt).Milliseconds()
			state.ensureResult()
			state.Result.StageSummaries = append(state.Result.StageSummaries, ImageStageSummary{
				Stage:      stage.Name(),
				Outcome:    "failed",
				DurationMS: durationMS,
				Message:    err.Error(),
			})
			return fmt.Errorf("%s failed after %dms: %w", stage.Name(), durationMS, err)
		}
		state.ensureResult()
		state.Result.StageSummaries = append(state.Result.StageSummaries, ImageStageSummary{
			Stage:      stage.Name(),
			Outcome:    "success",
			DurationMS: time.Since(startedAt).Milliseconds(),
		})
	}
	return nil
}

func (s *PipelineState) ensureResult() {
	if s.Result == nil {
		s.Result = &ImageProcessResult{}
	}
}

func (s *PipelineState) addTrace(stage, imageURL, assetType, outcome string, duration time.Duration, message string) {
	s.ensureResult()
	s.Result.ImageTraces = append(s.Result.ImageTraces, ImageStageTrace{
		Stage:      stage,
		ImageURL:   imageURL,
		AssetType:  assetType,
		Outcome:    outcome,
		DurationMS: duration.Milliseconds(),
		Message:    message,
	})
}

func (s *service) runParseStage(ctx context.Context, state *PipelineState) error {
	if s.sourceParser != nil {
		source, err := s.sourceParser.Parse(ctx, state.Task.Request)
		if err != nil {
			return err
		}
		state.Source = source
		return nil
	}
	if !s.capabilities.AllowSimpleSourceParsing {
		return fmt.Errorf("source parser is not configured in %s mode", s.capabilities.Mode)
	}
	state.Source = &SourceBundle{
		Images:      append([]string(nil), state.Task.Request.ImageURLs...),
		Text:        state.Task.Request.Text,
		ProductURL:  state.Task.Request.ProductURL,
		Marketplace: state.Task.Request.Marketplace,
		Country:     state.Task.Request.Country,
	}
	return nil
}

func (s *service) runAnalyzeContextStage(ctx context.Context, state *PipelineState) error {
	if s.contextAnalyzer != nil {
		analysis, err := s.contextAnalyzer.Analyze(ctx, state.Source)
		if err != nil {
			return err
		}
		state.Analysis = analysis
		if state.Source != nil {
			state.Source.Analysis = analysis
		}
		return nil
	}
	if !s.capabilities.AllowMissingContext {
		return fmt.Errorf("context analyzer is not configured in %s mode", s.capabilities.Mode)
	}
	return nil
}

func (s *service) runAuditStage(ctx context.Context, state *PipelineState) error {
	if s.imageInspector != nil {
		audits := make([]ImageAudit, 0, len(state.Source.Images))
		for _, imageURL := range state.Source.Images {
			startedAt := time.Now()
			audit, err := s.imageInspector.Inspect(ctx, state.Source, imageURL)
			if err != nil {
				state.addTrace("audit_images", imageURL, string(AssetTypeSourceImage), "failed", time.Since(startedAt), err.Error())
				return err
			}
			if audit != nil {
				audits = append(audits, *audit)
				message := fmt.Sprintf("quality=%.2f sharpness=%.2f", audit.QualityScore, audit.SharpnessScore)
				if audit.PrimaryObject != "" {
					message += " object=" + audit.PrimaryObject
				}
				state.addTrace("audit_images", imageURL, string(AssetTypeSourceImage), "success", time.Since(startedAt), message)
			}
		}
		state.Audits = audits
		return nil
	}
	if !s.capabilities.AllowDefaultAudit {
		return fmt.Errorf("image inspector is not configured in %s mode", s.capabilities.Mode)
	}
	state.Audits = make([]ImageAudit, 0, len(state.Source.Images))
	for _, imageURL := range state.Source.Images {
		state.Audits = append(state.Audits, ImageAudit{ImageURL: imageURL, QualityScore: 1, SharpnessScore: 1})
	}
	return nil
}

func (s *service) runRankStage(ctx context.Context, state *PipelineState) error {
	if s.imageRanker != nil {
		candidates, err := s.imageRanker.Select(ctx, state.Source, state.Audits, state.Analysis)
		if err != nil {
			return err
		}
		state.Candidates = candidates
		return nil
	}
	if !s.capabilities.AllowDefaultRanking {
		return fmt.Errorf("image ranker is not configured in %s mode", s.capabilities.Mode)
	}
	candidates := &ImageCandidateSet{}
	if len(state.Source.Images) > 0 {
		candidates.PrimarySource = state.Source.Images[0]
		candidates.HeroCandidates = []string{state.Source.Images[0]}
	}
	if len(state.Source.Images) > 1 {
		candidates.SceneCandidates = append(candidates.SceneCandidates, state.Source.Images[1:]...)
	}
	state.Candidates = candidates
	return nil
}

func (s *service) runSubjectStage(ctx context.Context, state *PipelineState) error {
	state.ensureResult()
	if state.Candidates == nil || state.Candidates.PrimarySource == "" {
		return fmt.Errorf("no primary source image selected")
	}
	if s.reuseExistingAssets && canReuseAsset(state.Result.SubjectCutout) {
		state.addTrace("extract_subject", state.Candidates.PrimarySource, string(AssetTypeSubjectCutout), "reused", 0, "reused existing subject cutout")
		return nil
	}
	if s.subjectExtractor != nil {
		startedAt := time.Now()
		asset, err := s.subjectExtractor.Extract(ctx, state.Candidates.PrimarySource, state.Analysis)
		if err != nil {
			state.addTrace("extract_subject", state.Candidates.PrimarySource, string(AssetTypeSubjectCutout), "failed", time.Since(startedAt), err.Error())
			return err
		}
		state.Result.SubjectCutout = asset
		state.addTrace("extract_subject", state.Candidates.PrimarySource, string(AssetTypeSubjectCutout), "success", time.Since(startedAt), "")
		return nil
	}
	if !s.capabilities.AllowPassThroughMainImage {
		return fmt.Errorf("subject extractor is not configured in %s mode", s.capabilities.Mode)
	}
	state.Result.SubjectCutout = &ImageAsset{URL: state.Candidates.PrimarySource, Type: AssetTypeSubjectCutout, SourceURL: state.Candidates.PrimarySource, Operations: []string{"pass_through_subject"}}
	state.addTrace("extract_subject", state.Candidates.PrimarySource, string(AssetTypeSubjectCutout), "fallback", 0, "pass through subject")
	return nil
}

func (s *service) runCleanupStage(ctx context.Context, state *PipelineState) error {
	if state.Result == nil || state.Result.SubjectCutout == nil {
		return fmt.Errorf("subject cutout is required before cleanup")
	}
	if s.reuseExistingAssets && canReuseAsset(state.Result.MainImage) {
		state.addTrace("cleanup_image", state.Result.SubjectCutout.SourceURL, string(AssetTypeMainImage), "reused", 0, "reused existing main image")
		return nil
	}
	if s.imageCleaner != nil {
		startedAt := time.Now()
		asset, err := s.imageCleaner.Clean(ctx, state.Result.SubjectCutout, state.Analysis)
		if err != nil {
			state.addTrace("cleanup_image", state.Result.SubjectCutout.SourceURL, string(AssetTypeMainImage), "failed", time.Since(startedAt), err.Error())
			return err
		}
		state.Result.MainImage = asset
		state.addTrace("cleanup_image", state.Result.SubjectCutout.SourceURL, string(AssetTypeMainImage), "success", time.Since(startedAt), "")
		return nil
	}
	state.Result.MainImage = &ImageAsset{URL: state.Result.SubjectCutout.URL, Type: AssetTypeMainImage, SourceURL: state.Result.SubjectCutout.SourceURL, Operations: append(append([]string{}, state.Result.SubjectCutout.Operations...), "pass_through_cleanup")}
	state.addTrace("cleanup_image", state.Result.SubjectCutout.SourceURL, string(AssetTypeMainImage), "fallback", 0, "pass through cleanup")
	return nil
}

func (s *service) runWhiteBgStage(ctx context.Context, state *PipelineState) error {
	if state.Result == nil || state.Result.MainImage == nil {
		return fmt.Errorf("main image is required before white background rendering")
	}
	if s.reuseExistingAssets && canReuseAsset(state.Result.WhiteBgImage) {
		state.addTrace("render_white_bg", state.Result.MainImage.SourceURL, string(AssetTypeWhiteBgImage), "reused", 0, "reused existing white background image")
		return nil
	}
	if s.whiteBgRenderer != nil {
		startedAt := time.Now()
		asset, err := s.whiteBgRenderer.Render(ctx, state.Result.MainImage, state.Analysis)
		if err != nil {
			state.addTrace("render_white_bg", state.Result.MainImage.SourceURL, string(AssetTypeWhiteBgImage), "failed", time.Since(startedAt), err.Error())
			return err
		}
		state.Result.WhiteBgImage = asset
		state.addTrace("render_white_bg", state.Result.MainImage.SourceURL, string(AssetTypeWhiteBgImage), "success", time.Since(startedAt), "")
		return nil
	}
	state.Result.WhiteBgImage = &ImageAsset{URL: state.Result.MainImage.URL, Type: AssetTypeWhiteBgImage, SourceURL: state.Result.MainImage.SourceURL, Operations: append(append([]string{}, state.Result.MainImage.Operations...), "pass_through_white_bg")}
	state.addTrace("render_white_bg", state.Result.MainImage.SourceURL, string(AssetTypeWhiteBgImage), "fallback", 0, "pass through white background")
	return nil
}

func (s *service) runGalleryStage(ctx context.Context, state *PipelineState) error {
	state.ensureResult()
	if s.sceneRenderer != nil {
		startedAt := time.Now()
		images, err := s.sceneRenderer.Render(ctx, state.Result.SubjectCutout, state.Analysis)
		if err != nil {
			state.addTrace("render_gallery", state.Result.SubjectCutout.SourceURL, string(AssetTypeGalleryImage), "failed", time.Since(startedAt), err.Error())
			return err
		}
		state.Result.GalleryImages = images
		for _, image := range images {
			state.addTrace("render_gallery", image.SourceURL, string(AssetTypeGalleryImage), "success", time.Since(startedAt), "")
		}
		return nil
	}
	state.Result.GalleryImages = make([]ImageAsset, 0, len(state.Candidates.SceneCandidates))
	for _, imageURL := range state.Candidates.SceneCandidates {
		state.Result.GalleryImages = append(state.Result.GalleryImages, ImageAsset{URL: imageURL, Type: AssetTypeGalleryImage, SourceURL: imageURL, Operations: []string{"pass_through_gallery"}})
		state.addTrace("render_gallery", imageURL, string(AssetTypeGalleryImage), "fallback", 0, "pass through gallery")
	}
	return nil
}

func (s *service) runValidateStage(ctx context.Context, state *PipelineState) error {
	if s.marketValidator != nil {
		report, err := s.marketValidator.Validate(ctx, state.Task.Request, state.Result)
		if err != nil {
			return err
		}
		state.Result.Compliance = report
		if report != nil && !report.Passed {
			return NewNoRetryError(fmt.Errorf("marketplace validation failed"))
		}
		return nil
	}
	if !s.capabilities.AllowMissingValidator {
		return fmt.Errorf("marketplace validator is not configured in %s mode", s.capabilities.Mode)
	}
	state.Result.Compliance = &ComplianceReport{Marketplace: state.Task.Request.Marketplace, Passed: true}
	return nil
}

func (s *service) runQualityStage(ctx context.Context, state *PipelineState) error {
	if state.Result == nil {
		return fmt.Errorf("result is required before quality assessment")
	}
	if s.qualityAssessor == nil {
		return nil
	}
	startedAt := time.Now()
	assessment, err := s.qualityAssessor.Assess(ctx, state.Source, state.Audits, state.Candidates, state.Result)
	if err != nil {
		state.addTrace("assess_quality", "", "", "failed", time.Since(startedAt), err.Error())
		return err
	}
	state.Result.Quality = assessment
	message := ""
	if assessment != nil {
		message = fmt.Sprintf("overall=%.2f main=%.2f white_bg=%.2f", assessment.OverallScore, assessment.MainScore, assessment.WhiteBgScore)
	}
	state.addTrace("assess_quality", "", "", "success", time.Since(startedAt), message)
	return nil
}

func (s *service) runPublishStage(ctx context.Context, state *PipelineState) error {
	if s.assetPublisher == nil || state.Result == nil {
		return nil
	}
	if s.reuseExistingAssets {
		assets := collectAssets(state.Result)
		if len(assets) > 0 {
			allPublished := true
			for _, asset := range assets {
				if !canReusePublishedAsset(asset) {
					allPublished = false
					break
				}
			}
			if allPublished {
				for _, asset := range assets {
					state.addTrace("publish_assets", asset.SourceURL, string(asset.Type), "reused", 0, asset.URL)
				}
				return nil
			}
		}
	}
	startedAt := time.Now()
	err := s.assetPublisher.Publish(ctx, state.Task.Request, state.Result)
	if err != nil {
		state.addTrace("publish_assets", "", "", "failed", time.Since(startedAt), err.Error())
		return err
	}
	for _, asset := range collectAssets(state.Result) {
		state.addTrace("publish_assets", asset.SourceURL, string(asset.Type), "success", time.Since(startedAt), asset.URL)
	}
	return nil
}

func (s *service) runIPRiskStage(_ context.Context, state *PipelineState) error {
	state.ensureResult()
	state.Result.IPRisk = assessImageIPRisk(state.Source, state.Audits)
	if state.Result.IPRisk != nil && state.Result.IPRisk.Level == "high" {
		return NewNoRetryError(fmt.Errorf("image assets have high intellectual property risk"))
	}
	return nil
}

func (s *service) runReviewStage(ctx context.Context, state *PipelineState) error {
	if state.Result == nil {
		return fmt.Errorf("result is required before review assessment")
	}
	if s.reviewAssessor == nil {
		return nil
	}
	startedAt := time.Now()
	decision, err := s.reviewAssessor.Assess(ctx, state.Source, state.Audits, state.Candidates, state.Result)
	if err != nil {
		state.addTrace("assess_review", "", "", "failed", time.Since(startedAt), err.Error())
		return err
	}
	state.Result.Review = decision
	if state.Result.IPRisk != nil && state.Result.IPRisk.Level == "medium" {
		state.Result.Review.NeedsReview = true
		state.Result.Review.Reasons = uniqueStrings(append(state.Result.Review.Reasons, state.Result.IPRisk.Reasons...))
	}
	outcome := "success"
	message := ""
	if decision != nil && decision.NeedsReview {
		outcome = "needs_review"
		message = strings.Join(decision.Reasons, "; ")
	}
	state.addTrace("assess_review", "", "", outcome, time.Since(startedAt), message)
	return nil
}

func collectAssets(result *ImageProcessResult) []*ImageAsset {
	if result == nil {
		return nil
	}
	assets := make([]*ImageAsset, 0, 2+len(result.GalleryImages))
	if result.MainImage != nil {
		assets = append(assets, result.MainImage)
	}
	if result.WhiteBgImage != nil {
		assets = append(assets, result.WhiteBgImage)
	}
	if result.SubjectCutout != nil {
		assets = append(assets, result.SubjectCutout)
	}
	for idx := range result.GalleryImages {
		assets = append(assets, &result.GalleryImages[idx])
	}
	return assets
}
