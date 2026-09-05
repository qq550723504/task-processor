package catalogpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	productcatalog "task-processor/internal/product/catalog"
)

func TestRepositoryBoundsEncodedSnapshotBeforeTransactionAndMaterialization(t *testing.T) {
	t.Run("publication", func(t *testing.T) {
		db := openCatalogRepositoryTestDB(t).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
		if err := AutoMigrate(db); err != nil {
			t.Fatalf("AutoMigrate(): %v", err)
		}
		repository, err := NewRepository(db)
		if err != nil {
			t.Fatalf("NewRepository(): %v", err)
		}
		request := productcatalog.PublishRequest{
			Identity:      productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-a"},
			PublicationID: "publication-1",
			Snapshot:      persistenceSnapshotWithEncodedSize(t, productcatalog.MaxEncodedSnapshotBytes+1),
		}
		if _, err := repository.PublishSnapshot(context.Background(), request); !errors.Is(err, productcatalog.ErrInvalidSnapshot) {
			t.Fatalf("PublishSnapshot(over limit) error = %v", err)
		}
		var versions, heads int64
		if err := db.Model(&SnapshotVersionRecord{}).Count(&versions).Error; err != nil {
			t.Fatalf("count versions: %v", err)
		}
		if err := db.Model(&SnapshotHeadRecord{}).Count(&heads).Error; err != nil {
			t.Fatalf("count heads: %v", err)
		}
		if versions != 0 || heads != 0 {
			t.Fatalf("oversized publication changed storage: versions=%d heads=%d", versions, heads)
		}
	})

	for _, size := range []int{productcatalog.MaxEncodedSnapshotBytes, productcatalog.MaxEncodedSnapshotBytes + 1} {
		t.Run(fmt.Sprintf("read_%d", size), func(t *testing.T) {
			db := openCatalogRepositoryTestDB(t).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
			if err := AutoMigrate(db); err != nil {
				t.Fatalf("AutoMigrate(): %v", err)
			}
			repository, err := NewRepository(db)
			if err != nil {
				t.Fatalf("NewRepository(): %v", err)
			}
			identity := productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-a"}
			payload := persistenceEncodedSnapshot(t, size)
			sum := sha256.Sum256(payload)
			record := SnapshotVersionRecord{TenantID: identity.TenantID, ProductKey: identity.ProductKey, Version: 1, PublicationID: "publication-1", PayloadHash: hex.EncodeToString(sum[:]), SnapshotJSON: payload}
			if err := db.Create(&record).Error; err != nil {
				t.Fatalf("create version: %v", err)
			}
			if err := db.Create(&SnapshotHeadRecord{TenantID: identity.TenantID, ProductKey: identity.ProductKey, CurrentVersion: 1, PublicationID: "publication-1"}).Error; err != nil {
				t.Fatalf("create head: %v", err)
			}

			versioned := repository.(productcatalog.VersionedSnapshotReader)
			_, versionErr := versioned.GetSnapshot(context.Background(), identity, 1)
			_, currentErr := repository.GetCurrentSnapshot(context.Background(), identity)
			if size == productcatalog.MaxEncodedSnapshotBytes {
				if versionErr != nil || currentErr != nil {
					t.Fatalf("exact-limit read errors: versioned=%v current=%v", versionErr, currentErr)
				}
				return
			}
			if !errors.Is(versionErr, productcatalog.ErrRepositoryStateInvalid) || !errors.Is(currentErr, productcatalog.ErrRepositoryStateInvalid) {
				t.Fatalf("over-limit read errors: versioned=%v current=%v", versionErr, currentErr)
			}
		})
	}
}

func persistenceSnapshotWithEncodedSize(t *testing.T, size int) productcatalog.ProductSnapshot {
	t.Helper()
	return productcatalog.ProductSnapshot{Description: strings.Repeat("x", size-len(`{"description":""}`))}
}

