package amazon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	amazonModel "task-processor/internal/amazon/model"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/state"

	"github.com/sirupsen/logrus"
)

func TestParseTaskStatusMetadata(t *testing.T) {
	reasonCode, stage := parseTaskStatusMetadata("NONRETRYABLE: [stage:check_daily_limit] [DAILY_LIMIT_REACHED] limit reached")
	if reasonCode != amazonTaskReasonDailyLimitReached {
		t.Fatalf("reasonCode = %q, want %q", reasonCode, amazonTaskReasonDailyLimitReached)
	}
	if stage != "check_daily_limit" {
		t.Fatalf("stage = %q, want check_daily_limit", stage)
	}
}

func TestShouldPauseAmazonTask(t *testing.T) {
	if !shouldPauseAmazonTask(amazonTaskReasonAuthExpired) {
		t.Fatal("auth expired should pause task")
	}
	if !shouldPauseAmazonTask(amazonTaskReasonDailyLimitReached) {
		t.Fatal("daily limit should pause task")
	}
	if shouldPauseAmazonTask("VALIDATION_FAILED") {
		t.Fatal("validation failure should not pause task")
	}
}

func TestIsAmazonRetryableTaskError(t *testing.T) {
	if isAmazonRetryableTaskError(assertError("NONRETRYABLE: [stage:validate_input] [VALIDATION_FAILED] bad input")) {
		t.Fatal("nonretryable error should not be retried")
	}
	if !isAmazonRetryableTaskError(assertError("[stage:create_listing] [RATE_LIMIT] slow down")) {
		t.Fatal("rate limit error should remain retryable")
	}
}

func TestRecordAmazonDailyListingCountWithoutDailyLimit(t *testing.T) {
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

	recordAmazonDailyListingCount(
		&model.Task{
			TenantID: 1,
			StoreID:  2,
		},
		&amazonModel.TaskContext{
			StoreInfo: &managementapi.StoreRespDTO{
				ID:             2,
				DailyLimitType: "SPU",
			},
		},
		&state.MemoryManager{
			DailyCountManager: state.NewDailyCountManager(clientMgr),
		},
		logrus.New(),
	)

	if count != 1 {
		t.Fatalf("expected daily listing count 1 without daily limit, got %d", count)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
