package publish

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/state"
)

func TestSavePublishResultCalculateIncrementUsesStoreLimitType(t *testing.T) {
	handler := NewSavePublishResultHandler()
	limit := 100
	input := &PublishResultInput{
		StoreInfo: &listingruntime.StoreInfo{
			DailyLimit:     &limit,
			DailyLimitType: "SKU",
		},
		SheinResponse: &sheinproduct.SheinResponse{
			Info: sheinproduct.ResponseInfo{
				SKCList: []sheinproduct.ResponseSKC{
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-1"},
							{SKUCode: "sku-2"},
						},
					},
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-3"},
						},
					},
				},
			},
		},
	}

	got := handler.calculateIncrement(input)
	if got != 3 {
		t.Fatalf("expected SKU increment 3, got %d", got)
	}
}

func TestSavePublishResultRecordDailyListingCountWithoutDailyLimit(t *testing.T) {
	var count int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/rpc-api/listing/store/get-daily-listing-count":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": count,
			})
		case "/rpc-api/listing/store/set-daily-listing-count":
			var req struct {
				Count int64 `json:"count"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			count = req.Count
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": 0,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{
		BaseURL: server.URL,
	})
	clientMgr.GetClient()
	clientMgr.SetUserToken("test-token", "1")

	handler := NewSavePublishResultHandler()
	input := &PublishResultInput{
		Task: &model.Task{
			TenantID: 1,
			StoreID:  2,
		},
		MemoryManager: &state.MemoryManager{
			DailyCountManager: state.NewDailyCountManager(clientMgr),
		},
		StoreInfo: &listingruntime.StoreInfo{
			ID:             2,
			DailyLimit:     nil,
			DailyLimitType: "SKU",
		},
		SheinResponse: &sheinproduct.SheinResponse{
			Info: sheinproduct.ResponseInfo{
				SKCList: []sheinproduct.ResponseSKC{
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-1"},
							{SKUCode: "sku-2"},
						},
					},
				},
			},
		},
	}

	handler.recordDailyListingCount(nil, input)

	if count != 2 {
		t.Fatalf("expected daily listing count 2 without daily limit, got %d", count)
	}
}
