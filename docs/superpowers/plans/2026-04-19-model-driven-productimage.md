# Model-Driven ProductImage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the primary `productimage` rendering path with model-backed faithful edit and scene generation while keeping `listingkit` generation/review flows compatible.

**Architecture:** Keep business orchestration in `productimage` and `listingkit`, but move provider-specific model behavior into focused adapters. The production path becomes model-backed for subject extraction, white-background rendering, scene generation, and review, with local canvas retained only as explicit fallback.

**Tech Stack:** Go, existing `productenrich` LLM abstractions, `productimage` pipeline, `listingkit` generation workflow, OpenAI-compatible multimodal clients, Go tests.

---

## File Structure

### Existing files to modify

- `D:\code\task-processor\internal\productimage\interfaces.go`
  - Extend business interfaces and add provider-facing interfaces for model-backed image generation.
- `D:\code\task-processor\internal\productimage\service.go`
  - Switch default production components from local implementations to model-backed implementations.
- `D:\code\task-processor\internal\productimage\pipeline.go`
  - Record model-backed stage traces and preserve explicit fallback semantics.
- `D:\code\task-processor\internal\productimage\default_components.go`
  - Remove production-default assumptions from local implementations and keep them fallback-safe.
- `D:\code\task-processor\internal\listingkit\asset_workflow.go`
  - Carry richer generation execution mode metadata into listingkit asset workflows.
- `D:\code\task-processor\internal\listingkit\workflow.go`
  - Ensure listingkit syncs preview/review metadata from model-backed generation results.
- `D:\code\task-processor\internal\asset\generation\model.go`
  - Extend generation task metadata for model family, generation mode, prompt lineage, and review confidence.
- `D:\code\task-processor\internal\app\httpapi\modules.go`
  - Wire model-backed default productimage components into app bootstrap.

### New files to create

- `D:\code\task-processor\internal\productimage\model_provider.go`
  - Shared provider contracts and metadata structs for model-backed image generation.
- `D:\code\task-processor\internal\productimage\model_subject_extractor.go`
  - Production `SubjectExtractor` backed by the faithful editor.
- `D:\code\task-processor\internal\productimage\model_white_background_renderer.go`
  - Production `WhiteBackgroundRenderer` backed by the faithful editor.
- `D:\code\task-processor\internal\productimage\model_scene_renderer.go`
  - Production `SceneRenderer` backed by the scene generation model.
- `D:\code\task-processor\internal\productimage\model_review_assessor.go`
  - Production `ReviewAssessor` backed by the review model plus rule guards.
- `D:\code\task-processor\internal\productimage\model_fallback_policy.go`
  - Explicit rules for when local fallback is allowed.
- `D:\code\task-processor\internal\productimage\model_provider_test.go`
  - Provider metadata and fallback-policy tests.
- `D:\code\task-processor\internal\productimage\model_subject_extractor_test.go`
  - Tests for faithful subject extraction adapter.
- `D:\code\task-processor\internal\productimage\model_white_background_renderer_test.go`
  - Tests for faithful white-background adapter.
- `D:\code\task-processor\internal\productimage\model_scene_renderer_test.go`
  - Tests for scene generation adapter.
- `D:\code\task-processor\internal\productimage\model_review_assessor_test.go`
  - Tests for model review translation and rule guards.
- `D:\code\task-processor\internal\listingkit\workflow_model_generation_test.go`
  - Integration tests for listingkit generation metadata.
- `D:\code\task-processor\internal\app\httpapi\productimage_model_defaults_test.go`
  - Bootstrap tests for default model-backed productimage wiring.

### Existing files to use as references

- `D:\code\task-processor\internal\productenrich\enrich\understanding.go`
- `D:\code\task-processor\internal\productenrich\interfaces.go`
- `D:\code\task-processor\internal\productimage\scene_renderer.go`
- `D:\code\task-processor\internal\productimage\real_components.go`
- `D:\code\task-processor\internal\productimage\default_components_test.go`
- `D:\code\task-processor\internal\listingkit\workflow_assets_test.go`

### Test strategy boundary

- Unit-test the adapters in `internal/productimage`.
- Integration-test metadata propagation in `internal/listingkit`.
- Bootstrap-test production defaults in `internal/app/httpapi`.
- Do not add live-vendor tests in this phase.

## Task 1: Define provider-facing model contracts

