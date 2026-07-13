package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubSDSRepairHandlerService struct {
	session *listingkit.TaskSDSRepairSession
	result  *listingkit.TaskResult
	err     error
	req     *listingkit.ApplyTaskSDSRepairRequest
}

func (s *stubSDSRepairHandlerService) GetTaskSDSRepair(context.Context, string) (*listingkit.TaskSDSRepairSession, error) {
	return s.session, s.err
}

func (s *stubSDSRepairHandlerService) RepairAndRetryTaskSDS(_ context.Context, _ string, req *listingkit.ApplyTaskSDSRepairRequest) (*listingkit.TaskResult, error) {
	s.req = req
	return s.result, s.err
}

func TestRepairAndRetryTaskSDSReturnsUnprocessableForUnavailableLayer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubSDSRepairHandlerService{err: listingkit.ErrSDSRepairLayerUnavailable}
	h := &handler{taskSDSRepairService: svc}
	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/sds-repair/retry", h.RepairAndRetryTaskSDS)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/sds-repair/retry", strings.NewReader(`{"variants":[{"variant_id":101,"layer_id":"missing"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "sds_repair_layer_unavailable") {
		t.Fatalf("body = %s, want stable layer error", w.Body.String())
	}
	if svc.req == nil || len(svc.req.Variants) != 1 {
		t.Fatalf("request = %+v, want bound selection", svc.req)
	}
}

func TestGetTaskSDSRepairReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler{taskSDSRepairService: &stubSDSRepairHandlerService{err: errors.New("wrapped: " + listingkit.ErrTaskNotFound.Error())}}
	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/sds-repair", h.GetTaskSDSRepair)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/sds-repair", nil))
	if w.Code == http.StatusOK {
		t.Fatalf("status = %d, want non-200", w.Code)
	}
}
