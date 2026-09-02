package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/core/config"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	worker "task-processor/internal/platform/workerpool"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/tenantbridge"
)

func TestHTTPE2E_CurrentAmazonListingUsesSnapshotAndApprovedAssets(t *testing.T) {
	const productKey = "amazon-current-e2e-1"
	const approvedMainURL = "https://cdn.example.test/approved-amazon-main.png"

	logger := currentE2ELogger()
	cfg := currentE2EConfig(t)
	snapshot := catalog.ProductSnapshot{
		Title:         "Snapshot Bluetooth Earbuds",
		Brand:         "SoundPeak",
		CategoryPath:  []string{"Electronics", "Headphones"},
		Description:   "Bluetooth earbuds with active noise cancellation and long battery life.",
		SellingPoints: []string{"30 hour battery", "Dual microphone noise cancellation"},
		Variants: []catalog.Variant{{
			SKU: "SNAPSHOT-EARBUDS-001", Stock: 20, IsDefault: true,
			Price: &catalog.Price{Currency: "USD", Amount: 49.99, CostPrice: 20},
		}},
	}
	inventory := productasset.ApprovedAssetInventory{
		Scope: productasset.InventoryScope{TenantID: "app-http-test-tenant", ProductKey: productKey},
		Assets: []productasset.ApprovedAsset{{
			ID: "approved-amazon-main", RunID: "amazon-image-run-1", PlanRevision: 1,
			SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: approvedMainURL,
		}},
	}
	db, snapshots, assets := currentE2EProductReaders(t, productKey, snapshot, inventory)
	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{cfg: cfg, productCatalogDB: db},
		features: &featureRuntimeState{listingKitSupport: &listingKitSupport{
			approvedAssetReader: assets,
		}},
	}
	require.NoError(t, initializeProductSnapshotReader(deps))

	module, err := (amazonListingFeatureBuilder{buildAmazonListing: buildAmazonListingModuleResult}).build(logger, deps)
	require.NoError(t, err)
	require.NotNil(t, module)
	t.Cleanup(func() { closeCurrentE2EClosers(t, deps.shared.closers) })

	composition := httpFeatureComposition{amazonListingModule: module}
	bundle, err := composition.buildRuntimeBundle(appHTTPTestConfig)
	require.NoError(t, err)
	requireCurrentE2EPool(t, bundle, "amazon_listing")
	startCurrentE2EPools(t, bundle.pools())

	server, _ := bundle.buildServerBundle(0, appHTTPTestRouteAuthorization)
	httpServer := httptest.NewServer(server.Handler)
	t.Cleanup(httpServer.Close)
	client := authenticatedAppHTTPTestClient(httpServer.Client())

	taskID := createCurrentE2ETask(t, client, httpServer.URL+"/api/v1/amazon/listings/generate", map[string]any{
		"marketplace": "amazon",
		"product_key": productKey,
	})
	task := waitForCurrentE2ETask(t, client, httpServer.URL+"/api/v1/amazon/listings/tasks/"+taskID, amazonTaskTerminal)
	require.NotEqual(t, amazonlisting.TaskStatusFailed, task.Status, task.Error)
	require.NotNil(t, task.Result)
	require.Equal(t, snapshot.Title, task.Result.Title)
	require.Equal(t, approvedMainURL, task.Result.Images.MainImage)
	require.Equal(t, []string{"approved-amazon-main"}, task.Result.Source.ApprovedAssetIDs)

	workbench := getCurrentE2EJSON[amazonlisting.TaskWorkbench](t, client, httpServer.URL+"/api/v1/amazon/listings/tasks/"+taskID+"/workbench")
	require.Equal(t, taskID, workbench.TaskID)
	require.NotNil(t, workbench.ReviewSummary)
	require.NotEmpty(t, workbench.ActionBuckets)

	published, err := snapshots.GetCurrentSnapshot(context.Background(), catalog.SnapshotIdentity{TenantID: "app-http-test-tenant", ProductKey: productKey})
	require.NoError(t, err)
	require.Equal(t, snapshot, published.Snapshot)
}

