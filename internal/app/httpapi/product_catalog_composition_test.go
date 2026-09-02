package httpapi

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/amazonlisting"
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productasset "task-processor/internal/product/asset"
	productcatalog "task-processor/internal/product/catalog"
)

func TestAttachProductSnapshotRepositoryUsesTypedDatabaseAndProvidesProductionReader(t *testing.T) {
	db := openHTTPAPICatalogTestDB(t)
	if err := catalogpersistence.AutoMigrate(db); err != nil {
		t.Fatalf("catalog AutoMigrate() error = %v", err)
	}
	repository, err := catalogpersistence.NewRepository(db)
	if err != nil {
		t.Fatalf("catalog NewRepository() error = %v", err)
	}
	_, err = repository.PublishSnapshot(context.Background(), productcatalog.PublishRequest{
		Identity:      productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"},
		PublicationID: "source-run-1", Snapshot: productcatalog.ProductSnapshot{Title: "Production Bottle"},
	})
	if err != nil {
		t.Fatalf("PublishSnapshot() error = %v", err)
	}

	deps := &runtimeDeps{shared: &sharedRuntimeDeps{}, features: &featureRuntimeState{}}
	if err := attachProductSnapshotRepository(deps, db); err != nil {
		t.Fatalf("attachProductSnapshotRepository() error = %v", err)
	}
	got, err := deps.features.productSnapshotReader.GetProductSnapshot(context.Background(), listingkit.ProductSnapshotQuery{
		TenantID: "tenant-a", ProductKey: "product-1",
	})
	if err != nil {
		t.Fatalf("GetProductSnapshot() error = %v", err)
	}
	if got.Title != "Production Bottle" {
		t.Fatalf("GetProductSnapshot().Title = %q", got.Title)
	}
	_, err = deps.features.productSnapshotReader.GetProductSnapshot(context.Background(), listingkit.ProductSnapshotQuery{
		TenantID: "tenant-a", ProductKey: "missing",
	})
	if !errors.Is(err, listingkit.ErrProductSnapshotNotReady) {
		t.Fatalf("missing snapshot error = %v, want ListingKit ErrProductSnapshotNotReady", err)
	}
}

func TestAttachProductSnapshotRepositoryFailsClosedWithoutTypedDatabase(t *testing.T) {
	deps := &runtimeDeps{shared: &sharedRuntimeDeps{}, features: &featureRuntimeState{}}
	err := attachProductSnapshotRepository(deps, nil)
	if !errors.Is(err, productcatalog.ErrRepositoryUnavailable) {
		t.Fatalf("attachProductSnapshotRepository(nil) error = %v, want ErrRepositoryUnavailable", err)
	}
	if deps.features.productSnapshotReader != nil {
		t.Fatalf("reader attached without database: %T", deps.features.productSnapshotReader)
	}
}

func TestInitializeProductSnapshotReaderUsesOnlyTypedRuntimeDatabase(t *testing.T) {
	db := openHTTPAPICatalogTestDB(t)
	if err := catalogpersistence.AutoMigrate(db); err != nil {
		t.Fatalf("catalog AutoMigrate() error = %v", err)
	}
	deps := &runtimeDeps{shared: &sharedRuntimeDeps{productCatalogDB: db}, features: &featureRuntimeState{}}
	if err := initializeProductSnapshotReader(deps); err != nil {
		t.Fatalf("initializeProductSnapshotReader() error = %v", err)
	}
	if deps.features.productSnapshotReader == nil {
		t.Fatal("production reader was not attached from typed database")
	}

	withoutDB := &runtimeDeps{shared: &sharedRuntimeDeps{}, features: &featureRuntimeState{}}
	if err := initializeProductSnapshotReader(withoutDB); err != nil {
		t.Fatalf("initializeProductSnapshotReader(without DB) error = %v", err)
	}
	if withoutDB.features.productSnapshotReader != nil {
		t.Fatalf("reader attached without typed database: %T", withoutDB.features.productSnapshotReader)
	}
}

func TestAmazonListingFeatureBuilderMapsCatalogNotReadyAndRequiresBothReaders(t *testing.T) {
	db := openHTTPAPICatalogTestDB(t)
	if err := catalogpersistence.AutoMigrate(db); err != nil {
		t.Fatalf("catalog AutoMigrate() error = %v", err)
	}
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{},
		features: &featureRuntimeState{listingKitSupport: &listingKitSupport{approvedAssetReader: emptyApprovedAssetReader{}}},
	}
	if err := attachProductSnapshotRepository(deps, db); err != nil {
		t.Fatalf("attachProductSnapshotRepository() error = %v", err)
	}

	called := false
	builder := amazonListingFeatureBuilder{buildAmazonListing: func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error) {
		called = true
		_, readErr := input.ProductSnapshotReader.GetProductSnapshot(context.Background(), amazonlisting.ProductSnapshotQuery{
			TenantID: "tenant-a", ProductKey: "missing",
		})
		if !errors.Is(readErr, amazonlisting.ErrProductSnapshotNotReady) {
			t.Fatalf("Amazon reader missing error = %v, want ErrProductSnapshotNotReady", readErr)
		}
		return &amazonlistinghttpapi.Module{}, nil
	}}
	module, err := builder.build(logrus.New(), deps)
	if err != nil {
		t.Fatalf("builder.build() error = %v", err)
	}
	if module == nil || !called {
		t.Fatalf("builder result = %T, called = %v; want registered module", module, called)
	}

	deps.features.listingKitSupport.approvedAssetReader = nil
	deps.shared = nil
	called = false
	module, err = builder.build(logrus.New(), deps)
	if err != nil {
		t.Fatalf("builder.build(without assets) error = %v", err)
	}
	if module != nil || called {
		t.Fatalf("builder registered without approved assets: module=%T called=%v", module, called)
	}
}

