package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
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
	h, err := NewHandler(svc)
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
