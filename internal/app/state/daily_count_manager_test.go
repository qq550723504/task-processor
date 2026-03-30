package state

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
)

func TestDailyCountManagerIncrementCountSerializesPerStoreDate(t *testing.T) {
	var (
		mu    sync.Mutex
		count int64
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc-api/listing/store/get-daily-listing-count":
			mu.Lock()
			current := count
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": current,
			})
		case "/rpc-api/listing/store/set-daily-listing-count":
			var req struct {
				Count int64 `json:"count"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			time.Sleep(20 * time.Millisecond)

			mu.Lock()
			count = req.Count
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
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
	manager := NewDailyCountManager(clientMgr)

	var wg sync.WaitGroup
	results := make(chan int64, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- manager.IncrementCount(1, 2, "2026-03-30", 1)
		}()
	}

	wg.Wait()
	close(results)

	var maxResult int64
	for result := range results {
		if result > maxResult {
			maxResult = result
		}
	}

	mu.Lock()
	finalCount := count
	mu.Unlock()

	if finalCount != 2 {
		t.Fatalf("expected final count 2, got %d", finalCount)
	}
	if maxResult != 2 {
		t.Fatalf("expected one increment call to observe count 2, got %d", maxResult)
	}
}
