package activity

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"

	"github.com/sirupsen/logrus"
)

func TestGetStoreInfoPrefersRepository(t *testing.T) {
	service := &activityRegistrationServiceImpl{
		storeRepo: stubActivityStoreRepo{
			store: &listingadmin.Store{
				ID:       11,
				TenantID: 22,
				Username: "repo-user",
			},
		},
		logger: logrus.NewEntry(logrus.New()),
	}

	store, err := service.getStoreInfo(context.Background(), 11)
	if err != nil {
		t.Fatalf("getStoreInfo() error = %v", err)
	}
	if store == nil || store.Username != "repo-user" {
		t.Fatalf("getStoreInfo() = %+v, want repository store", store)
	}
}

func TestGetMappingByPlatformProductIDAndStorePrefersRepository(t *testing.T) {
	service := &activityRegistrationServiceImpl{
		mappingRepo: stubActivityMappingRepo{
			mapping: &listingadmin.ProductImportMapping{
				PlatformProductID: "SKC-1",
				StoreID:           8,
				ProductID:         "ASIN-1",
				CostPrice:         activityTestFloat64Ptr(10),
			},
		},
		logger: logrus.NewEntry(logrus.New()),
	}

	mapping, err := service.getMappingByPlatformProductIDAndStore(context.Background(), "SKC-1", 8)
	if err != nil {
		t.Fatalf("getMappingByPlatformProductIDAndStore() error = %v", err)
	}
	if mapping == nil || mapping.ProductID != "ASIN-1" {
		t.Fatalf("getMappingByPlatformProductIDAndStore() = %+v, want repository mapping", mapping)
	}
}

type stubActivityStoreRepo struct {
	store *listingadmin.Store
}

func (s stubActivityStoreRepo) FindStoreByID(_ context.Context, _ int64) (*listingadmin.Store, error) {
	return s.store, nil
}

type stubActivityMappingRepo struct {
	mapping *listingadmin.ProductImportMapping
}

func (s stubActivityMappingRepo) FindLatest(_ context.Context, _ listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error) {
	return s.mapping, nil
}

func activityTestFloat64Ptr(v float64) *float64 { return &v }