func persistenceEncodedSnapshot(t *testing.T, size int) []byte {
	t.Helper()
	snapshot := persistenceSnapshotWithEncodedSize(t, size)
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if len(payload) != size {
		t.Fatalf("encoded size = %d, want %d", len(payload), size)
	}
	return payload
}

func TestRepositoryPublishesImmutableVersionsWithTenantProductIsolationAndIdempotency(t *testing.T) {
	db := openCatalogRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	repository, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	ctx := context.Background()
	firstRequest := productcatalog.PublishRequest{
		Identity:      productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"},
		PublicationID: "source-run-1", Snapshot: productcatalog.ProductSnapshot{Title: "Bottle v1"},
	}
	first, err := repository.PublishSnapshot(ctx, firstRequest)
	if err != nil {
		t.Fatalf("PublishSnapshot(first) error = %v", err)
	}
	replay, err := repository.PublishSnapshot(ctx, firstRequest)
	if err != nil {
		t.Fatalf("PublishSnapshot(replay) error = %v", err)
	}
	if !reflect.DeepEqual(replay, first) || first.Version != 1 {
		t.Fatalf("idempotent replay = %+v, first = %+v", replay, first)
	}

	conflict := firstRequest
	conflict.Snapshot.Title = "different payload"
	if _, err := repository.PublishSnapshot(ctx, conflict); !errors.Is(err, productcatalog.ErrPublicationConflict) {
		t.Fatalf("PublishSnapshot(conflict) error = %v, want ErrPublicationConflict", err)
	}

	second, err := repository.PublishSnapshot(ctx, productcatalog.PublishRequest{
		Identity: firstRequest.Identity, PublicationID: "source-run-2", Snapshot: productcatalog.ProductSnapshot{Title: "Bottle v2"},
	})
	if err != nil {
		t.Fatalf("PublishSnapshot(second) error = %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("second.Version = %d, want 2", second.Version)
	}
	versioned, ok := repository.(productcatalog.VersionedSnapshotReader)
	if !ok {
		t.Fatal("repository does not implement VersionedSnapshotReader")
	}
	historical, err := versioned.GetSnapshot(ctx, firstRequest.Identity, first.Version)
	if err != nil {
		t.Fatalf("GetSnapshot(historical) error = %v", err)
	}
	if historical.Version != first.Version || historical.Snapshot.Title != "Bottle v1" {
		t.Fatalf("GetSnapshot(historical) = %+v, want immutable version 1", historical)
	}

	for _, request := range []productcatalog.PublishRequest{
		{Identity: productcatalog.SnapshotIdentity{TenantID: "tenant-b", ProductKey: "product-1"}, PublicationID: "source-run-1", Snapshot: productcatalog.ProductSnapshot{Title: "Tenant B"}},
		{Identity: productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-2"}, PublicationID: "source-run-1", Snapshot: productcatalog.ProductSnapshot{Title: "Product 2"}},
	} {
		published, publishErr := repository.PublishSnapshot(ctx, request)
		if publishErr != nil {
			t.Fatalf("PublishSnapshot(%+v) error = %v", request.Identity, publishErr)
		}
		if published.Version != 1 {
			t.Fatalf("isolated version for %+v = %d, want 1", request.Identity, published.Version)
		}
	}

	current, err := repository.GetCurrentSnapshot(ctx, firstRequest.Identity)
	if err != nil {
		t.Fatalf("GetCurrentSnapshot() error = %v", err)
	}
	if current.Version != 2 || current.PublicationID != "source-run-2" || current.Snapshot.Title != "Bottle v2" {
		t.Fatalf("GetCurrentSnapshot() = %+v, want current version 2", current)
	}
	current.Snapshot.Title = "mutated"
	again, err := repository.GetCurrentSnapshot(ctx, firstRequest.Identity)
	if err != nil {
		t.Fatalf("GetCurrentSnapshot(again) error = %v", err)
	}
	if again.Snapshot.Title != "Bottle v2" {
		t.Fatalf("persisted snapshot was mutated through returned value: %+v", again)
	}
}

func TestRepositoryReturnsStableNotReadyError(t *testing.T) {
	db := openCatalogRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	repository, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}

	_, err = repository.GetCurrentSnapshot(context.Background(), productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "missing"})
	if !errors.Is(err, productcatalog.ErrSnapshotNotReady) {
		t.Fatalf("GetCurrentSnapshot() error = %v, want ErrSnapshotNotReady", err)
	}
}