func TestHTTPE2E_CurrentListingKitUsesReadOnlySnapshotAndApprovedAssets(t *testing.T) {
	const (
		productKey      = "listingkit-current-e2e-1"
		sourceImageURL  = "https://source.example.test/unapproved-source.png"
		approvedMainURL = "https://cdn.example.test/approved-listingkit-main.png"
		sheinTenantID   = int64(227)
		sheinStoreID    = int64(869)
	)

	logger := currentE2ELogger()
	cfg := currentE2EConfig(t)
	originalSnapshot := catalog.ProductSnapshot{
		Title:         "Snapshot Bluetooth Earbuds",
		Brand:         "SoundPeak",
		CategoryPath:  []string{"Electronics", "Headphones"},
		Description:   "Snapshot facts are read-only during ListingKit generation.",
		SellingPoints: []string{"Approved asset only"},
		Images:        []catalog.Image{{URL: sourceImageURL, Role: "primary"}},
		Variants: []catalog.Variant{{
			SKU: "SNAPSHOT-EARBUDS-001", Stock: 20, IsDefault: true,
			Price:  &catalog.Price{Currency: "USD", Amount: 49.99, CostPrice: 20},
			Images: []catalog.Image{{URL: sourceImageURL, Role: "primary"}},
		}},
	}
	inventory := productasset.ApprovedAssetInventory{
		Scope: productasset.InventoryScope{TenantID: "app-http-test-tenant", ProductKey: productKey},
		Assets: []productasset.ApprovedAsset{{
			ID: "approved-listingkit-main", RunID: "listingkit-image-run-1", PlanRevision: 1,
			SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: approvedMainURL,
		}},
	}
	db, snapshots, assets := currentE2EProductReaders(t, productKey, originalSnapshot, inventory)
	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{cfg: cfg, productCatalogDB: db},
		features: &featureRuntimeState{
			listingKitSupport: &listingKitSupport{approvedAssetReader: assets},
		},
	}
	require.NoError(t, initializeProductSnapshotReader(deps))

	storeRepo := &currentE2EListingStoreRepository{store: listingadmin.Store{
		ID: sheinStoreID, TenantID: sheinTenantID, StoreID: "869", Name: "Current E2E SHEIN Store",
		Username: "current-e2e-shein-store", LoginURL: "https://example.test/shein-login",
		ShopType: "marketplace", Region: "US", Platform: "shein", Status: 0,
	}}
	uploadStore, err := listingkit.NewLocalImageUploadStore(t.TempDir())
	require.NoError(t, err)
	builder := newListingKitFeatureBuilder()
	productionBuild := builder.buildListingKit
	builder.buildListingKit = func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error) {
		input.Runtime.Support.Repositories.Admin.Store = func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
			return storeRepo, nil, nil
		}
		input.Runtime.Support.Repositories.Core.ApprovedAsset = func(*config.Config, *logrus.Logger) (listingkit.ApprovedAssetInventoryReader, []func() error, error) {
			return assets, nil, nil
		}
		input.Runtime.Support.Hooks.ImageUploadStoreBuilder = func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore {
			return uploadStore
		}
		return productionBuild(input)
	}
	module, err := builder.build(logger, deps)
	require.NoError(t, err)
	require.NotNil(t, module)
	t.Cleanup(func() { closeCurrentE2EClosers(t, deps.shared.closers) })

	restoreTenantResolver := tenantbridge.ConfigureLegacyTenantResolver(currentE2ELegacyTenantResolver{
		tenantID: "app-http-test-tenant", legacyTenantID: sheinTenantID,
	})
	t.Cleanup(restoreTenantResolver)

	composition := httpFeatureComposition{listingKitModule: module}
	bundle, err := composition.buildRuntimeBundle(appHTTPTestConfig)
	require.NoError(t, err)
	requireCurrentE2EPool(t, bundle, "listing_kit")
	startCurrentE2EPools(t, bundle.pools())

	server, _ := bundle.buildServerBundle(0, appHTTPTestRouteAuthorization)
	httpServer := httptest.NewServer(server.Handler)
	t.Cleanup(httpServer.Close)
	client := authenticatedAppHTTPTestClient(httpServer.Client())
	enableCurrentE2EListingKitSubscription(t, client, httpServer.URL, "studio")

	taskID := createCurrentE2ETask(t, client, httpServer.URL+"/api/v1/listing-kits/generate", map[string]any{
		"product_key": productKey, "platforms": []string{"shein"}, "country": "US", "language": "en",
		"shein_store_id": sheinStoreID,
	})
	task := waitForCurrentE2ETask(t, client, httpServer.URL+"/api/v1/listing-kits/tasks/"+taskID, listingKitTaskTerminal)
	require.NotEqual(t, core.TaskStatusFailed, task.Status, task.Error)
	require.NotNil(t, task.Result)
	require.NotNil(t, task.Result.CatalogProduct)
	require.NotNil(t, task.Result.ApprovedAssetInventory)
	require.NotNil(t, task.Result.CanonicalProduct)
	require.Equal(t, originalSnapshot.Title, task.Result.CatalogProduct.Title)
	require.Len(t, task.Result.CanonicalProduct.Images, 1)
	require.Equal(t, approvedMainURL, task.Result.CanonicalProduct.Images[0].URL)
	require.NotEqual(t, sourceImageURL, task.Result.CanonicalProduct.Images[0].URL)
	require.NotNil(t, task.Result.Shein)
	require.NotNil(t, task.Result.Shein.RequestDraft.ImageInfo)
	require.Equal(t, approvedMainURL, task.Result.Shein.RequestDraft.ImageInfo.MainImage)
	require.Empty(t, task.Result.Shein.RequestDraft.ImageInfo.Source)

	preview := getCurrentE2EJSON[listingkit.ListingKitPreview](t, client, httpServer.URL+"/api/v1/listing-kits/tasks/"+taskID+"/preview?platform=shein")
	require.Equal(t, taskID, preview.TaskID)
	require.Equal(t, "shein", preview.SelectedPlatform)
	require.NotNil(t, preview.Catalog)
	require.NotNil(t, preview.ApprovedAssetInventory)
	require.NotNil(t, preview.Shein)
	require.NotNil(t, preview.Shein.DraftPayload)
	require.Equal(t, approvedMainURL, preview.Shein.DraftPayload.ImageInfo.MainImage)
	require.Empty(t, preview.Shein.DraftPayload.ImageInfo.Source)

	const revisedName = "Revised Snapshot Bluetooth Earbuds"
	revised := postCurrentE2EJSON[listingkit.ListingKitPreview](t, client, httpServer.URL+"/api/v1/listing-kits/tasks/"+taskID+"/revision", map[string]any{
		"platform": "shein", "actor": "current-e2e", "reason": "verify current revision route",
		"shein": map[string]any{"spu_name": revisedName},
	})
	require.NotNil(t, revised.ApplyResult)
	require.True(t, revised.ApplyResult.Applied)
	require.NotNil(t, revised.Shein)
	require.NotNil(t, revised.Shein.DraftPayload)
	require.Equal(t, revisedName, revised.Shein.DraftPayload.SpuName)
	require.Equal(t, approvedMainURL, revised.Shein.DraftPayload.ImageInfo.MainImage)

	history := getCurrentE2EJSON[listingkit.ListingKitRevisionHistoryPage](t, client, httpServer.URL+"/api/v1/listing-kits/tasks/"+taskID+"/revision-history")
	require.Equal(t, taskID, history.TaskID)
	require.NotEmpty(t, history.Items)
	require.Equal(t, listingkit.RevisionActionTypeEdit, history.Items[0].ActionType)
	exported := getCurrentE2EJSON[listingkit.ListingKitExport](t, client, httpServer.URL+"/api/v1/listing-kits/tasks/"+taskID+"/export?platform=shein")
	require.Equal(t, taskID, exported.TaskID)
	require.Equal(t, "shein", exported.SelectedPlatform)
	require.NotNil(t, exported.Shein)
	require.NotNil(t, exported.Shein.DraftPayload)
	require.Equal(t, revisedName, exported.Shein.DraftPayload.SpuName)
	require.Equal(t, approvedMainURL, exported.Shein.DraftPayload.ImageInfo.MainImage)

	published, err := snapshots.GetCurrentSnapshot(context.Background(), catalog.SnapshotIdentity{TenantID: "app-http-test-tenant", ProductKey: productKey})
	require.NoError(t, err)
	require.Equal(t, originalSnapshot, published.Snapshot, "production workflow mutated the persisted Snapshot")
}