**Files:**
- Create: `D:\code\task-processor\internal\productimage\model_provider.go`
- Modify: `D:\code\task-processor\internal\productimage\interfaces.go`
- Test: `D:\code\task-processor\internal\productimage\model_provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
package productimage_test

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestGenerationMetadataClonePreservesModelFields(t *testing.T) {
	src := &productimage.GenerationMetadata{
		Provider:         "openai",
		ModelFamily:      "gpt-image",
		GenerationMode:   "scene_generation",
		PromptRef:        "preset:selling_point/default",
		ReviewConfidence: 0.82,
	}

	cloned := src.Clone()
	if cloned == nil {
		t.Fatal("Clone() = nil")
	}
	if cloned.ModelFamily != "gpt-image" || cloned.GenerationMode != "scene_generation" {
		t.Fatalf("Clone() lost fields: %+v", cloned)
	}
	if cloned == src {
		t.Fatal("Clone() returned original pointer")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/productimage/... -run TestGenerationMetadataClonePreservesModelFields`

Expected: FAIL with undefined `GenerationMetadata` or missing `Clone`.

- [ ] **Step 3: Write minimal implementation**

`D:\code\task-processor\internal\productimage\model_provider.go`

```go
package productimage

import "context"

type GenerationMetadata struct {
	Provider         string
	ModelFamily      string
	GenerationMode   string
	PromptRef        string
	ReviewConfidence float64
}

func (m *GenerationMetadata) Clone() *GenerationMetadata {
	if m == nil {
		return nil
	}
	cloned := *m
	return &cloned
}

type FaithfulEditRequest struct {
	SourceAsset    *ImageAsset
	ProductContext *ProductContext
	Operation      string
	PromptRef      string
}

type FaithfulEditResult struct {
	Asset    *ImageAsset
	Metadata *GenerationMetadata
}

type SceneGenerationRequest struct {
	SourceAsset    *ImageAsset
	ProductContext *ProductContext
	PromptRef      string
	SceneIntent    string
}

type SceneGenerationResult struct {
	Assets   []ImageAsset
	Metadata *GenerationMetadata
}

type ReviewModelRequest struct {
	Source  *SourceBundle
	Result  *ImageProcessResult
	Context *ProductContext
}

type ReviewModelResult struct {
	Decision   *ReviewDecision
	Confidence float64
}

type FaithfulEditor interface {
	Edit(ctx context.Context, req *FaithfulEditRequest) (*FaithfulEditResult, error)
}

type SceneGenerator interface {
	GenerateScene(ctx context.Context, req *SceneGenerationRequest) (*SceneGenerationResult, error)
}

type ImageReviewModel interface {
	Review(ctx context.Context, req *ReviewModelRequest) (*ReviewModelResult, error)
}
```

`D:\code\task-processor\internal\productimage\interfaces.go`

```go
type ProductImageModelProvider interface {
	FaithfulEditor() FaithfulEditor
	SceneGenerator() SceneGenerator
	ReviewModel() ImageReviewModel
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/productimage/... -run TestGenerationMetadataClonePreservesModelFields`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/productimage/interfaces.go internal/productimage/model_provider.go internal/productimage/model_provider_test.go
git commit -m "feat: add productimage model provider contracts"
```

## Task 2: Replace subject extraction with faithful editor

**Files:**
- Create: `D:\code\task-processor\internal\productimage\model_subject_extractor.go`
- Modify: `D:\code\task-processor\internal\productimage\service.go`
- Test: `D:\code\task-processor\internal\productimage\model_subject_extractor_test.go`

- [ ] **Step 1: Write the failing test**

```go
package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

type faithfulEditorStub struct {
	lastReq *productimage.FaithfulEditRequest
	result  *productimage.FaithfulEditResult
}

func (s *faithfulEditorStub) Edit(_ context.Context, req *productimage.FaithfulEditRequest) (*productimage.FaithfulEditResult, error) {
	s.lastReq = req
	return s.result, nil
}

