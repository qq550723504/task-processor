package inventory

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"

	"github.com/sirupsen/logrus"
)

type fakeSheinInventoryProductRepo struct {
	upsertItems    []listingadmin.ProductData
	attributeItems []listingadmin.ProductData
	page           *listingadmin.ProductDataPage
}

func (f *fakeSheinInventoryProductRepo) ListProductData(context.Context, listingadmin.ProductDataQuery) (*listingadmin.ProductDataPage, error) {
	return f.page, nil
}
func (f *fakeSheinInventoryProductRepo) GetProductData(context.Context, int64, int64) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeSheinInventoryProductRepo) CreateProductData(context.Context, *listingadmin.ProductData) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeSheinInventoryProductRepo) UpdateProductData(context.Context, *listingadmin.ProductData) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeSheinInventoryProductRepo) UpdateProductDataStatus(context.Context, int64, int64, int16) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeSheinInventoryProductRepo) DeleteProductData(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (f *fakeSheinInventoryProductRepo) UpsertProductDataBatch(_ context.Context, items []listingadmin.ProductData) (int, error) {
	f.upsertItems = append([]listingadmin.ProductData(nil), items...)
	return len(items), nil
}
func (f *fakeSheinInventoryProductRepo) BatchUpdateAttributesByPlatformProductID(_ context.Context, items []listingadmin.ProductData) (int, error) {
	f.attributeItems = append([]listingadmin.ProductData(nil), items...)
	return len(items), nil
}

func TestSaveInventoryProductSnapshotPrefersRepository(t *testing.T) {
	repo := &fakeSheinInventoryProductRepo{}
	service := &inventorySyncServiceImpl{productDataRepo: repo, logger: logrus.NewEntry(logrus.New())}
	prod := &InventoryProductSnapshot{TenantID: 1, StoreID: 2, Platform: "SHEIN", ProductID: "p1", PlatformProductID: "pp1"}

	if err := service.saveInventoryProductSnapshot(context.Background(), prod); err != nil {
		t.Fatalf("saveInventoryProductSnapshot() error = %v", err)
	}
	if len(repo.upsertItems) != 1 || repo.upsertItems[0].PlatformProductID != "pp1" {
		t.Fatalf("unexpected repo items: %#v", repo.upsertItems)
	}
}

func TestUpdateInventoryProductAttributesPrefersRepository(t *testing.T) {
	repo := &fakeSheinInventoryProductRepo{}
	service := &inventorySyncServiceImpl{productDataRepo: repo, logger: logrus.NewEntry(logrus.New())}
	prod := &InventoryProductSnapshot{TenantID: 1, StoreID: 2, Platform: "SHEIN", PlatformProductID: "pp1"}

	count, err := service.updateInventoryProductAttributes(context.Background(), prod, `{"a":1}`)
	if err != nil {
		t.Fatalf("updateInventoryProductAttributes() error = %v", err)
	}
	if count != 1 || len(repo.attributeItems) != 1 || string(repo.attributeItems[0].Attributes) != `{"a":1}` {
		t.Fatalf("unexpected repo attribute items: %#v", repo.attributeItems)
	}
}

func TestFetchProductsForInventorySyncPrefersRepository(t *testing.T) {
	repo := &fakeSheinInventoryProductRepo{
		page: &listingadmin.ProductDataPage{
			Items: []listingadmin.ProductData{{TenantID: 1, StoreID: sheinInvPtrInt64(2), Platform: "SHEIN", ProductID: "p1", PlatformProductID: "pp1"}},
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

func TestGetStoreSiteAbbrUsesRepositoryStore(t *testing.T) {
	service := &inventorySyncServiceImpl{
		storeRepo: stubSheinInventoryStoreRepo{
			store: &listingadmin.Store{ID: 2, Region: "US"},
		},
		logger: logrus.NewEntry(logrus.New()),
	}

	got, err := service.getStoreSiteAbbr(2)
	if err != nil {
		t.Fatalf("getStoreSiteAbbr() error = %v", err)
	}
	if got != "shein-us" {
		t.Fatalf("getStoreSiteAbbr() = %q, want shein-us", got)
	}
}

type stubSheinInventoryStoreRepo struct {
	store *listingadmin.Store
}

func (s stubSheinInventoryStoreRepo) FindStoreByID(_ context.Context, _ int64) (*listingadmin.Store, error) {
	return s.store, nil
}
