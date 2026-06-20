package sync

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"

	"github.com/sirupsen/logrus"
)

type fakeTemuProductDataRepo struct {
	items []listingadmin.ProductData
}

func (f *fakeTemuProductDataRepo) ListProductData(context.Context, listingadmin.ProductDataQuery) (*listingadmin.ProductDataPage, error) {
	panic("unexpected call")
}
func (f *fakeTemuProductDataRepo) GetProductData(context.Context, int64, int64) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuProductDataRepo) CreateProductData(context.Context, *listingadmin.ProductData) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuProductDataRepo) UpdateProductData(context.Context, *listingadmin.ProductData) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuProductDataRepo) UpdateProductDataStatus(context.Context, int64, int64, int16) (*listingadmin.ProductData, error) {
	panic("unexpected call")
}
func (f *fakeTemuProductDataRepo) DeleteProductData(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (f *fakeTemuProductDataRepo) UpsertProductDataBatch(_ context.Context, items []listingadmin.ProductData) (int, error) {
	f.items = append([]listingadmin.ProductData(nil), items...)
	return len(items), nil
}
func (f *fakeTemuProductDataRepo) BatchUpdateAttributesByPlatformProductID(context.Context, []listingadmin.ProductData) (int, error) {
	panic("unexpected call")
}

func TestSaveProductsPrefersRepository(t *testing.T) {
	repo := &fakeTemuProductDataRepo{}
	service := &productSyncServiceImpl{productDataRepo: repo, logger: logrus.NewEntry(logrus.New())}

	count, err := service.SaveProducts(context.Background(), []*TemuProductSnapshot{{
		TenantID:          1,
		StoreID:           2,
		Platform:          "TEMU",
		ProductID:         "p1",
		Title:             "title",
		OriginalPrice:     types.FlexibleString("10.50"),
		Stock:             types.FlexibleString("3"),
		PlatformProductID: "pp1",
	}})
	if err != nil {
		t.Fatalf("SaveProducts() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("SaveProducts() count = %d, want 1", count)
	}
	if len(repo.items) != 1 || repo.items[0].PlatformProductID != "pp1" {
		t.Fatalf("repo items = %#v", repo.items)
	}
}
