package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func TestStudioAsyncJobStartsAndReturnsSucceededDesignJob(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-1",
				ImageURL: "https://example.com/design.png",
			}},
		},
	}
	subscriptionService := activeStudioSubscriptionService(t)
	h, err := NewHandler(svc, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	var started struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal start body: %v", err)
	}
	if started.JobID == "" || started.Status != "running" {
		t.Fatalf("started = %+v, want running job id", started)
	}

	var polled struct {
		Status string                           `json:"status"`
		Result *listingkit.StudioDesignResponse `json:"result"`
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/"+started.JobID, nil)
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("poll status = %d, want 200 body=%s", resp.Code, resp.Body.String())
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &polled); err != nil {
			t.Fatalf("unmarshal poll body: %v", err)
		}
		if polled.Status == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if polled.Status != "succeeded" || polled.Result == nil || len(polled.Result.Images) != 1 {
		t.Fatalf("polled = %+v, want succeeded design result", polled)
	}
	if svc.studioDesignReq == nil || svc.studioDesignReq.Prompt != "retro cherries" {
		t.Fatalf("studio design req = %+v, want bound prompt", svc.studioDesignReq)
	}
}

func activeStudioSubscriptionService(t *testing.T) *listingsubscription.Service {
	t.Helper()
	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertEntitlement(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{Status: listingsubscription.StatusActive}); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestStudioAsyncJobRejectsUnknownPath(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/unknown","body":{}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestStudioAsyncJobReturnsNotFoundForMissingJob(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/missing", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestStudioAsyncJobFileStorePersistsCompletedJobs(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "studio-async-jobs.json")
	store, err := newStudioAsyncJobFileStore(storePath, studioAsyncJobTTL, studioAsyncJobMaxLen)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	job := store.create("/studio/designs")
	store.succeed(job.ID, map[string]any{
		"images": []map[string]any{{
			"id":        "design-1",
			"image_url": "https://example.com/design.png",
		}},
	})

	reloaded, err := newStudioAsyncJobFileStore(storePath, studioAsyncJobTTL, studioAsyncJobMaxLen)
	if err != nil {
		t.Fatalf("reload file store: %v", err)
	}
	persisted, ok := reloaded.get(job.ID)
	if !ok {
		t.Fatalf("persisted job %q not found after reload", job.ID)
	}
	if persisted.Status != studioAsyncJobSucceeded || persisted.Result == nil {
		t.Fatalf("persisted job = %+v, want succeeded result", persisted)
	}
}

func TestStudioAsyncJobHandlerUsesConfiguredFileStore(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "configured-studio-async-jobs.json")
	store, err := newStudioAsyncJobFileStore(storePath, studioAsyncJobTTL, studioAsyncJobMaxLen)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	job := store.create("/studio/designs")
	store.succeed(job.ID, map[string]any{"prompt": "persisted"})

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{}, WithStudioAsyncJobStorePath(storePath))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/"+job.ID, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	var payload struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.JobID != job.ID || payload.Status != "succeeded" {
		t.Fatalf("payload = %+v, want persisted succeeded job", payload)
	}
}

func TestStudioAsyncJobHandlerRejectsInvalidConfiguredFileStore(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}

	if _, err := NewHandler(&stubGenerationTaskService{}, WithStudioAsyncJobStorePath(filepath.Join(parentFile, "jobs.json"))); err == nil {
		t.Fatal("NewHandler returned nil error, want invalid file store error")
	}
}

func TestStudioAsyncJobHandlerUsesExplicitFileStorePathOption(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "option-configured-studio-async-jobs.json")
	store, err := newStudioAsyncJobFileStore(storePath, studioAsyncJobTTL, studioAsyncJobMaxLen)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	job := store.create("/studio/designs")
	store.succeed(job.ID, map[string]any{"prompt": "persisted"})

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{}, WithStudioAsyncJobStorePath(storePath))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/"+job.ID, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
}
