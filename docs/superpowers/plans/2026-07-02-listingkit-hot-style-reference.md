# ListingKit Hot Style Reference Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a hot-selling reference-image mode that extracts safe reusable style direction, feeds it into existing POD artwork generation, and leaves the SHEIN listing pipeline unchanged.

**Architecture:** Add a small backend analysis service beside existing Studio media generation, expose it through the existing ListingKit Studio API, then persist and use the extracted brief in the current SHEIN/POD Studio UI. The new layer prepares prompt/reference inputs only; existing design generation, artwork review, product-image generation, and SHEIN task creation continue through current paths.

**Tech Stack:** Go, Gin, GORM-backed ListingKit service patterns, existing `AIChatCompleter.AnalyzeImage`, Next.js/React, TypeScript, Vitest, Go tests.

## Global Constraints

- Do not change SHEIN assembler or submission payloads.
- Do not add a new image storage backend.
- Do not automate competitor-product scraping.
- Do not bypass the existing artwork review step.
- Do not promise exact same style, exact same print, or exact same sales performance.
- Use 1 to 5 uploaded or already-selected reference image URLs.
- The feature must produce abstract, sanitized design direction and must avoid logos, brand marks, exact text, characters, faces, and unique layouts from reference images.

---

## File Structure

**Backend model and service**

- Modify `internal/listingkit/model_request_studio_support.go`
  - Add `StudioReferenceAnalysisRequest` and `StudioReferenceAnalysisResponse`.
- Modify `internal/listingkit/interfaces_services.go`
  - Add `AnalyzeStudioReferenceStyle` to `StudioMediaService`.
- Create `internal/listingkit/studio_reference_analysis.go`
  - Own request validation, per-image analysis, brief merging, prompt sanitization, and warning generation.
- Create `internal/listingkit/studio_reference_analysis_test.go`
  - Cover validation, image limit, sanitizer, malformed analyzer response fallback, and partial failures.

**Backend API**

- Create `internal/listingkit/api/studio_reference_analysis_handler.go`
  - Bind request, call `studioMediaService.AnalyzeStudioReferenceStyle`, return JSON.
- Create `internal/listingkit/api/studio_reference_analysis_handler_test.go`
  - Cover success, empty image URL list, service unavailable, and analyzer error.
- Modify `internal/listingkit/api/studio_media_handler_test_stubs_test.go`
  - Add the new method to handler test stubs.
- Modify `internal/listingkit/api/handler_core_service_test_stubs_test.go`
  - Add the new method to core service stubs.
- Modify `internal/listingkit/httpapi/routes_task.go`
  - Add `AnalyzeStudioReferenceStyle(c *gin.Context)` to route interfaces.
- Modify `internal/listingkit/httpapi/routes_descriptor_task.go`
  - Register `POST /api/v1/listing-kits/studio/reference-style/analyze`.
- Modify `internal/listingkit/httpapi/http_module_test.go`
  - Add the method to `stubRouteHandler`.
- Modify `internal/listingkit/httpapi/routes_interface_test.go`
  - Add the method to route test stubs.

**Frontend API, types, and persistence**

- Modify `web/listingkit-ui/src/lib/types/shein-studio-generation.ts`
  - Add request/response types for reference analysis.
- Modify `web/listingkit-ui/src/lib/types/shein-studio-draft.ts`
  - Persist reference image URLs, brief, and prompt in Studio drafts.
- Modify `web/listingkit-ui/src/lib/types/shein-studio-batch.ts`
  - Persist the same fields on saved batches.
- Modify `web/listingkit-ui/src/lib/api/shein-studio.ts`
  - Add `analyzeSheinStudioReferenceStyle`.
- Modify `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts`
  - Serialize and hydrate the new draft fields.
- Modify `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
  - Normalize the new batch fields from backend payloads.
- Modify `web/listingkit-ui/src/lib/shein-studio/storage-shared.ts`
  - Persist the new fields in local draft/session storage.
- Modify matching tests under `web/listingkit-ui/src/lib/api` and `web/listingkit-ui/src/lib/shein-studio`.

**Frontend UI and generation wiring**

- Modify `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
  - Add state fields and setters.
- Modify `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
  - Pass state and handlers into the generation panel.
- Modify `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.tsx`
  - Pass reference props into the form section.
- Modify `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-form-sections.tsx`
  - Add compact "热销款参考" controls inside the existing POD artwork generation area.
- Modify `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
  - Merge `hotStyleReferencePrompt` into the design-generation prompt and include `hotStyleReferenceImageUrls` in `productReferenceImageUrls`.
- Modify `web/listingkit-ui/src/lib/shein-studio/generation-controller.ts`
  - Ensure generated requests carry the reference prompt and URLs for batch execution.
- Update related component/controller tests.

---

### Task 1: Backend Reference Analysis Service

**Files:**
- Modify: `internal/listingkit/model_request_studio_support.go`
- Modify: `internal/listingkit/interfaces_services.go`
- Create: `internal/listingkit/studio_reference_analysis.go`
- Create: `internal/listingkit/studio_reference_analysis_test.go`

