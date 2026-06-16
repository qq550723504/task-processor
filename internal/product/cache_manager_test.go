package product

import (
	"encoding/json"
	"errors"
	"testing"

	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

type stubCacheManagerRawJSONClient struct {
	createErr      error
	rawResp        *RawJsonResp
	createCalls    int
	anyFreshnessOK bool
}

func (s *stubCacheManagerRawJSONClient) GetRawJsonData(*RawJsonReq) (*RawJsonResp, error) {
	if s.rawResp != nil {
		return s.rawResp, nil
	}
	return nil, errors.New("cache miss")
}

func (s *stubCacheManagerRawJSONClient) CreateRawJsonData(*RawJsonCreateReq) (int64, error) {
	s.createCalls++
	return 0, s.createErr
}

func (s *stubCacheManagerRawJSONClient) GetRawJsonDataAnyFreshness(*RawJsonReq) (*RawJsonResp, error) {
	if s.anyFreshnessOK && s.rawResp != nil {
		return s.rawResp, nil
	}
	return nil, errors.New("cache miss")
}

func TestCacheManagerSaveToCacheIgnoresDuplicateKeyViolation(t *testing.T) {
	t.Parallel()

	client := &stubCacheManagerRawJSONClient{
		createErr: errors.New(`ERROR: duplicate key value violates unique constraint "uk_listing_raw_json_data_product_region" (SQLSTATE 23505)`),
	}
	manager := NewCacheManager(client, logrus.NewEntry(logrus.New()))

	err := manager.SaveToCache(&FetchRequest{
		TenantID:  1,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B001",
		Creator:   "tester",
	}, &model.Product{Asin: "B001"})
	if err != nil {
		t.Fatalf("SaveToCache() error = %v, want nil for duplicate cache entry", err)
	}
}

func TestCacheManagerCacheProductOverwritesStaleCacheRecordWhenShipsFromMissing(t *testing.T) {
	t.Parallel()

	client := &stubCacheManagerRawJSONClient{
		rawResp: &RawJsonResp{
			ID:          42,
			Platform:    "amazon",
			ProductID:   "B001",
			Region:      "us",
			RawJSONData: `{"asin":"B001","shipsFrom":""}`,
		},
		anyFreshnessOK: true,
	}
	manager := NewCacheManager(client, logrus.NewEntry(logrus.New()))

	err := manager.CacheProduct(&FetchRequest{
		TenantID:  1,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B001",
		Creator:   "tester",
	}, &model.Product{Asin: "B001", ShipsFrom: "Amazon"})
	if err != nil {
		t.Fatalf("CacheProduct() error = %v", err)
	}
	if client.createCalls != 1 {
		t.Fatalf("CreateRawJsonData() calls = %d, want 1 when stale cache record should be overwritten", client.createCalls)
	}
}

func TestCacheManagerDecideRawCacheWrite(t *testing.T) {
	t.Parallel()

	freshRaw, err := json.Marshal(&model.Product{Asin: "B001", ShipsFrom: "Amazon"})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	tests := []struct {
		name   string
		client *stubCacheManagerRawJSONClient
		want   cacheWriteDecision
	}{
		{
			name: "skip when fresh cache already usable",
			client: &stubCacheManagerRawJSONClient{
				rawResp: &RawJsonResp{
					RawJSONData: string(freshRaw),
				},
			},
			want: cacheWriteSkip,
		},
		{
			name: "save when stale cache needs overwrite",
			client: &stubCacheManagerRawJSONClient{
				rawResp: &RawJsonResp{
					RawJSONData: `{"asin":"B001","shipsFrom":""}`,
				},
				anyFreshnessOK: true,
			},
			want: cacheWriteSave,
		},
		{
			name: "skip when only old raw record exists but should not overwrite",
			client: &stubCacheManagerRawJSONClient{
				rawResp: &RawJsonResp{
					RawJSONData: string(freshRaw),
				},
				anyFreshnessOK: true,
			},
			want: cacheWriteSkip,
		},
		{
			name:   "save when no cache record exists",
			client: &stubCacheManagerRawJSONClient{},
			want:   cacheWriteSave,
		},
	}

	req := &FetchRequest{
		TenantID:  1,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B001",
		Creator:   "tester",
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			manager := NewCacheManager(tt.client, logrus.NewEntry(logrus.New()))
			if got := manager.decideRawCacheWrite(req); got != tt.want {
				t.Fatalf("decideRawCacheWrite() = %v, want %v", got, tt.want)
			}
		})
	}
}
