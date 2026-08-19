package sheinloginmanaged

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/sheinlogin"
)

func TestNewAccountProviderReturnsProvider(t *testing.T) {
	if NewAccountProviderWithStoreClientFactory(nil) == nil {
		t.Fatal("expected account provider")
	}
}

func TestNewStoreSyncClientFactoryReturnsFactory(t *testing.T) {
	if NewStoreSyncClientFactoryWithStoreAPI(nil) == nil {
		t.Fatal("expected store sync factory")
	}
}

func TestFindDuplicateStoreIgnoresDisabledHistoricalStore(t *testing.T) {
	page := &listingadmin.PageResult[*listingadmin.StoreRespDTO]{
		List: []*listingadmin.StoreRespDTO{{
			ID:       560,
			TenantID: 246,
			Platform: "SHEIN",
			StoreID:  "9843414915",
			Status:   1,
		}},
		Total:    1,
		PageNo:   1,
		PageSize: 200,
	}

	duplicate, err := findDuplicateStore(
		context.Background(),
		stubStoreAPI{page: page},
		sheinlogin.Account{StoreID: 867, TenantID: 246},
		"9843414915",
	)
	if err != nil {
		t.Fatalf("find duplicate store: %v", err)
	}
	if duplicate != nil {
		t.Fatalf("duplicate = %+v, want disabled historical store to be ignored", duplicate)
	}
}
