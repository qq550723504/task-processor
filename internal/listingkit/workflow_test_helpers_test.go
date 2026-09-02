package listingkit

import (
	"context"
	"sort"
	"testing"

	"task-processor/internal/listingkit/reviewstore"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/canonical"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/shared/aiidentity"
)

func testCatalogSnapshot(t testing.TB, product *canonical.Product) *catalog.ProductSnapshot {
	t.Helper()
	snapshot, err := catalog.Normalize(product)
	if err != nil {
		t.Fatalf("catalog.Normalize() error = %v", err)
	}
	return snapshot
}

type stubWorkflowProductSnapshotReader struct {
	task       *stubProductSnapshotTask
	product    *stubProductSnapshotFixture
	processErr error
	snapshot   catalog.ProductSnapshot
	err        error
	lastReq    *ProductSnapshotQuery
	calls      []ProductSnapshotQuery
}

type stubWorkflowApprovedAssetReader struct {
	inventory productasset.ApprovedAssetInventory
	err       error
}

func (s *stubWorkflowApprovedAssetReader) GetApprovedInventory(_ context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	if s.err != nil {
		return productasset.ApprovedAssetInventory{}, s.err
	}
	if len(s.inventory.Assets) > 0 {
		inventory := productasset.CloneApprovedAssetInventory(s.inventory)
		inventory.Scope = scope
		return inventory, nil
	}
	return productasset.ApprovedAssetInventory{
		Scope: scope,
		Assets: []productasset.ApprovedAsset{{
			ID: "approved-main", Role: productasset.RoleMain, URL: "https://example.com/approved-main.png",
		}},
	}, nil
}

func (s *stubWorkflowProductSnapshotReader) GetProductSnapshot(_ context.Context, query ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	s.calls = append(s.calls, query)
	s.lastReq = &query
	if s.err != nil {
		return catalog.ProductSnapshot{}, s.err
	}
	if s.processErr != nil {
		return catalog.ProductSnapshot{}, s.processErr
	}
	if s.product != nil {
		return s.product.catalogSnapshot()
	}
	return s.snapshot, nil
}

type stubProductSnapshotRequest struct {
	ImageURLs  []string
	Text       string
	ProductURL string
}

type stubProductSnapshotTask struct {
	ID string
	aiidentity.PersistedExecutionEnvelope
	Request *stubProductSnapshotRequest
}

type stubProductSnapshotFixture struct {
	Title          string
	Category       []string
	Attributes     map[string]string
	Specifications *canonical.ProductSpecs
	Variants       []stubProductVariantFixture
	SellingPoints  []string
	SEOKeywords    []string
	Description    string
	Images         []string
}

type stubProductVariantFixture struct {
	SKU        string
	Attributes map[string]string
	Price      *canonical.PriceInfo
	Stock      int
	Images     []string
	Barcode    string
	IsDefault  bool
}

func (f *stubProductSnapshotFixture) catalogSnapshot() (catalog.ProductSnapshot, error) {
	if f == nil {
		return catalog.ProductSnapshot{}, nil
	}
	product := &canonical.Product{
		Title:          f.Title,
		CategoryPath:   append([]string(nil), f.Category...),
		Attributes:     testCanonicalAttributes(f.Attributes),
		Specifications: f.Specifications,
		SellingPoints:  append([]string(nil), f.SellingPoints...),
		SEOKeywords:    append([]string(nil), f.SEOKeywords...),
		Description:    f.Description,
		Images:         testCanonicalImages(f.Images),
	}
	for _, variant := range f.Variants {
		product.Variants = append(product.Variants, canonical.Variant{
			SKU:        variant.SKU,
			Attributes: testCanonicalAttributes(variant.Attributes),
			Price:      variant.Price,
			Stock:      variant.Stock,
			Images:     testCanonicalImages(variant.Images),
			Barcode:    variant.Barcode,
			IsDefault:  variant.IsDefault,
		})
	}
	snapshot, err := catalog.Normalize(product)
	if err != nil {
		return catalog.ProductSnapshot{}, err
	}
	return *snapshot, nil
}

func testCanonicalAttributes(values map[string]string) map[string]canonical.Attribute {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]canonical.Attribute, len(values))
	for _, key := range keys {
		result[key] = canonical.Attribute{Value: values[key]}
	}
	return result
}

func testCanonicalImages(urls []string) []canonical.Image {
	if len(urls) == 0 {
		return nil
	}
	result := make([]canonical.Image, 0, len(urls))
	for _, url := range urls {
		result = append(result, canonical.Image{URL: url})
	}
	return result
}

type noopTaskSubmitter struct{}

func (noopTaskSubmitter) Submit(string) error { return nil }

type supportDependencySeed struct {
	sdsSyncService            sdsusecase.Service
	sdsBaselineRemoteProvider SDSBaselineRemoteProvider
	uploadedImageRepository   UploadedImageRepository
	assembler                 Assembler
	reviewRepository          reviewstore.Repository
}

type taskDependencySeed struct {
	sdsLoginStatusProvider SDSLoginStatusProvider
}

func seedWorkflowDeps(s *service) *service {
	return s
}

func seedSupportDeps(s *service, deps supportDependencySeed) *service {
	if s == nil {
		return nil
	}
	if s.workflowDeps.productSnapshots == nil {
		s.workflowDeps.productSnapshots = &stubWorkflowProductSnapshotReader{snapshot: catalog.ProductSnapshot{
			Title:    "Test Product",
			Variants: []catalog.Variant{{SKU: "TEST-001", IsDefault: true}},
		}}
	}
	if s.workflowDeps.approvedAssets == nil {
		s.workflowDeps.approvedAssets = &stubWorkflowApprovedAssetReader{}
	}
	if s.supportDeps.sdsSyncService == nil {
		s.supportDeps.sdsSyncService = deps.sdsSyncService
	}
	if s.supportDeps.sdsBaselineRemoteProvider == nil {
		s.supportDeps.sdsBaselineRemoteProvider = deps.sdsBaselineRemoteProvider
	}
	if s.supportDeps.uploadedImageRepository == nil {
		s.supportDeps.uploadedImageRepository = deps.uploadedImageRepository
	}
	if s.supportDeps.assembler == nil {
		s.supportDeps.assembler = deps.assembler
	}
	if s.supportDeps.reviewRepository == nil {
		s.supportDeps.reviewRepository = deps.reviewRepository
	}
	return s
}

func seedTaskDeps(s *service, deps taskDependencySeed) *service {
	if s == nil {
		return nil
	}
	if s.taskDeps.sdsLoginStatusProvider == nil {
		s.taskDeps.sdsLoginStatusProvider = deps.sdsLoginStatusProvider
	}
	return s
}

func seedWorkflowServices(s *service, productSnapshots ProductSnapshotReader) *service {
	if s == nil {
		return nil
	}
	if productSnapshots != nil {
		s.workflowDeps.productSnapshots = productSnapshots
	}
	return s
}