func currentE2EProductReaders(t *testing.T, productKey string, snapshot catalog.ProductSnapshot, inventory productasset.ApprovedAssetInventory) (*gorm.DB, catalog.Repository, productasset.Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "current-product-http-e2e.sqlite")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, catalogpersistence.AutoMigrate(db))
	require.NoError(t, assetpersistence.AutoMigrate(db))

	snapshots, err := catalogpersistence.NewRepository(db)
	require.NoError(t, err)
	_, err = snapshots.PublishSnapshot(context.Background(), catalog.PublishRequest{
		Identity:      catalog.SnapshotIdentity{TenantID: "app-http-test-tenant", ProductKey: productKey},
		PublicationID: "current-e2e-snapshot-1",
		Snapshot:      snapshot,
	})
	require.NoError(t, err)

	assets, err := assetpersistence.NewRepository(db)
	require.NoError(t, err)
	_, err = assets.CommitApproval(context.Background(), productasset.ApprovalCommit{
		TenantID: inventory.Scope.TenantID, ProductKey: inventory.Scope.ProductKey,
		ActionID: "current-e2e-approval-1", Assets: inventory.Assets,
	})
	require.NoError(t, err)
	return db, snapshots, assets
}

type currentE2EListingStoreRepository struct {
	listingadmin.StoreRepository
	store listingadmin.Store
}

