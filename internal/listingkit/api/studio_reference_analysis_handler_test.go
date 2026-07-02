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
	t.Parallel()

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
		t.Fatalf("service request = %+v, want reference image", service.req)
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
	t.Parallel()

	gin.SetMode(gin.TestMode)
	service := &stubStudioReferenceAnalysisService{err: errors.New("invalid request: reference_image_urls is required")}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(service))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
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