**Interfaces:**
- Consumes: `AIChatCompleter.AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error)` from `internal/listingkit/ai_contracts.go`.
- Produces: `AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error)` on `StudioMediaService`.

- [ ] **Step 1: Write failing model/service tests**

Add `internal/listingkit/studio_reference_analysis_test.go`:

```go
package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type stubReferenceAnalysisCompleter struct {
	responses []string
	errAt     int
	calls     []string
}

func (s *stubReferenceAnalysisCompleter) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, errors.New("not used")
}

func (s *stubReferenceAnalysisCompleter) Generate(context.Context, string) (string, error) {
	return "", errors.New("not used")
}

func (s *stubReferenceAnalysisCompleter) AnalyzeImage(_ context.Context, imageURL string, prompt string) (string, error) {
	s.calls = append(s.calls, imageURL+"|"+prompt)
	if s.errAt > 0 && len(s.calls) == s.errAt {
		return "", errors.New("vision failed")
	}
	idx := len(s.calls) - 1
	if idx < len(s.responses) {
		return s.responses[idx], nil
	}
	return `{"motif":"retro flowers","palette":["cream","red"],"composition":"large centered badge","avoid":["logos","exact text"]}`, nil
}

func (s *stubReferenceAnalysisCompleter) GetDefaultModel() string {
	return "vision-test"
}

func TestAnalyzeStudioReferenceStyleRejectsEmptyReferences(t *testing.T) {
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: &stubReferenceAnalysisCompleter{}})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{})
	if err == nil || !strings.Contains(err.Error(), "reference_image_urls is required") {
		t.Fatalf("error = %v, want reference_image_urls validation", err)
	}
}

func TestAnalyzeStudioReferenceStyleLimitsReferencesAndSanitizesPrompt(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"sports mascot","palette":["navy","cream"],"composition":"varsity badge","typography":"bold collegiate","avoid":["Nike logo","exact slogan"]}`,
		`{"motif":"floral border","palette":["red","cream"],"composition":"arched frame","typography":"distressed serif","avoid":["brand mark"]}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png", "https://example.com/b.png", "https://example.com/c.png", "https://example.com/d.png", "https://example.com/e.png", "https://example.com/f.png"},
		ProductName:        "T-shirt",
		CategoryPath:       []string{"Apparel", "Tops"},
		BasePrompt:         "summer",
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if len(completer.calls) != 5 {
		t.Fatalf("calls = %d, want 5", len(completer.calls))
	}
	if strings.Contains(strings.ToLower(resp.SanitizedPrompt), "nike") || strings.Contains(strings.ToLower(resp.SanitizedPrompt), "exact slogan") {
		t.Fatalf("sanitized prompt contains unsafe source material: %q", resp.SanitizedPrompt)
	}
	if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), "original") {
		t.Fatalf("sanitized prompt = %q, want originality instruction", resp.SanitizedPrompt)
	}
	if len(resp.Warnings) == 0 {
		t.Fatalf("warnings = nil, want warning for truncated reference list")
	}
}

func TestAnalyzeStudioReferenceStyleFallsBackForMalformedJSON(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{"retro cherry badge, cream background, no logos"}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if !strings.Contains(resp.ReferenceStyleBrief, "retro cherry badge") {
		t.Fatalf("brief = %q, want malformed text retained as brief", resp.ReferenceStyleBrief)
	}
}

func TestAnalyzeStudioReferenceStyleUsesPartialSuccess(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{
		responses: []string{`{"motif":"western floral","palette":["tan","red"],"composition":"center badge"}`},
		errAt:     2,
	}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png", "https://example.com/b.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Fatalf("warnings = nil, want partial failure warning")
	}
	if !strings.Contains(resp.SanitizedPrompt, "western floral") {
		t.Fatalf("sanitized prompt = %q, want successful image analysis used", resp.SanitizedPrompt)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/listingkit -run "TestAnalyzeStudioReferenceStyle" -count=1
```

Expected: FAIL because `StudioReferenceAnalysisRequest`, `StudioReferenceAnalysisResponse`, and `AnalyzeStudioReferenceStyle` do not exist.

- [ ] **Step 3: Add model and interface types**

In `internal/listingkit/model_request_studio_support.go`, add after `StudioDesignResponse`:

```go
type StudioReferenceAnalysisRequest struct {
	ReferenceImageURLs []string `json:"reference_image_urls,omitempty"`
	ProductName        string   `json:"product_name,omitempty"`
	CategoryPath       []string `json:"category_path,omitempty"`
	BasePrompt         string   `json:"base_prompt,omitempty"`
	UserInstruction    string   `json:"user_instruction,omitempty"`
}

type StudioReferenceAnalysisResponse struct {
	ReferenceStyleBrief string   `json:"reference_style_brief,omitempty"`
	SanitizedPrompt     string   `json:"sanitized_prompt,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}
```

In `internal/listingkit/interfaces_services.go`, update `StudioMediaService`:

```go
type StudioMediaService interface {
	UploadImages(ctx context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error)
	GetUploadedImage(ctx context.Context, key string) (*UploadedImageFile, error)
	AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error)
	GenerateStudioDesigns(ctx context.Context, req *StudioDesignRequest) (*StudioDesignResponse, error)
	GenerateStudioProductImages(ctx context.Context, req *StudioProductImageRequest) (*StudioProductImageResponse, error)
	RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error)
}
```

- [ ] **Step 4: Implement service and sanitizer**

Create `internal/listingkit/studio_reference_analysis.go`:

```go
package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const maxStudioReferenceAnalysisImages = 5

