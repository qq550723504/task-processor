package validation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/app/state"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	sheinctx "task-processor/internal/shein/context"
)

func TestCheckDailyLimitHandlerReturnsPausedHandledErrorWhenLimitReached(t *testing.T) {
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

	mem := state.NewMemoryManager(context.Background(), clientMgr)

	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			Task:          &model.Task{ID: 1, TenantID: 1, StoreID: 2},
			MemoryManager: mem,
			StoreInfo: &managementapi.StoreRespDTO{
				ID:             2,
				DailyLimit:     &limit,
				DailyLimitType: "SPU",
			},
		},
	}

	err := NewCheckDailyLimitHandler().Handle(ctx)
	if err == nil {
		t.Fatal("expected handled error")
	}

	handledErr, ok := shein.AsTaskHandledError(err)
	if !ok {
		t.Fatalf("expected TaskHandledError, got %T", err)
	}
	if handledErr.TargetStatus() != model.TaskStatusPaused {
		t.Fatalf("target status = %v, want %v", handledErr.TargetStatus(), model.TaskStatusPaused)
	}
	wantMsg := "[DAILY_LIMIT_REACHED] 店铺已达到每日上架限额(5/5)，已暂停上架到当日结束"
	if handledErr.ErrorMessage() != wantMsg {
		t.Fatalf("error message = %q, want %q", handledErr.ErrorMessage(), wantMsg)
	}
}