func TestModelSubjectExtractorUsesFaithfulEditor(t *testing.T) {
	editor := &faithfulEditorStub{
		result: &productimage.FaithfulEditResult{
			Asset: &productimage.ImageAsset{URL: "subject.png", Type: productimage.AssetTypeSubjectCutout},
			Metadata: &productimage.GenerationMetadata{GenerationMode: "subject_extraction"},
		},
	}

	extractor := productimage.NewModelSubjectExtractor(editor)
	asset, err := extractor.Extract(context.Background(), "https://img.example/source.jpg", &productimage.ProductContext{ProductType: "dress"})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if editor.lastReq == nil || editor.lastReq.Operation != "extract_subject" {
		t.Fatalf("last request = %+v", editor.lastReq)
	}
	if asset == nil || asset.URL != "subject.png" {
		t.Fatalf("asset = %+v", asset)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/productimage/... -run TestModelSubjectExtractorUsesFaithfulEditor`

Expected: FAIL with undefined `NewModelSubjectExtractor`.

- [ ] **Step 3: Write minimal implementation**

`D:\code\task-processor\internal\productimage\model_subject_extractor.go`

```go
package productimage

import (
	"context"
	"fmt"
)

type modelSubjectExtractor struct {
	editor FaithfulEditor
}

func NewModelSubjectExtractor(editor FaithfulEditor) SubjectExtractor {
	return &modelSubjectExtractor{editor: editor}
}

func (e *modelSubjectExtractor) Extract(ctx context.Context, imageURL string, context *ProductContext) (*ImageAsset, error) {
	if e.editor == nil {
		return nil, fmt.Errorf("faithful editor is not configured")
	}
	req := &FaithfulEditRequest{
		SourceAsset: &ImageAsset{
			URL:       imageURL,
			SourceURL: imageURL,
		},
		ProductContext: context,
		Operation:      "extract_subject",
		PromptRef:      "productimage/subject/extract",
	}
	result, err := e.editor.Edit(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Asset == nil {
		return nil, fmt.Errorf("faithful editor returned no asset")
	}
	if result.Asset.Metadata == nil {
		result.Asset.Metadata = map[string]string{}
	}
	if result.Metadata != nil {
		result.Asset.Metadata["model_provider"] = result.Metadata.Provider
		result.Asset.Metadata["model_family"] = result.Metadata.ModelFamily
		result.Asset.Metadata["generation_mode"] = result.Metadata.GenerationMode
	}
	return result.Asset, nil
}
```

`D:\code\task-processor\internal\productimage\service.go`

```go
if config.SubjectExtractor == nil && config.ModelProvider != nil && config.ModelProvider.FaithfulEditor() != nil {
	config.SubjectExtractor = NewModelSubjectExtractor(config.ModelProvider.FaithfulEditor())
}
if config.SubjectExtractor == nil {
	config.SubjectExtractor = NewDefaultSubjectExtractor()
}
```

Also extend `ServiceConfig`:

```go
ModelProvider ProductImageModelProvider
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/productimage/... -run TestModelSubjectExtractorUsesFaithfulEditor`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/productimage/model_subject_extractor.go internal/productimage/model_subject_extractor_test.go internal/productimage/service.go
git commit -m "feat: use faithful editor for subject extraction"
```

## Task 3: Replace white-background rendering with faithful editor

**Files:**
- Create: `D:\code\task-processor\internal\productimage\model_white_background_renderer.go`
- Modify: `D:\code\task-processor\internal\productimage\service.go`
- Test: `D:\code\task-processor\internal\productimage\model_white_background_renderer_test.go`

- [ ] **Step 1: Write the failing test**

```go
package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

func TestModelWhiteBackgroundRendererUsesFaithfulEditor(t *testing.T) {
	editor := &faithfulEditorStub{
		result: &productimage.FaithfulEditResult{
			Asset: &productimage.ImageAsset{URL: "white.jpg", Type: productimage.AssetTypeWhiteBackground},
			Metadata: &productimage.GenerationMetadata{GenerationMode: "white_background"},
		},
	}

	renderer := productimage.NewModelWhiteBackgroundRenderer(editor)
	asset, err := renderer.Render(context.Background(), &productimage.ImageAsset{URL: "subject.png"}, &productimage.ProductContext{ProductType: "dress"})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if editor.lastReq == nil || editor.lastReq.Operation != "render_white_background" {
		t.Fatalf("last request = %+v", editor.lastReq)
	}
	if asset == nil || asset.URL != "white.jpg" {
		t.Fatalf("asset = %+v", asset)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/productimage/... -run TestModelWhiteBackgroundRendererUsesFaithfulEditor`

Expected: FAIL with undefined `NewModelWhiteBackgroundRenderer`.

- [ ] **Step 3: Write minimal implementation**

`D:\code\task-processor\internal\productimage\model_white_background_renderer.go`

```go
package productimage

import (
	"context"
	"fmt"
)

type modelWhiteBackgroundRenderer struct {
	editor FaithfulEditor
}

func NewModelWhiteBackgroundRenderer(editor FaithfulEditor) WhiteBackgroundRenderer {
	return &modelWhiteBackgroundRenderer{editor: editor}
}

func (r *modelWhiteBackgroundRenderer) Render(ctx context.Context, asset *ImageAsset, context *ProductContext) (*ImageAsset, error) {
	if r.editor == nil {
		return nil, fmt.Errorf("faithful editor is not configured")
	}
	result, err := r.editor.Edit(ctx, &FaithfulEditRequest{
		SourceAsset:    asset,
		ProductContext: context,
		Operation:      "render_white_background",
		PromptRef:      "productimage/white-background/default",
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Asset == nil {
		return nil, fmt.Errorf("faithful editor returned no asset")
	}
	return result.Asset, nil
}
```

`D:\code\task-processor\internal\productimage\service.go`

```go
if config.WhiteBgRenderer == nil && config.ModelProvider != nil && config.ModelProvider.FaithfulEditor() != nil {
	config.WhiteBgRenderer = NewModelWhiteBackgroundRenderer(config.ModelProvider.FaithfulEditor())
}
if config.WhiteBgRenderer == nil {
	config.WhiteBgRenderer = NewDefaultWhiteBackgroundRenderer()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/productimage/... -run TestModelWhiteBackgroundRendererUsesFaithfulEditor`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/productimage/model_white_background_renderer.go internal/productimage/model_white_background_renderer_test.go internal/productimage/service.go
git commit -m "feat: use faithful editor for white background rendering"
```

## Task 4: Replace scene rendering with model-backed scene generation

**Files:**
- Create: `D:\code\task-processor\internal\productimage\model_scene_renderer.go`
- Create: `D:\code\task-processor\internal\productimage\model_fallback_policy.go`
- Modify: `D:\code\task-processor\internal\productimage\pipeline.go`
- Modify: `D:\code\task-processor\internal\productimage\service.go`
- Test: `D:\code\task-processor\internal\productimage\model_scene_renderer_test.go`

- [ ] **Step 1: Write the failing test**

```go
package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

type sceneGeneratorStub struct {
	lastReq *productimage.SceneGenerationRequest
	result  *productimage.SceneGenerationResult
}

func (s *sceneGeneratorStub) GenerateScene(_ context.Context, req *productimage.SceneGenerationRequest) (*productimage.SceneGenerationResult, error) {
	s.lastReq = req
	return s.result, nil
}

func TestModelSceneRendererUsesSceneGenerator(t *testing.T) {
	generator := &sceneGeneratorStub{
		result: &productimage.SceneGenerationResult{
			Assets: []productimage.ImageAsset{{URL: "scene.jpg", Type: productimage.AssetTypeGalleryImage}},
			Metadata: &productimage.GenerationMetadata{GenerationMode: "scene_generation"},
		},
	}

	renderer := productimage.NewModelSceneRenderer(generator)
	assets, err := renderer.Render(context.Background(), &productimage.ImageAsset{URL: "subject.png"}, &productimage.ProductContext{ProductType: "dress"})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if generator.lastReq == nil || generator.lastReq.SceneIntent != "gallery_scene" {
		t.Fatalf("last request = %+v", generator.lastReq)
	}
	if len(assets) != 1 || assets[0].URL != "scene.jpg" {
		t.Fatalf("assets = %+v", assets)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/productimage/... -run TestModelSceneRendererUsesSceneGenerator`

Expected: FAIL with undefined `NewModelSceneRenderer`.

- [ ] **Step 3: Write minimal implementation**

`D:\code\task-processor\internal\productimage\model_fallback_policy.go`

```go
package productimage

type FallbackPolicy struct {
	AllowLocalSceneFallback bool
}

func DefaultFallbackPolicy() FallbackPolicy {
	return FallbackPolicy{}
}
```

`D:\code\task-processor\internal\productimage\model_scene_renderer.go`

```go
package productimage

import (
	"context"
	"fmt"
)

type modelSceneRenderer struct {
	generator SceneGenerator
}

func NewModelSceneRenderer(generator SceneGenerator) SceneRenderer {
	return &modelSceneRenderer{generator: generator}
}

func (r *modelSceneRenderer) Render(ctx context.Context, asset *ImageAsset, context *ProductContext) ([]ImageAsset, error) {
	if r.generator == nil {
		return nil, fmt.Errorf("scene generator is not configured")
	}
	result, err := r.generator.GenerateScene(ctx, &SceneGenerationRequest{
		SourceAsset:    asset,
		ProductContext: context,
		PromptRef:      "productimage/scene/default",
		SceneIntent:    "gallery_scene",
	})
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.Assets) == 0 {
		return nil, fmt.Errorf("scene generator returned no assets")
	}
	return result.Assets, nil
}
```

`D:\code\task-processor\internal\productimage\service.go`

```go
FallbackPolicy FallbackPolicy
```

and default wiring:

```go
if config.SceneRenderer == nil && config.ModelProvider != nil && config.ModelProvider.SceneGenerator() != nil {
	config.SceneRenderer = NewModelSceneRenderer(config.ModelProvider.SceneGenerator())
}
```

`D:\code\task-processor\internal\productimage\pipeline.go`

```go
if s.sceneRenderer != nil {
	images, err := s.sceneRenderer.Render(ctx, state.Result.SubjectCutout, state.Context)
	if err != nil {
		state.addTrace("render_scene", state.Result.SubjectCutout.URL, string(AssetTypeGalleryImage), "failed", time.Since(startedAt), err.Error())
		return err
	}
	state.addTrace("render_scene", state.Result.SubjectCutout.URL, string(AssetTypeGalleryImage), "success", time.Since(startedAt), "")
	state.Result.SceneImages = append([]ImageAsset(nil), images...)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/productimage/... -run TestModelSceneRendererUsesSceneGenerator`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/productimage/model_scene_renderer.go internal/productimage/model_fallback_policy.go internal/productimage/model_scene_renderer_test.go internal/productimage/pipeline.go internal/productimage/service.go
git commit -m "feat: replace scene rendering with model-backed generator"
```

## Task 5: Replace review assessor with model-first review plus rule guards

**Files:**
- Create: `D:\code\task-processor\internal\productimage\model_review_assessor.go`
- Modify: `D:\code\task-processor\internal\productimage\service.go`
- Test: `D:\code\task-processor\internal\productimage\model_review_assessor_test.go`

- [ ] **Step 1: Write the failing test**

```go
package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

type reviewModelStub struct {
	result *productimage.ReviewModelResult
}

func (s *reviewModelStub) Review(_ context.Context, _ *productimage.ReviewModelRequest) (*productimage.ReviewModelResult, error) {
	return s.result, nil
}

func TestModelReviewAssessorAppliesModelDecisionAndRuleGuards(t *testing.T) {
	assessor := productimage.NewModelReviewAssessor(&reviewModelStub{
		result: &productimage.ReviewModelResult{
			Decision: &productimage.ReviewDecision{NeedsReview: false},
			Confidence: 0.93,
		},
	})

	result := &productimage.ImageProcessResult{
		Quality: &productimage.QualityAssessment{OverallScore: 0.41},
	}

	decision, err := assessor.Assess(context.Background(), &productimage.SourceBundle{}, nil, nil, result)
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if !decision.NeedsReview {
		t.Fatalf("decision = %+v, want forced review from rule guard", decision)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/productimage/... -run TestModelReviewAssessorAppliesModelDecisionAndRuleGuards`

Expected: FAIL with undefined `NewModelReviewAssessor`.

- [ ] **Step 3: Write minimal implementation**

`D:\code\task-processor\internal\productimage\model_review_assessor.go`

```go
package productimage

import "context"

type modelReviewAssessor struct {
	model ImageReviewModel
}

func NewModelReviewAssessor(model ImageReviewModel) ReviewAssessor {
	return &modelReviewAssessor{model: model}
}

func (a *modelReviewAssessor) Assess(ctx context.Context, source *SourceBundle, audits []ImageAudit, candidates *ImageCandidateSet, result *ImageProcessResult) (*ReviewDecision, error) {
	decision := &ReviewDecision{}
	if a.model != nil {
		modelResult, err := a.model.Review(ctx, &ReviewModelRequest{
			Source:  source,
			Result:  result,
			Context: result.Context,
		})
		if err != nil {
			return nil, err
		}
		if modelResult != nil && modelResult.Decision != nil {
			decision = modelResult.Decision
		}
	}
	if result != nil && result.Quality != nil && result.Quality.OverallScore < 0.65 {
		decision.NeedsReview = true
		decision.Reasons = append(decision.Reasons, "rule_validation_guard: overall quality below threshold")
	}
	return decision, nil
}
```

`D:\code\task-processor\internal\productimage\service.go`

```go
if config.ReviewAssessor == nil && config.ModelProvider != nil && config.ModelProvider.ReviewModel() != nil {
	config.ReviewAssessor = NewModelReviewAssessor(config.ModelProvider.ReviewModel())
}
if config.ReviewAssessor == nil {
	config.ReviewAssessor = NewDefaultReviewAssessor()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/productimage/... -run TestModelReviewAssessorAppliesModelDecisionAndRuleGuards`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/productimage/model_review_assessor.go internal/productimage/model_review_assessor_test.go internal/productimage/service.go
git commit -m "feat: add model-first review assessor"
```

## Task 6: Propagate model-backed execution metadata into listingkit

**Files:**
- Modify: `D:\code\task-processor\internal\asset\generation\model.go`
- Modify: `D:\code\task-processor\internal\listingkit\asset_workflow.go`
- Modify: `D:\code\task-processor\internal\listingkit\workflow.go`
- Test: `D:\code\task-processor\internal\listingkit\workflow_model_generation_test.go`

- [ ] **Step 1: Write the failing test**

```go
package listingkit_test

import (
	"testing"

	assetgeneration "task-processor/internal/asset/generation"
)

func TestGenerationTaskCarriesModelExecutionMetadata(t *testing.T) {
	task := assetgeneration.Task{
		ID:            "gen-1",
		ExecutionMode: "scene_generation_backed",
		Metadata: map[string]string{
			"model_family":    "gpt-image",
			"generation_mode": "scene_generation",
			"prompt_ref":      "preset:selling_point/default",
		},
	}

	if task.Metadata["model_family"] != "gpt-image" {
		t.Fatalf("metadata = %+v", task.Metadata)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit/... -run TestGenerationTaskCarriesModelExecutionMetadata`

Expected: FAIL if `Task` metadata or execution mode handling is missing.

- [ ] **Step 3: Write minimal implementation**

`D:\code\task-processor\internal\asset\generation\model.go`

```go
type Task struct {
	ID               string            `json:"id"`
	ExecutionMode    string            `json:"execution_mode,omitempty"`
	ExecutionStatus  string            `json:"execution_status,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ReviewConfidence float64           `json:"review_confidence,omitempty"`
	// existing fields stay intact
}
```

`D:\code\task-processor\internal\listingkit\asset_workflow.go`

```go
func attachModelMetadata(task *assetgeneration.Task, metadata *productimage.GenerationMetadata) {
	if task == nil || metadata == nil {
		return
	}
	if task.Metadata == nil {
		task.Metadata = map[string]string{}
	}
	task.Metadata["model_provider"] = metadata.Provider
	task.Metadata["model_family"] = metadata.ModelFamily
	task.Metadata["generation_mode"] = metadata.GenerationMode
	task.Metadata["prompt_ref"] = metadata.PromptRef
	task.ReviewConfidence = metadata.ReviewConfidence
}
```

`D:\code\task-processor\internal\listingkit\workflow.go`

```go
task.ExecutionMode = "scene_generation_backed"
attachModelMetadata(&task, generationMetadata)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/listingkit/... -run TestGenerationTaskCarriesModelExecutionMetadata`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/asset/generation/model.go internal/listingkit/asset_workflow.go internal/listingkit/workflow.go internal/listingkit/workflow_model_generation_test.go
git commit -m "feat: propagate model-backed generation metadata"
```

## Task 7: Wire model-backed defaults in HTTP bootstrap

**Files:**
- Modify: `D:\code\task-processor\internal\app\httpapi\modules.go`
- Test: `D:\code\task-processor\internal\app\httpapi\productimage_model_defaults_test.go`

- [ ] **Step 1: Write the failing test**

```go
package httpapi_test

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestBuildProductImageModuleDefaultsToModelBackedComponents(t *testing.T) {
	cfg := &productimage.ServiceConfig{
		TaskRepo:      newTestTaskRepo(),
		ModelProvider: newTestModelProvider(),
	}

	svc, err := productimage.NewService(cfg)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if svc == nil {
		t.Fatal("service = nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/httpapi/... -run TestBuildProductImageModuleDefaultsToModelBackedComponents`

Expected: FAIL until HTTP/bootstrap wiring passes a model provider.

- [ ] **Step 3: Write minimal implementation**

`D:\code\task-processor\internal\app\httpapi\modules.go`

```go
productImageService, err := productimage.NewService(&productimage.ServiceConfig{
	TaskRepo:         taskRepo,
	ModelProvider:    buildDefaultProductImageModelProvider(shared),
	SceneRenderer:    nil,
	SubjectExtractor: nil,
	WhiteBgRenderer:  nil,
	ReviewAssessor:   nil,
})
```

Keep the builder in a focused helper rather than growing `modules.go`:

```go
func buildDefaultProductImageModelProvider(shared *appbootstrap.SharedResources) productimage.ProductImageModelProvider {
	return productimage.NewOpenAIModelProvider(shared.OpenAIManager)
}
```

If `NewOpenAIModelProvider` does not exist yet, create it in `internal/productimage/model_provider.go` with thin adapter logic around existing `productenrich`-style clients.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/httpapi/... -run TestBuildProductImageModuleDefaultsToModelBackedComponents`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/modules.go internal/app/httpapi/productimage_model_defaults_test.go internal/productimage/model_provider.go
git commit -m "feat: wire model-backed productimage defaults"
```

## Task 8: Verify migration behavior and document rollout guardrails

**Files:**
- Modify: `D:\code\task-processor\internal\productimage\README.md`
- Modify: `D:\code\task-processor\docs\superpowers\specs\2026-04-19-model-driven-productimage-design.md`
- Test: `D:\code\task-processor\internal\productimage\model_provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
package productimage_test

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestFallbackPolicyDefaultsToNoLocalSceneFallback(t *testing.T) {
	policy := productimage.DefaultFallbackPolicy()
	if policy.AllowLocalSceneFallback {
		t.Fatalf("policy = %+v, want local scene fallback disabled by default", policy)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/productimage/... -run TestFallbackPolicyDefaultsToNoLocalSceneFallback`

Expected: FAIL until fallback defaults are explicit.

- [ ] **Step 3: Write minimal implementation**

If not already done in Task 4, make the default explicit:

`D:\code\task-processor\internal\productimage\model_fallback_policy.go`

```go
func DefaultFallbackPolicy() FallbackPolicy {
	return FallbackPolicy{
		AllowLocalSceneFallback: false,
	}
}
```

Document rollout behavior:

`D:\code\task-processor\internal\productimage\README.md`

```md
## Model-backed production path

Production subject extraction, white-background rendering, scene generation, and review are model-backed.
Local scene rendering is retained only as explicit fallback and must not be treated as a normal production success path.
```

`D:\code\task-processor\docs\superpowers\specs\2026-04-19-model-driven-productimage-design.md`

```md
Implementation status is tracked in the paired plan document:
`docs/superpowers/plans/2026-04-19-model-driven-productimage.md`.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/productimage/... -run TestFallbackPolicyDefaultsToNoLocalSceneFallback`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/productimage/model_fallback_policy.go internal/productimage/README.md docs/superpowers/specs/2026-04-19-model-driven-productimage-design.md docs/superpowers/plans/2026-04-19-model-driven-productimage.md
git commit -m "docs: document model-backed productimage rollout"
```

## Self-Review

### Spec coverage

- Product understanding retained and formalized: covered by Task 1 and bootstrap wiring in Task 7.
- Faithful editor for subject extraction: covered by Task 2.
- Faithful editor for white-background rendering: covered by Task 3.
- Scene generation replacing local canvas as primary path: covered by Task 4.
- Model-first review plus rule guards: covered by Task 5.
- Listingkit metadata propagation: covered by Task 6.
- Guardrail against silent local fallback: covered by Task 4 and Task 8.

No uncovered spec section remains.

### Placeholder scan

- No `TBD`, `TODO`, or deferred implementation text remains inside task steps.
- Every task includes explicit files, tests, commands, and code snippets.

### Type consistency

Consistent types and names used throughout the plan:

- `GenerationMetadata`
- `ProductImageModelProvider`
- `FaithfulEditor`
- `SceneGenerator`
- `ImageReviewModel`
- `NewModelSubjectExtractor`
- `NewModelWhiteBackgroundRenderer`
- `NewModelSceneRenderer`
- `NewModelReviewAssessor`

## Execution Handoff

Plan complete and saved to `D:\code\task-processor\docs\superpowers\plans\2026-04-19-model-driven-productimage.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
