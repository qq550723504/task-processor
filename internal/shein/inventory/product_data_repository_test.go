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

func TestFetchProductsForInventorySyncPrefersSyncedProducts(t *testing.T) {
	attributes := `[{"skc_name":"skc-1","skc_code":"O1","sku_info":[{"sku_code":"I1","mapping_info":{"ProductID":"B0TEST","SKU":"seller-sku-1","PlatformProductID":"I1"}}]}]`
	source := &fakeSheinSyncedInventoryProductSource{
		records: []SyncedInventoryProductRecord{{
			TenantID:                1,
			StoreID:                 2,
			SKCName:                 "skc-1",
			SKCCode:                 "O1",
			ProductNameMulti:        "Synced product",
			MainImageURL:            "https://example.com/main.jpg",
			InventorySyncAttributes: attributes,
			IsActive:                true,
		}},
	}
	repo := &fakeSheinInventoryProductRepo{
		page: &listingadmin.ProductDataPage{
			Items: []listingadmin.ProductData{{TenantID: 1, StoreID: sheinInvPtrInt64(2), Platform: "SHEIN", PlatformProductID: "old-source"}},
		},
	}
	service := &inventorySyncServiceImpl{
		syncedProductSource: source,
		productDataRepo:     repo,
		logger:              logrus.NewEntry(logrus.New()),
	}

	products, err := service.FetchProductsForInventorySync(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("FetchProductsForInventorySync() error = %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected one synced product, got %#v", products)
	}
	if products[0].PlatformProductID != "skc-1" || products[0].ProductID != "B0TEST" {
		t.Fatalf("unexpected synced product identity: %#v", products[0])
	}
	if products[0].Attributes != attributes {
		t.Fatalf("synced product attributes = %q, want %q", products[0].Attributes, attributes)
	}
}

func TestFetchProductsForInventorySyncPaginatesSyncedProducts(t *testing.T) {
	records := make([]SyncedInventoryProductRecord, 0, 105)
	for i := 0; i < 105; i++ {
		records = append(records, SyncedInventoryProductRecord{
			TenantID:                1,
			StoreID:                 2,
			SKCName:                 "skc-" + string(rune('a'+i%26)),
			InventorySyncAttributes: `[{"skc_name":"skc","skc_code":"O1","sku_info":[]}]`,
			IsActive:                true,
		})
	}
	source := &fakeSheinSyncedInventoryProductSource{records: records, pageCap: 100}
	service := &inventorySyncServiceImpl{
		syncedProductSource: source,
		logger:              logrus.NewEntry(logrus.New()),
	}

	products, err := service.FetchProductsForInventorySync(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("FetchProductsForInventorySync() error = %v", err)
	}
	if len(products) != 105 {
		t.Fatalf("products = %d, want 105", len(products))
	}
	if len(source.queries) != 2 {
		t.Fatalf("queries = %#v, want two pages", source.queries)
	}
	if source.queries[0].Page != 1 || source.queries[1].Page != 2 {
		t.Fatalf("unexpected page queries: %#v", source.queries)
	}
}

func TestUpdateInventoryProductAttributesUsesSyncedProductSource(t *testing.T) {
	source := &fakeSheinSyncedInventoryProductSource{}
	service := &inventorySyncServiceImpl{
		syncedProductSource: source,
		productDataRepo:     &fakeSheinInventoryProductRepo{},
		logger:              logrus.NewEntry(logrus.New()),
	}
	prod := &InventoryProductSnapshot{
		Source:            inventoryProductSourceSheinSyncedProduct,
		TenantID:          1,
		StoreID:           2,
		Platform:          "SHEIN",
		PlatformProductID: "skc-1",
	}

	count, err := service.updateInventoryProductAttributes(context.Background(), prod, `{"updated":true}`)
	if err != nil {
		t.Fatalf("updateInventoryProductAttributes() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("update count = %d, want 1", count)
	}
	if source.updatedSKCName != "skc-1" || source.updatedAttributes != `{"updated":true}` {
		t.Fatalf("unexpected synced update: %#v", source)
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

type fakeSheinSyncedInventoryProductSource struct {
	records           []SyncedInventoryProductRecord
	pageCap           int
	queries           []SyncedInventoryProductQuery
	updatedSKCName    string
	updatedAttributes string
}

func (f *fakeSheinSyncedInventoryProductSource) ListSyncedInventoryProducts(_ context.Context, query SyncedInventoryProductQuery) ([]SyncedInventoryProductRecord, int64, error) {
	f.queries = append(f.queries, query)
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if f.pageCap > 0 && pageSize > f.pageCap {
		pageSize = f.pageCap
	}
	start := (page - 1) * pageSize
	if start >= len(f.records) {
		return nil, int64(len(f.records)), nil
	}
	end := start + pageSize
	if end > len(f.records) {
		end = len(f.records)
	}
	return f.records[start:end], int64(len(f.records)), nil
}

func (f *fakeSheinSyncedInventoryProductSource) UpdateSyncedInventoryProductAttributes(_ context.Context, _ int64, _ int64, skcName string, attributes string) (int, error) {
	f.updatedSKCName = skcName
	f.updatedAttributes = attributes
	return 1, nil
}