type studioReferenceImageAnalysis struct {
	Motif       string   `json:"motif,omitempty"`
	Palette     []string `json:"palette,omitempty"`
	Composition string   `json:"composition,omitempty"`
	Typography  string   `json:"typography,omitempty"`
	Density     string   `json:"density,omitempty"`
	ProductFit  string   `json:"product_fit,omitempty"`
	Avoid       []string `json:"avoid,omitempty"`
	Raw         string   `json:"-"`
}

func (s *taskStudioMediaService) AnalyzeStudioReferenceStyle(ctx context.Context, req *StudioReferenceAnalysisRequest) (*StudioReferenceAnalysisResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	urls := normalizeStudioReferenceImageURLs(req.ReferenceImageURLs)
	if len(urls) == 0 {
		return nil, fmt.Errorf("invalid request: reference_image_urls is required")
	}
	if s == nil || s.promptDiversifier == nil {
		return nil, fmt.Errorf("reference_analysis_unavailable: studio reference analyzer is not configured")
	}

	warnings := make([]string, 0)
	if len(urls) > maxStudioReferenceAnalysisImages {
		urls = urls[:maxStudioReferenceAnalysisImages]
		warnings = append(warnings, "最多分析 5 张参考图，已忽略多余图片。")
	}

	analyses := make([]studioReferenceImageAnalysis, 0, len(urls))
	for _, imageURL := range urls {
		raw, err := s.promptDiversifier.AnalyzeImage(ctx, imageURL, buildStudioReferenceAnalysisPrompt(req))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("参考图分析失败：%s", compactStudioGenerationError(err)))
			continue
		}
		analysis := parseStudioReferenceImageAnalysis(raw)
		analyses = append(analyses, analysis)
	}
	if len(analyses) == 0 {
		return nil, fmt.Errorf("reference_analysis_failed: no reference image could be analyzed")
	}

	brief := buildStudioReferenceStyleBrief(req, analyses)
	sanitized := sanitizeStudioReferencePrompt(brief)
	if strings.TrimSpace(sanitized) == "" {
		return nil, fmt.Errorf("reference_analysis_failed: generated reference brief is empty")
	}
	if sanitized != brief {
		warnings = append(warnings, "已移除品牌、Logo、原文案或过于接近原图的描述。")
	}
	return &StudioReferenceAnalysisResponse{
		ReferenceStyleBrief: brief,
		SanitizedPrompt:     sanitized,
		Warnings:            warnings,
	}, nil
}