func (r *currentE2EListingStoreRepository) GetStore(_ context.Context, tenantID, storeID int64) (*listingadmin.Store, error) {
	if tenantID != r.store.TenantID || storeID != r.store.ID {
		return nil, listingadmin.ErrStoreNotFound
	}
	store := r.store
	return &store, nil
}

type currentE2ELegacyTenantResolver struct {
	tenantID       string
	legacyTenantID int64
}

func (r currentE2ELegacyTenantResolver) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	if strings.TrimSpace(tenantID) != r.tenantID {
		return 0, false, nil
	}
	return r.legacyTenantID, true, nil
}

func currentE2ELogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	return logger
}

func currentE2EConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.LoadConfigFromFileWithoutValidation("../../../config/config-test.yaml")
	require.NoError(t, err)
	return cfg
}

func requireCurrentE2EPool(t *testing.T, bundle runtimeBundle, name string) {
	t.Helper()
	for _, item := range bundle.workerPools {
		if item.Name == name && item.Pool != nil {
			return
		}
	}
	t.Fatalf("production worker pool %q is not registered", name)
}

func startCurrentE2EPools(t *testing.T, pools []worker.WorkerPool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	for _, pool := range pools {
		pool.Start(ctx)
	}
	t.Cleanup(func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		for _, pool := range pools {
			pool.Stop(stopCtx)
		}
	})
}

func closeCurrentE2EClosers(t *testing.T, closers []func() error) {
	t.Helper()
	for i := len(closers) - 1; i >= 0; i-- {
		require.NoError(t, closers[i]())
	}
}

func enableCurrentE2EListingKitSubscription(t *testing.T, client *http.Client, baseURL, moduleCode string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"status": "active", "limits": map[string]int{}})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPut, baseURL+"/api/v1/listing-kits/admin/subscription/entitlements/"+moduleCode, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Roles", "platform_admin")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func createCurrentE2ETask(t *testing.T, client *http.Client, url string, payload any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, responseBody(resp))
	var result struct {
		TaskID string `json:"task_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.TaskID)
	return result.TaskID
}

func postCurrentE2EJSON[T any](t *testing.T, client *http.Client, url string, payload any) T {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, responseBody(resp))
	var result T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func waitForCurrentE2ETask[T any](t *testing.T, client *http.Client, url string, terminal func(T) (bool, string)) T {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		result := getCurrentE2EJSON[T](t, client, url)
		done, status := terminal(result)
		if done {
			return result
		}
		if status == "failed" {
			t.Fatalf("task at %s failed", url)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("task at %s did not reach terminal state", url)
	var zero T
	return zero
}

func getCurrentE2EJSON[T any](t *testing.T, client *http.Client, url string) T {
	t.Helper()
	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, responseBody(resp))
	var result T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func responseBody(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	return fmt.Sprintf("HTTP status %d", resp.StatusCode)
}

func listingKitTaskTerminal(result listingkit.TaskResult) (bool, string) {
	switch result.Status {
	case core.TaskStatusCompleted, core.TaskStatusNeedsReview, core.TaskStatusFailed:
		return true, string(result.Status)
	default:
		return false, string(result.Status)
	}
}

func amazonTaskTerminal(result amazonlisting.TaskResult) (bool, string) {
	switch result.Status {
	case amazonlisting.TaskStatusCompleted, amazonlisting.TaskStatusNeedsReview, amazonlisting.TaskStatusRejected, amazonlisting.TaskStatusFailed:
		return true, string(result.Status)
	default:
		return false, string(result.Status)
	}
}