func TestRepositorySerializesConcurrentPublicationsForOneProduct(t *testing.T) {
	db := openCatalogRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	repository, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	identity := productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"}
	start := make(chan struct{})
	results := make(chan productcatalog.PublishedSnapshot, 2)
	errorsCh := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for index := 1; index <= 2; index++ {
		index := index
		go func() {
			ready.Done()
			<-start
			published, publishErr := repository.PublishSnapshot(context.Background(), productcatalog.PublishRequest{
				Identity: identity, PublicationID: fmt.Sprintf("source-run-%d", index),
				Snapshot: productcatalog.ProductSnapshot{Title: fmt.Sprintf("v%d", index)},
			})
			results <- published
			errorsCh <- publishErr
		}()
	}
	ready.Wait()
	close(start)
	versions := make([]int, 0, 2)
	for range 2 {
		if publishErr := <-errorsCh; publishErr != nil {
			t.Fatalf("concurrent PublishSnapshot() error = %v", publishErr)
		}
		versions = append(versions, int((<-results).Version))
	}
	sort.Ints(versions)
	if !reflect.DeepEqual(versions, []int{1, 2}) {
		t.Fatalf("concurrent versions = %v, want [1 2]", versions)
	}
}

func TestRepositoryRollsBackVersionWhenHeadAdvanceFails(t *testing.T) {
	db := openCatalogRepositoryTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	repository, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	ctx := context.Background()
	identity := productcatalog.SnapshotIdentity{TenantID: "tenant-a", ProductKey: "product-1"}
	first, err := repository.PublishSnapshot(ctx, productcatalog.PublishRequest{Identity: identity, PublicationID: "source-run-1", Snapshot: productcatalog.ProductSnapshot{Title: "v1"}})
	if err != nil || first.Version != 1 {
		t.Fatalf("publish first = (%+v, %v)", first, err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_snapshot_head_update BEFORE UPDATE ON product_snapshot_heads BEGIN SELECT RAISE(ABORT, 'head update rejected'); END`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if _, err := repository.PublishSnapshot(ctx, productcatalog.PublishRequest{Identity: identity, PublicationID: "source-run-2", Snapshot: productcatalog.ProductSnapshot{Title: "v2"}}); err == nil {
		t.Fatal("PublishSnapshot(second) error = nil, want transaction failure")
	}
	current, err := repository.GetCurrentSnapshot(ctx, identity)
	if err != nil {
		t.Fatalf("GetCurrentSnapshot() error = %v", err)
	}
	if current.Version != 1 || current.Snapshot.Title != "v1" {
		t.Fatalf("current after failed publish = %+v, want unchanged v1", current)
	}
	var count int64
	if err := db.Table("product_snapshot_versions").Where("tenant_id = ? AND product_key = ? AND publication_id = ?", "tenant-a", "product-1", "source-run-2").Count(&count).Error; err != nil {
		t.Fatalf("count rolled back version: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back version count = %d, want 0", count)
	}
}

func TestRepositoryRejectsNilDatabase(t *testing.T) {
	repository, err := NewRepository(nil)
	if repository != nil || !errors.Is(err, productcatalog.ErrRepositoryUnavailable) {
		t.Fatalf("NewRepository(nil) = (%T, %v), want nil ErrRepositoryUnavailable", repository, err)
	}
	if err := AutoMigrate(nil); !errors.Is(err, productcatalog.ErrRepositoryUnavailable) {
		t.Fatalf("AutoMigrate(nil) error = %v, want ErrRepositoryUnavailable", err)
	}
}

func openCatalogRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "catalog.sqlite")), &gorm.Config{})
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