func TestProductionCompositionBuildsAmazonListingFromPublishedSnapshotAndApprovedAssets(t *testing.T) {
	db := openHTTPAPICatalogTestDB(t)
	if err := catalogpersistence.AutoMigrate(db); err != nil {
		t.Fatalf("catalog AutoMigrate() error = %v", err)
	}
	if err := assetpersistence.AutoMigrate(db); err != nil {
		t.Fatalf("asset AutoMigrate() error = %v", err)
	}
	snapshots, err := catalogpersistence.NewRepository(db)
	if err != nil {
		t.Fatalf("catalog NewRepository() error = %v", err)
	}
	assets, err := assetpersistence.NewRepository(db)
	if err != nil {
		t.Fatalf("asset NewRepository() error = %v", err)
	}
	_, err = snapshots.PublishSnapshot(context.Background(), productcatalog.PublishRequest{
		Identity:      productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"},
		PublicationID: "source-run-1",
		Snapshot: productcatalog.ProductSnapshot{
			Title: "Production Bottle", Description: "Insulated steel bottle", SellingPoints: []string{"Leakproof"},
		},
	})
	if err != nil {
		t.Fatalf("PublishSnapshot() error = %v", err)
	}
	_, err = assets.CommitApproval(context.Background(), productasset.ApprovalCommit{
		TenantID: "tenant-a", ProductKey: "product-1", ActionID: "approve-1",
		Assets: []productasset.ApprovedAsset{{
			ID: "asset-main", RunID: "image-run-1", PlanRevision: 1, SlotID: "main", Attempt: 1,
			Role: productasset.RoleMain, URL: "https://cdn.example.test/main.png",
		}},
	})
	if err != nil {
		t.Fatalf("CommitApproval() error = %v", err)
	}

	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{productCatalogDB: db},
		features: &featureRuntimeState{listingKitSupport: &listingKitSupport{approvedAssetReader: assets}},
	}
	if err := initializeProductSnapshotReader(deps); err != nil {
		t.Fatalf("initializeProductSnapshotReader() error = %v", err)
	}
	var artifacts *amazonlisting.WorkflowArtifacts
	builder := amazonListingFeatureBuilder{buildAmazonListing: func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error) {
		workflow := amazonlisting.NewListingWorkflow(input.ProductSnapshotReader, input.ApprovedAssetInventoryReader, amazonlisting.NewAssembler(), nil, nil)
		task := &amazonlisting.Task{ID: "amazon-task-1", Request: &amazonlisting.GenerateRequest{Marketplace: "amazon", ProductKey: "product-1"}}
		task.ExecutionTenantID = "tenant-a"
		artifacts, err = workflow.Run(context.Background(), task)
		return &amazonlistinghttpapi.Module{}, err
	}}
	module, err := builder.build(logrus.New(), deps)
	if err != nil {
		t.Fatalf("builder.build() error = %v", err)
	}
	if module == nil || artifacts == nil || artifacts.Draft == nil {
		t.Fatalf("production build result module=%T artifacts=%+v", module, artifacts)
	}
	if artifacts.Draft.Title != "Production Bottle" || artifacts.Draft.Images.MainImage != "https://cdn.example.test/main.png" {
		t.Fatalf("Amazon draft = %+v", artifacts.Draft)
	}
}

func TestListingKitFeatureBuilderRequiresSnapshotAndApprovedAssetReaders(t *testing.T) {
	for _, test := range []struct {
		name     string
		shared   *sharedRuntimeDeps
		features *featureRuntimeState
	}{
		{
			name: "missing snapshot reader", shared: &sharedRuntimeDeps{},
			features: &featureRuntimeState{listingKitSupport: &listingKitSupport{approvedAssetReader: emptyApprovedAssetReader{}}},
		},
		{
			name: "missing approved asset reader", shared: nil,
			features: &featureRuntimeState{productSnapshotReader: stubCompositionProductSnapshotReader{}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			builder := listingKitFeatureBuilder{buildListingKit: func(listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
				called = true
				return &listingkithttpapi.Module{}, nil
			}}
			module, err := builder.build(logrus.New(), &runtimeDeps{shared: test.shared, features: test.features})
			if err != nil {
				t.Fatalf("builder.build() error = %v", err)
			}
			if called || module != nil {
				t.Fatalf("ListingKit registered with incomplete readers: called=%v module=%T", called, module)
			}
		})
	}
}

type emptyApprovedAssetReader struct{}

func (emptyApprovedAssetReader) GetApprovedInventory(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
}

func openHTTPAPICatalogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "httpapi-catalog.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
