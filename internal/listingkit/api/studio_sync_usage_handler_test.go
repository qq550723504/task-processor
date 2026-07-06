package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

type studioSyncUsageCase struct {
	name     string
	path     string
	body     string
	metric   string
	register func(*gin.Engine, *handler)
	seed     func(*stubStudioMediaHandlerService)
}

func studioSyncUsageCases() []studioSyncUsageCase {
	return []studioSyncUsageCase{
		{
			name:   "designs",
			path:   "/api/v1/listing-kits/studio/designs",
			body:   `{"prompt":"retro cherries","count":1}`,
			metric: "design_jobs",
			register: func(router *gin.Engine, h *handler) {
				router.POST("/api/v1/listing-kits/studio/designs", h.GenerateStudioDesigns)
			},
			seed: func(s *stubStudioMediaHandlerService) {
				s.studioDesigns = &listingkit.StudioDesignResponse{
					Prompt: "retro cherries",
					Images: []listingkit.StudioGeneratedImage{{
						ID:       "design-1",
						ImageURL: "https://example.com/design.png",
					}},
				}
			},
		},
		{
			name:   "product_images",
			path:   "/api/v1/listing-kits/studio/product-images",
			body:   `{"prompt":"model wearing print","source_design_url":"https://example.com/design.png","count":1}`,
			metric: "product_image_jobs",
			register: func(router *gin.Engine, h *handler) {
				router.POST("/api/v1/listing-kits/studio/product-images", h.GenerateStudioProductImages)
			},
			seed: func(s *stubStudioMediaHandlerService) {
				s.studioProductImages = &listingkit.StudioProductImageResponse{
					Images: []listingkit.StudioGeneratedImage{{
						ID:       "product-image-1",
						ImageURL: "https://example.com/product.png",
					}},
				}
			},
		},
		{
			name:   "shein_image_regeneration",
			path:   "/api/v1/listing-kits/tasks/task-1/shein-images/regenerate",
			body:   `{"image_url":"https://example.com/old.png","prompt":"replace background"}`,
			metric: "image_regenerations",
			register: func(router *gin.Engine, h *handler) {
				router.POST("/api/v1/listing-kits/tasks/:task_id/shein-images/regenerate", h.RegenerateSheinDataImage)
			},
			seed: func(s *stubStudioMediaHandlerService) {
				s.regeneratedSheinImage = &listingkit.RegenerateSheinDataImageResponse{
					Preview: &listingkit.ListingKitPreview{TaskID: "task-1"},
					Image: listingkit.StudioGeneratedImage{
						ID:       "regenerated-1",
						ImageURL: "https://example.com/new.png",
					},
					ReplacedURL: "https://example.com/old.png",
				}
			},
		},
	}
}

func TestStudioSyncHandlersDoNotRecordUsageForInvalidJSON(t *testing.T) {
	t.Parallel()

	for _, tc := range studioSyncUsageCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w, subscriptionService := serveStudioSyncUsageRequest(t, tc, &stubStudioMediaHandlerService{}, `{`)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 body=%s", w.Code, w.Body.String())
			}
			if usage := studioUsageMetric(t, subscriptionService, tc.metric); usage != 0 {
				t.Fatalf("%s usage = %d, want 0 for invalid JSON", tc.metric, usage)
			}
		})
	}
}

func TestStudioSyncHandlersDoNotRecordUsageWhenServiceFails(t *testing.T) {
	t.Parallel()

	for _, tc := range studioSyncUsageCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &stubStudioMediaHandlerService{err: errors.New("invalid request: generation failed")}
			tc.seed(service)
			w, subscriptionService := serveStudioSyncUsageRequest(t, tc, service, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 body=%s", w.Code, w.Body.String())
			}
			if usage := studioUsageMetric(t, subscriptionService, tc.metric); usage != 0 {
				t.Fatalf("%s usage = %d, want 0 for service failure", tc.metric, usage)
			}
		})
	}
}

func TestStudioSyncHandlersRecordUsageAfterSuccess(t *testing.T) {
	t.Parallel()

	for _, tc := range studioSyncUsageCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &stubStudioMediaHandlerService{}
			tc.seed(service)
			w, subscriptionService := serveStudioSyncUsageRequest(t, tc, service, tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
			}
			if usage := studioUsageMetric(t, subscriptionService, tc.metric); usage != 1 {
				t.Fatalf("%s usage = %d, want 1 after success", tc.metric, usage)
			}
		})
	}
}

func serveStudioSyncUsageRequest(t *testing.T, tc studioSyncUsageCase, service *stubStudioMediaHandlerService, body string) (*httptest.ResponseRecorder, *listingsubscription.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	subscriptionService := activeStudioSubscriptionService(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioMediaService(service), WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	router := gin.New()
	tc.register(router, h)

	req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w, subscriptionService
}

func studioUsageMetric(t *testing.T, service *listingsubscription.Service, metric string) int {
	t.Helper()
	summary, err := service.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	for _, item := range summary.Entitlements {
		if item.Module.Code == listingsubscription.ModuleStudio {
			return item.Used[metric]
		}
	}
	return 0
}
