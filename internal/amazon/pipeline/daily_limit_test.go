package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	amazonmodel "task-processor/internal/amazon/model"
	"task-processor/internal/app/state"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
)

func TestDailyLimitHandlerReservesQuotaWhenUnderLimit(t *testing.T) {
	limit := 5
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/store/try-consume-daily-quota" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"allowed":      true,
				"newCount":     3,
				"remaining":    2,
				"reachedLimit": false,
			},
		})
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("test-token", "1")

	taskContext := &amazonmodel.TaskContext{
		Data: map[string]any{
			"tenantId": int64(1),
			"storeId":  int64(2),
		},
		StoreInfo: &managementapi.StoreRespDTO{
			ID:             2,
			DailyLimit:     &limit,
			DailyLimitType: "SPU",
		},
	}

	handler := NewDailyLimitHandler(&amazonmodel.Services{
		MemoryManager: state.NewMemoryManager(context.Background(), clientMgr),
	})
	if err := handler.Handle(context.Background(), taskContext); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !taskContext.DailyQuotaReserved {
		t.Fatal("expected daily quota reservation to be recorded")
	}
	if taskContext.DailyQuotaIncrement != 1 {
		t.Fatalf("expected increment 1, got %d", taskContext.DailyQuotaIncrement)
	}
}

func TestDailyLimitHandlerReturnsNonRetryableWhenLimitReached(t *testing.T) {
	limit := 5
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/store/try-consume-daily-quota" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"allowed":      false,
				"newCount":     5,
				"remaining":    0,
				"reachedLimit": true,
			},
		})
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("test-token", "1")

	taskContext := &amazonmodel.TaskContext{
		Data: map[string]any{
			"tenantId": int64(1),
			"storeId":  int64(2),
		},
		StoreInfo: &managementapi.StoreRespDTO{
			ID:             2,
			DailyLimit:     &limit,
			DailyLimitType: "SPU",
		},
	}

	handler := NewDailyLimitHandler(&amazonmodel.Services{
		MemoryManager: state.NewMemoryManager(context.Background(), clientMgr),
	})
	err := handler.Handle(context.Background(), taskContext)
	if err == nil {
		t.Fatal("expected nonretryable error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "nonretryable:") {
		t.Fatalf("expected nonretryable error, got %v", err)
	}
}