func normalizeStudioReferenceImageURLs(urls []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(urls))
	for _, raw := range urls {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func buildStudioReferenceAnalysisPrompt(req *StudioReferenceAnalysisRequest) string {
	product := strings.TrimSpace(req.ProductName)
	category := strings.Join(req.CategoryPath, " > ")
	basePrompt := strings.TrimSpace(req.BasePrompt)
	userInstruction := strings.TrimSpace(req.UserInstruction)
	return strings.TrimSpace(fmt.Sprintf(`Analyze this ecommerce product image as a style reference for original POD artwork.
Return JSON only with keys: motif, palette, composition, typography, density, product_fit, avoid.
Extract broad reusable commercial style only. Do not ask to copy logos, brand marks, exact slogans, exact characters, faces, or the same unique layout.
Product: %s
Category: %s
Existing user theme: %s
User instruction: %s`, product, category, basePrompt, userInstruction))
}

func parseStudioReferenceImageAnalysis(raw string) studioReferenceImageAnalysis {
	cleaned := strings.TrimSpace(raw)
	var analysis studioReferenceImageAnalysis
	if err := json.Unmarshal([]byte(cleaned), &analysis); err != nil {
		return studioReferenceImageAnalysis{Raw: cleaned}
	}
	analysis.Raw = cleaned
	return analysis
}

func buildStudioReferenceStyleBrief(req *StudioReferenceAnalysisRequest, analyses []studioReferenceImageAnalysis) string {
	parts := []string{"Hot-selling reference style direction for original POD artwork."}
	if product := strings.TrimSpace(req.ProductName); product != "" {
		parts = append(parts, "Base product: "+product+".")
	}
	if category := strings.TrimSpace(strings.Join(req.CategoryPath, " > ")); category != "" {
		parts = append(parts, "Category: "+category+".")
	}
	for _, item := range analyses {
		if item.Raw != "" && item.Motif == "" && item.Composition == "" {
			parts = append(parts, "Reference notes: "+item.Raw+".")
			continue
		}
		if item.Motif != "" {
			parts = append(parts, "Motif family: "+item.Motif+".")
		}
		if len(item.Palette) > 0 {
			parts = append(parts, "Palette direction: "+strings.Join(item.Palette, ", ")+".")
		}
		if item.Composition != "" {
			parts = append(parts, "Composition family: "+item.Composition+".")
		}
		if item.Typography != "" {
			parts = append(parts, "Typography feel: "+item.Typography+".")
		}
		if item.Density != "" {
			parts = append(parts, "Visual density: "+item.Density+".")
		}
		if item.ProductFit != "" {
			parts = append(parts, "Product fit: "+item.ProductFit+".")
		}
		if len(item.Avoid) > 0 {
			parts = append(parts, "Avoid from references: "+strings.Join(item.Avoid, ", ")+".")
		}
	}
	parts = append(parts, "Create a new original design in this broad style family. Do not reproduce logos, brand marks, exact text, characters, faces, or the same unique layout from any reference image.")
	return strings.Join(parts, " ")
}

func sanitizeStudioReferencePrompt(value string) string {
	replacements := []string{
		"Nike", "", "nike", "", "logo", "brand-neutral mark", "Logo", "brand-neutral mark",
		"exact slogan", "new short generic phrase", "same wording", "new wording",
		"copy this exact", "reinterpret as original", "same character", "original motif",
	}
	sanitized := strings.NewReplacer(replacements...).Replace(value)
	sanitized = strings.Join(strings.Fields(sanitized), " ")
	return strings.TrimSpace(sanitized)
}
```

- [ ] **Step 5: Run focused backend service tests**

Run:

```powershell
go test ./internal/listingkit -run "TestAnalyzeStudioReferenceStyle" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit backend service slice**

Run:

```powershell
git add internal/listingkit/model_request_studio_support.go internal/listingkit/interfaces_services.go internal/listingkit/studio_reference_analysis.go internal/listingkit/studio_reference_analysis_test.go
git commit -m "feat: analyze studio hot style references"
```

---

### Task 2: Backend API Route and Handler

**Files:**
- Create: `internal/listingkit/api/studio_reference_analysis_handler.go`
- Create: `internal/listingkit/api/studio_reference_analysis_handler_test.go`
- Modify: `internal/listingkit/api/studio_media_handler_test_stubs_test.go`
- Modify: `internal/listingkit/api/handler_core_service_test_stubs_test.go`
- Modify: `internal/listingkit/httpapi/routes_task.go`
- Modify: `internal/listingkit/httpapi/routes_descriptor_task.go`
- Modify: `internal/listingkit/httpapi/http_module_test.go`
- Modify: `internal/listingkit/httpapi/routes_interface_test.go`

**Interfaces:**
- Consumes: `AnalyzeStudioReferenceStyle(ctx context.Context, req *listingkit.StudioReferenceAnalysisRequest) (*listingkit.StudioReferenceAnalysisResponse, error)`.
- Produces: `POST /api/v1/listing-kits/studio/reference-style/analyze`.

- [ ] **Step 1: Write failing handler tests**

Create `internal/listingkit/api/studio_reference_analysis_handler_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubStudioReferenceAnalysisService struct {
	req  *listingkit.StudioReferenceAnalysisRequest
	resp *listingkit.StudioReferenceAnalysisResponse
	err  error
}

func (s *stubStudioReferenceAnalysisService) UploadImages(context.Context, *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	return nil, errors.New("not used")
}
func (s *stubStudioReferenceAnalysisService) GetUploadedImage(context.Context, string) (*listingkit.UploadedImageFile, error) {
	return nil, errors.New("not used")
}
func (s *stubStudioReferenceAnalysisService) AnalyzeStudioReferenceStyle(_ context.Context, req *listingkit.StudioReferenceAnalysisRequest) (*listingkit.StudioReferenceAnalysisResponse, error) {
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}
func (s *stubStudioReferenceAnalysisService) GenerateStudioDesigns(context.Context, *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, errors.New("not used")
}
func (s *stubStudioReferenceAnalysisService) GenerateStudioProductImages(context.Context, *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	return nil, errors.New("not used")
}
func (s *stubStudioReferenceAnalysisService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not used")
}

func TestAnalyzeStudioReferenceStyleHandlerReturnsBrief(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubStudioReferenceAnalysisService{resp: &listingkit.StudioReferenceAnalysisResponse{
		ReferenceStyleBrief: "retro badge",
		SanitizedPrompt:     "original retro badge",
		Warnings:            []string{"safe"},
	}}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(service))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/reference-style/analyze", h.AnalyzeStudioReferenceStyle)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/reference-style/analyze", strings.NewReader(`{"reference_image_urls":["https://example.com/a.png"],"base_prompt":"summer"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if service.req == nil || len(service.req.ReferenceImageURLs) != 1 {
		t.Fatalf("service request = %+v, want reference image")
	}
	var body listingkit.StudioReferenceAnalysisResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.SanitizedPrompt != "original retro badge" {
		t.Fatalf("sanitized prompt = %q", body.SanitizedPrompt)
	}
}

func TestAnalyzeStudioReferenceStyleHandlerMapsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubStudioReferenceAnalysisService{err: errors.New("invalid request: reference_image_urls is required")}
	h, _ := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(service))
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/reference-style/analyze", h.AnalyzeStudioReferenceStyle)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/reference-style/analyze", strings.NewReader(`{"reference_image_urls":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/listingkit/api -run "TestAnalyzeStudioReferenceStyleHandler" -count=1
```

Expected: FAIL because `AnalyzeStudioReferenceStyle` handler does not exist.

- [ ] **Step 3: Implement handler**

Create `internal/listingkit/api/studio_reference_analysis_handler.go`:

```go
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) AnalyzeStudioReferenceStyle(c *gin.Context) {
	var req listingkit.StudioReferenceAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if h.studioMediaService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "reference_analysis_unavailable", "message": "studio media service is not configured"})
		return
	}
	response, err := h.studioMediaService.AnalyzeStudioReferenceStyle(requestContext(c), &req)
	if err != nil {
		status := http.StatusInternalServerError
		errorCode := "reference_analysis_failed"
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
			errorCode = "invalid_request"
		}
		if strings.Contains(err.Error(), "reference_analysis_unavailable") {
			status = http.StatusNotImplemented
			errorCode = "reference_analysis_unavailable"
		}
		c.JSON(status, gin.H{"error": errorCode, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}
```

- [ ] **Step 4: Add route interfaces and descriptor**

In `internal/listingkit/httpapi/routes_task.go`, add `AnalyzeStudioReferenceStyle(c *gin.Context)` to both route handler interfaces that already contain `GenerateStudioDesigns`.

In `internal/listingkit/httpapi/routes_descriptor_task.go`, insert the descriptor beside the Studio design routes:

```go
httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/studio/reference-style/analyze", Module: "listing-kit", Handler: handler.AnalyzeStudioReferenceStyle},
```

Update `stubRouteHandler`, `taskOnlyRouteHandler`, and `studioGenerationOnlyRouteHandler` test stubs with:

```go
func (stubRouteHandler) AnalyzeStudioReferenceStyle(*gin.Context) {}
```

Use the local receiver name required by each test stub.

- [ ] **Step 5: Update shared API test stubs**

In `internal/listingkit/api/studio_media_handler_test_stubs_test.go` and `internal/listingkit/api/handler_core_service_test_stubs_test.go`, add:

```go
func (s *stubStudioMediaHandlerService) AnalyzeStudioReferenceStyle(context.Context, *listingkit.StudioReferenceAnalysisRequest) (*listingkit.StudioReferenceAnalysisResponse, error) {
	return nil, nil
}
```

For value receiver stubs such as `stubHandlerCoreService`, use:

```go
func (stubHandlerCoreService) AnalyzeStudioReferenceStyle(context.Context, *listingkit.StudioReferenceAnalysisRequest) (*listingkit.StudioReferenceAnalysisResponse, error) {
	return nil, nil
}
```

- [ ] **Step 6: Run route and API tests**

Run:

```powershell
go test ./internal/listingkit/api ./internal/listingkit/httpapi -run "TestAnalyzeStudioReferenceStyleHandler|Route|Descriptor|Studio" -count=1
```

Expected: PASS for the new handler tests and existing route/interface tests.

- [ ] **Step 7: Commit API slice**

Run:

```powershell
git add internal/listingkit/api internal/listingkit/httpapi
git commit -m "feat: expose studio hot style reference analysis"
```

---

### Task 3: Frontend API Types and Persistence

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/shein-studio-generation.ts`
- Modify: `web/listingkit-ui/src/lib/types/shein-studio-draft.ts`
- Modify: `web/listingkit-ui/src/lib/types/shein-studio-batch.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batch-drafts.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/storage-shared.ts`
- Test: matching `.test.ts` files in those directories.

**Interfaces:**
- Consumes: `POST /studio/reference-style/analyze` through the existing `apiRequest`/`apiAsyncRequest` client style.
- Produces: `analyzeSheinStudioReferenceStyle(body: SheinStudioReferenceAnalysisRequest): Promise<SheinStudioReferenceAnalysisResponse>` and persisted draft fields.

- [ ] **Step 1: Write failing frontend API test**

In `web/listingkit-ui/src/lib/api/shein-studio.test.ts`, add a test near existing Studio API tests:

```ts
it("posts hot style reference analysis payload", async () => {
  const fetchMock = vi.fn().mockResolvedValue(
    new Response(
      JSON.stringify({
        reference_style_brief: "retro badge",
        sanitized_prompt: "original retro badge",
        warnings: ["safe"],
      }),
      { status: 200, headers: { "content-type": "application/json" } },
    ),
  );
  vi.stubGlobal("fetch", fetchMock);

  const result = await analyzeSheinStudioReferenceStyle({
    referenceImageUrls: ["https://example.com/a.png"],
    productName: "T-shirt",
    categoryPath: ["Apparel"],
    basePrompt: "summer",
    userInstruction: "women audience",
  });

  expect(fetchMock).toHaveBeenCalledWith(
    expect.stringContaining("/studio/reference-style/analyze"),
    expect.objectContaining({
      method: "POST",
      body: JSON.stringify({
        reference_image_urls: ["https://example.com/a.png"],
        product_name: "T-shirt",
        category_path: ["Apparel"],
        base_prompt: "summer",
        user_instruction: "women audience",
      }),
    }),
  );
  expect(result).toEqual({
    referenceStyleBrief: "retro badge",
    sanitizedPrompt: "original retro badge",
    warnings: ["safe"],
  });
});
```

Add the missing import:

```ts
import { analyzeSheinStudioReferenceStyle } from "@/lib/api/shein-studio";
```

- [ ] **Step 2: Run API test to verify it fails**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/lib/api/shein-studio.test.ts
```

Expected: FAIL because `analyzeSheinStudioReferenceStyle` is not exported.

- [ ] **Step 3: Add frontend types and API wrapper**

In `web/listingkit-ui/src/lib/types/shein-studio-generation.ts`, add:

```ts
export type SheinStudioReferenceAnalysisRequest = {
  referenceImageUrls: string[];
  productName?: string;
  categoryPath?: string[];
  basePrompt?: string;
  userInstruction?: string;
};

export type SheinStudioReferenceAnalysisResponse = {
  referenceStyleBrief: string;
  sanitizedPrompt: string;
  warnings: string[];
};
```

In `web/listingkit-ui/src/lib/api/shein-studio.ts`, import these types and add:

```ts
export async function analyzeSheinStudioReferenceStyle(
  body: SheinStudioReferenceAnalysisRequest,
) {
  const payload = await apiRequest<{
    reference_style_brief?: string;
    sanitized_prompt?: string;
    warnings?: string[];
  }>("/studio/reference-style/analyze", {
    method: "POST",
    body: {
      reference_image_urls: body.referenceImageUrls,
      product_name: body.productName,
      category_path: body.categoryPath,
      base_prompt: body.basePrompt,
      user_instruction: body.userInstruction,
    },
    timeoutMs: 120_000,
  });
  return {
    referenceStyleBrief: payload.reference_style_brief ?? "",
    sanitizedPrompt: payload.sanitized_prompt ?? "",
    warnings: payload.warnings ?? [],
  } satisfies SheinStudioReferenceAnalysisResponse;
}
```

- [ ] **Step 4: Add draft and batch fields**

Add these optional fields to the relevant draft, saved batch, and persisted state types:

```ts
hotStyleReferenceImageUrls?: string[];
hotStyleReferenceBrief?: string;
hotStyleReferencePrompt?: string;
```

In serializers that emit snake_case payloads, map to:

```ts
hot_style_reference_image_urls: input.hotStyleReferenceImageUrls,
hot_style_reference_brief: input.hotStyleReferenceBrief,
hot_style_reference_prompt: input.hotStyleReferencePrompt,
```

In hydrators/normalizers, map from snake_case or camelCase:

```ts
hotStyleReferenceImageUrls:
  raw.hot_style_reference_image_urls ?? raw.hotStyleReferenceImageUrls ?? [],
hotStyleReferenceBrief:
  raw.hot_style_reference_brief ?? raw.hotStyleReferenceBrief ?? "",
hotStyleReferencePrompt:
  raw.hot_style_reference_prompt ?? raw.hotStyleReferencePrompt ?? "",
```

- [ ] **Step 5: Add persistence tests**

In `web/listingkit-ui/src/lib/shein-studio/storage-shared.test.ts`, add a case that stores and restores:

```ts
hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
hotStyleReferenceBrief: "retro badge",
hotStyleReferencePrompt: "original retro badge",
```

Expected assertion:

```ts
expect(restored.hotStyleReferenceImageUrls).toEqual(["https://example.com/ref.png"]);
expect(restored.hotStyleReferenceBrief).toBe("retro badge");
expect(restored.hotStyleReferencePrompt).toBe("original retro badge");
```

- [ ] **Step 6: Run focused frontend type/API/persistence tests**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/lib/api/shein-studio.test.ts src/lib/shein-studio/storage-shared.test.ts src/lib/api/shein-studio-batch-drafts.test.ts src/lib/api/shein-studio-batches.test.ts
```

Expected: PASS.

- [ ] **Step 7: Commit frontend API/persistence slice**

Run:

```powershell
git add web/listingkit-ui/src/lib/types web/listingkit-ui/src/lib/api web/listingkit-ui/src/lib/shein-studio
git commit -m "feat: persist hot style reference inputs"
```

---

### Task 4: Studio UI Controls

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-state.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-form-sections.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

**Interfaces:**
- Consumes: `analyzeSheinStudioReferenceStyle` from Task 3.
- Produces: editable UI state for `hotStyleReferenceImageUrls`, `hotStyleReferenceBrief`, and `hotStyleReferencePrompt`.

- [ ] **Step 1: Write failing component test**

In `shein-studio-generation-panel.test.tsx`, add a test using the existing harness style:

```tsx
it("renders hot style reference controls and applies extracted prompt", async () => {
  const user = userEvent.setup();
  const analyzeReferenceStyle = vi.fn().mockResolvedValue({
    referenceStyleBrief: "retro badge with cream and red palette",
    sanitizedPrompt: "Create an original retro badge with cream and red palette.",
    warnings: [],
  });

  render(
    <SheinStudioGenerationPanel
      {...baseGenerationPanelProps}
      hotStyleReferenceImageUrls={["https://example.com/ref.png"]}
      hotStyleReferenceBrief=""
      hotStyleReferencePrompt=""
      setHotStyleReferenceImageUrls={vi.fn()}
      setHotStyleReferenceBrief={vi.fn()}
      setHotStyleReferencePrompt={baseGenerationPanelProps.setPrompt}
      analyzeReferenceStyle={analyzeReferenceStyle}
    />,
  );

  await user.click(screen.getByRole("button", { name: "提取热销款风格" }));

  expect(analyzeReferenceStyle).toHaveBeenCalledWith({
    referenceImageUrls: ["https://example.com/ref.png"],
    basePrompt: baseGenerationPanelProps.prompt,
  });
  expect(baseGenerationPanelProps.setPrompt).toHaveBeenCalledWith(
    expect.stringContaining("original retro badge"),
  );
});
```

Adjust prop names to match existing `baseGenerationPanelProps` in the file.

- [ ] **Step 2: Run component test to verify it fails**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx
```

Expected: FAIL because the new props and controls do not exist.

- [ ] **Step 3: Add workbench state fields**

In `shein-studio-workbench-state.ts`, add defaults:

```ts
hotStyleReferenceImageUrls: [],
hotStyleReferenceBrief: "",
hotStyleReferencePrompt: "",
```

Add setters in the returned state API:

```ts
setHotStyleReferenceImageUrls: (value) => setField("hotStyleReferenceImageUrls", value),
setHotStyleReferenceBrief: (value) => setField("hotStyleReferenceBrief", value),
setHotStyleReferencePrompt: (value) => setField("hotStyleReferencePrompt", value),
```

Include the fields in reset, hydration, and persistence snapshots wherever `prompt`, `selectedSdsImages`, and `productImagePrompt` are currently copied.

- [ ] **Step 4: Thread props through workbench and panel**

In `shein-studio-workbench.tsx`, import `analyzeSheinStudioReferenceStyle` and pass a handler:

```tsx
const analyzeReferenceStyle = useCallback(
  (input: { referenceImageUrls: string[]; basePrompt?: string }) =>
    analyzeSheinStudioReferenceStyle({
      referenceImageUrls: input.referenceImageUrls,
      basePrompt: input.basePrompt,
      productName: selectedGroup?.productName,
      categoryPath: selectedGroup?.categoryPath,
    }),
  [selectedGroup?.categoryPath, selectedGroup?.productName],
);
```

Pass state fields and setters into `SheinStudioGenerationPanel`, then from panel into `ArtworkGenerationSettings`.

- [ ] **Step 5: Add compact UI controls**

In `shein-studio-generation-form-sections.tsx`, inside `ArtworkGenerationSettings`, add a section below the theme prompt:

```tsx
<div className="space-y-3 rounded-lg border border-border bg-background px-3 py-3">
  <div className="flex flex-wrap items-center justify-between gap-2">
    <div>
      <p className="text-sm font-medium text-foreground">热销款参考</p>
      <p className="text-xs leading-5 text-muted-foreground">
        提取图案、配色和构图方向，生成相似风格的原创 POD 图案。
      </p>
    </div>
    <Button
      disabled={disabled || hotStyleReferenceImageUrls.length === 0 || isAnalyzingReferenceStyle}
      onClick={handleAnalyzeReferenceStyle}
      type="button"
    >
      提取热销款风格
    </Button>
  </div>
  <Textarea
    className="min-h-24 rounded-lg px-3 py-2"
    disabled={disabled}
    onChange={(event) => setHotStyleReferencePrompt(event.target.value)}
    placeholder="提取后可在这里微调风格要求。"
    value={hotStyleReferencePrompt}
  />
  {hotStyleReferenceBrief ? (
    <p className="text-xs leading-5 text-muted-foreground">{hotStyleReferenceBrief}</p>
  ) : null}
</div>
```

Use existing local `Button` and `Textarea` imports/patterns in the file. Keep card radius at `rounded-lg` or existing local style; do not introduce a new visual system.

- [ ] **Step 6: Run focused UI tests**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit UI controls slice**

Run:

```powershell
git add web/listingkit-ui/src/components/listingkit/shein-studio
git commit -m "feat: add hot style reference controls"
```

---

### Task 5: Wire Reference Prompt Into Generation

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-actions.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/generation-controller.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/draft-input.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/generation-controller.test.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/draft-input.test.ts`

**Interfaces:**
- Consumes: `hotStyleReferencePrompt` and `hotStyleReferenceImageUrls` from UI state.
- Produces: `generateSheinStudioDesigns` input with appended prompt and `productReferenceImageUrls`.

- [ ] **Step 1: Write failing generation wiring test**

In `generation-controller.test.ts`, add:

```ts
it("appends hot style reference prompt and image urls to design generation input", async () => {
  const generateDesigns = vi.fn().mockResolvedValue({
    prompt: "summer",
    transparentBackground: false,
    images: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    warnings: [],
  });

  await runSheinStudioGeneration({
    ...baseGenerationInput,
    prompt: "summer",
    hotStyleReferencePrompt: "Create an original retro badge.",
    hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
    generateDesigns,
  });

  expect(generateDesigns).toHaveBeenCalledWith(
    expect.objectContaining({
      prompt: expect.stringContaining("Create an original retro badge."),
      productReferenceImageUrls: expect.arrayContaining(["https://example.com/ref.png"]),
    }),
    expect.anything(),
  );
});
```

Use the existing helper name in the file if it differs from `runSheinStudioGeneration`; the assertion shape is the important contract.

- [ ] **Step 2: Run test to verify it fails**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/lib/shein-studio/generation-controller.test.ts
```

Expected: FAIL because reference fields are not wired into generation.

- [ ] **Step 3: Add prompt/reference composition helper**

In `generation-controller.ts`, add:

```ts
export function buildHotStyleReferenceGenerationInput(input: {
  prompt: string;
  hotStyleReferencePrompt?: string;
  productReferenceImageUrls?: string[];
  hotStyleReferenceImageUrls?: string[];
}) {
  const promptParts = [input.prompt.trim()];
  const referencePrompt = input.hotStyleReferencePrompt?.trim();
  if (referencePrompt) {
    promptParts.push(
      "Hot-selling reference direction for original artwork:",
      referencePrompt,
    );
  }
  const productReferenceImageUrls = Array.from(
    new Set([
      ...(input.productReferenceImageUrls ?? []),
      ...(input.hotStyleReferenceImageUrls ?? []),
    ].map((item) => item.trim()).filter(Boolean)),
  );
  return {
    prompt: promptParts.filter(Boolean).join("\n"),
    productReferenceImageUrls,
  };
}
```

Use this helper immediately before calling `generateSheinStudioDesigns`.

- [ ] **Step 4: Include fields in draft input and workbench actions**

Where draft inputs are assembled, add:

```ts
hotStyleReferenceImageUrls,
hotStyleReferenceBrief,
hotStyleReferencePrompt,
```

Where generation inputs are built from state, pass:

```ts
hotStyleReferenceImageUrls: state.hotStyleReferenceImageUrls,
hotStyleReferencePrompt: state.hotStyleReferencePrompt,
```

- [ ] **Step 5: Run focused generation and draft tests**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/lib/shein-studio/generation-controller.test.ts src/lib/shein-studio/draft-input.test.ts src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit generation wiring slice**

Run:

```powershell
git add web/listingkit-ui/src/components/listingkit/shein-studio web/listingkit-ui/src/lib/shein-studio
git commit -m "feat: use hot style references in studio generation"
```

---

### Task 6: Verification and Boundary Check

**Files:**
- Modify: none unless verification reveals a narrow defect from Tasks 1-5.

**Interfaces:**
- Consumes: completed backend and frontend slices.
- Produces: verified implementation ready for normal review or release work.

- [ ] **Step 1: Run backend focused suites**

Run:

```powershell
go test ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi -run "ReferenceStyle|Studio|Route|Descriptor" -count=1
```

Expected: PASS.

- [ ] **Step 2: Run frontend focused suites**

Run from `web/listingkit-ui`:

```powershell
npm test -- --run src/lib/api/shein-studio.test.ts src/lib/shein-studio/storage-shared.test.ts src/lib/shein-studio/generation-controller.test.ts src/lib/shein-studio/draft-input.test.ts src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
```

Expected: PASS.

- [ ] **Step 3: Run frontend static checks**

Run from `web/listingkit-ui`:

```powershell
npm run lint
npm run typecheck
```

Expected: PASS. If `typecheck` fails because of existing generated `.next/dev/types/routes.d.ts` cache noise, clear `.next` and rerun once before changing feature code.

- [ ] **Step 4: Run backend package build check**

Run:

```powershell
go test ./internal/listingkit -count=1
```

Expected: PASS.

- [ ] **Step 5: Inspect final diff scope**

Run:

```powershell
git status --short
git diff --stat HEAD~5..HEAD
```

Expected: only ListingKit reference-analysis backend files, Studio API/route files, and SHEIN Studio frontend files changed. Unrelated pre-existing files should remain unstaged and uncommitted.

- [ ] **Step 6: Commit verification cleanup if needed**

If verification required a narrow fix, run:

```powershell
git add internal/listingkit web/listingkit-ui/src/lib web/listingkit-ui/src/components/listingkit/shein-studio
git commit -m "fix: stabilize hot style reference workflow"
```

If no cleanup was needed, do not create an empty commit.

---

## Self-Review

Spec coverage:

- Reference image analysis is covered by Task 1.
- API endpoint is covered by Task 2.
- Frontend types, API wrapper, and persistence are covered by Task 3.
- Studio form controls are covered by Task 4.
- Existing generation chain integration is covered by Task 5.
- Verification and non-goal protection are covered by Task 6.

Placeholder scan:

- The plan contains no `TBD`, `TODO`, or intentionally incomplete steps.
- Each task has concrete files, commands, expected results, and commit commands.

Type consistency:

- Backend request and response use `StudioReferenceAnalysisRequest` and `StudioReferenceAnalysisResponse`.
- Frontend request and response use `SheinStudioReferenceAnalysisRequest` and `SheinStudioReferenceAnalysisResponse`.
- Persisted frontend field names are consistently `hotStyleReferenceImageUrls`, `hotStyleReferenceBrief`, and `hotStyleReferencePrompt`.
