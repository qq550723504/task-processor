package sync

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	managementapi "task-processor/internal/listingadmin"

	"github.com/sirupsen/logrus"
)

type fakeTemuInventoryProductRepo struct {
	upsertItems    []listingadmin.ProductData
	attributeItems []listingadmin.ProductData
	page           *listingadmin.ProductDataPage
}

func (f *fakeTemuInventoryProductRepo) ListProductData(context.Context, listingadmin.ProductDataQuery) (*listingadmin.ProductDataPage, error) {
	return f.page, nil
}
func (f *fakeTemuInventoryProductRepo) GetProductData(context.Context, int64, int64) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuInventoryProductRepo) CreateProductData(context.Context, *listingadmin.ProductData) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuInventoryProductRepo) UpdateProductData(context.Context, *listingadmin.ProductData) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuInventoryProductRepo) UpdateProductDataStatus(context.Context, int64, int64, int16) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuInventoryProductRepo) DeleteProductData(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (f *fakeTemuInventoryProductRepo) UpsertProductDataBatch(_ context.Context, items []listingadmin.ProductData) (int, error) {
	f.upsertItems = append([]listingadmin.ProductData(nil), items...)
	return len(items), nil
}
func (f *fakeTemuInventoryProductRepo) BatchUpdateAttributesByPlatformProductID(_ context.Context, items []listingadmin.ProductData) (int, error) {
	f.attributeItems = append([]listingadmin.ProductData(nil), items...)
	return len(items), nil
}

func TestSaveInventoryProductSnapshotPrefersRepository(t *testing.T) {
	repo := &fakeTemuInventoryProductRepo{}
	service := &inventorySyncServiceImpl{productDataRepo: repo, logger: logrus.NewEntry(logrus.New())}
	prod := &TemuInventoryProductSnapshot{TenantID: 1, StoreID: 2, Platform: "TEMU", ProductID: "p1", PlatformProductID: "pp1"}

	if err := service.saveInventoryProductSnapshot(context.Background(), prod); err != nil {
		t.Fatalf("saveInventoryProductSnapshot() error = %v", err)
	}
	if len(repo.upsertItems) != 1 || repo.upsertItems[0].PlatformProductID != "pp1" {
		t.Fatalf("unexpected repo items: %#v", repo.upsertItems)
	}
}

func TestUpdateInventoryProductAttributesPrefersRepository(t *testing.T) {
	repo := &fakeTemuInventoryProductRepo{}
	service := &inventorySyncServiceImpl{productDataRepo: repo, logger: logrus.NewEntry(logrus.New())}
	prod := &TemuInventoryProductSnapshot{TenantID: 1, StoreID: 2, Platform: "TEMU", PlatformProductID: "pp1"}

	count, err := service.updateInventoryProductAttributes(context.Background(), prod, `{"a":1}`)
	if err != nil {
		t.Fatalf("updateInventoryProductAttributes() error = %v", err)
	}
	if count != 1 || len(repo.attributeItems) != 1 || string(repo.attributeItems[0].Attributes) != `{"a":1}` {
		t.Fatalf("unexpected repo attribute items: %#v", repo.attributeItems)
	}
}

func TestFetchProductsForInventorySyncPrefersRepository(t *testing.T) {
	repo := &fakeTemuInventoryProductRepo{
		page: &listingadmin.ProductDataPage{
			Items: []listingadmin.ProductData{{TenantID: 1, StoreID: temuSyncPtrInt64(2), Platform: "TEMU", ProductID: "p1", PlatformProductID: "pp1"}},
		},
	}
	service := &inventorySyncServiceImpl{productDataRepo: repo, logger: logrus.NewEntry(logrus.New())}

	products, err := service.FetchProductsForInventorySync(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("FetchProductsForInventorySync() error = %v", err)
	}
	if len(products) != 1 || products[0].PlatformProductID != "pp1" {
		t.Fatalf("unexpected products: %#v", products)
	}
}

func TestGetStorePriceTypeUsesRepositoryStore(t *testing.T) {
	service := &inventorySyncServiceImpl{
		storeRepo: stubTemuInventoryStoreRepo{
			store: &listingadmin.Store{ID: 2, PriceType: "original"},
		},
		logger: logrus.NewEntry(logrus.New()),
	}

	if got := service.getStorePriceType(2); got != "original" {
		t.Fatalf("getStorePriceType() = %q, want original", got)
	}
}

type stubTemuInventoryStoreRepo struct {
	store *listingadmin.Store
}

func (s stubTemuInventoryStoreRepo) FindStoreByID(_ context.Context, _ int64) (*listingadmin.Store, error) {
	return s.store, nil
}

var _ = managementapi.StoreRespDTO{}
