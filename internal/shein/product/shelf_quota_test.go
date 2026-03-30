package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/app/state"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	otherclient "task-processor/internal/shein/api/other"
	sheinclient "task-processor/internal/shein/client"
	sheinctx "task-processor/internal/shein/context"

	"github.com/imroc/req/v3"
)

func TestShelfQuotaHandlerReturnsPausedHandledErrorWhenQuotaExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sheinclient.GetQueryShelfQuotaEndpoint() {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "0",
			"msg":  "success",
			"info": map[string]any{
				"need":              true,
				"remain_count":      0,
				"total_quota_count": 10,
				"on_shelf_count":    10,
			},
		})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			Task:          &model.Task{ID: 1, TenantID: 1, StoreID: 2},
			MemoryManager: state.NewMemoryManager(context.Background(), nil),
		},
		APIClients: sheinctx.APIClients{
			OtherAPI: otherclient.NewClient(baseClient),
		},
	}

	err := NewShelfQuotaHandler().Handle(ctx)
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
	wantMsg := "[SHELF_QUOTA_EXHAUSTED] SKC上架额度不足"
	if handledErr.ErrorMessage() != wantMsg {
		t.Fatalf("error message = %q, want %q", handledErr.ErrorMessage(), wantMsg)
	}
}
