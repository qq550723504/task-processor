package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubRevisionValidateService struct {
	stubTaskLifecycleHandlerService
	result *listingkit.RevisionValidationResult
	err    error
}

func (s *stubRevisionValidateService) ValidateTaskRevision(_ context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.RevisionValidationResult, error) {
	return s.result, s.err
}

func TestValidateTaskRevisionReturnsValidationPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubRevisionValidateService{
		result: &listingkit.RevisionValidationResult{
			TaskID:   "task-1",
			Platform: "shein",
			Valid:    false,
			FieldErrors: []listingkit.RevisionFieldError{{
				FieldPath: "shein.category_id",
				Code:      "invalid_value",
				Message:   "category_id 必须大于 0",
			}},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/revision/validate", h.ValidateTaskRevision)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/revision/validate", strings.NewReader(`{"platform":"shein","shein":{"category_id":0}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var body listingkit.RevisionValidationResult
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Valid {
		t.Fatalf("valid = true, want false; body=%+v", body)
	}
	if len(body.FieldErrors) != 1 || body.FieldErrors[0].FieldPath != "shein.category_id" {
		t.Fatalf("field errors = %+v", body.FieldErrors)
	}
}

func TestValidateTaskRevisionReturnsNotFoundForMissingRestoreRevision(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubRevisionValidateService{
		err: listingkit.ErrRevisionHistoryRecordNotFound,
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/revision/validate", h.ValidateTaskRevision)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/revision/validate", strings.NewReader(`{"platform":"shein","restore_from_revision_id":"missing"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}
