//go:build integration

package assetpersistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	productasset "task-processor/internal/product/asset"
)

func TestPostgresApprovedInventoryExactVersionReadContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, sqlDB := openApprovedAssetPostgres(t, ctx)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("exact hit preserves organization product platform and version", func(t *testing.T) {
		commit := repositoryTestCommit("org-exact", "product-exact", "approve-v7", "asset-v7")
		commit.TargetPlatform = "shein"
		commit.SourceSnapshotVersion = 7
		if _, err := repository.CommitApproval(ctx, commit); err != nil {
			t.Fatal(err)
		}

		scope := productasset.InventoryScope{
			TenantID:              "org-exact",
			ProductKey:            "product-exact",
			TargetPlatform:        "shein",
			SourceSnapshotVersion: 7,
		}
		inventory, err := repository.GetApprovedInventory(ctx, scope)
		if err != nil {
			t.Fatal(err)
		}
		if inventory.Scope != scope || len(inventory.Assets) != 1 || inventory.Assets[0].ID != "asset-v7" {
			t.Fatalf("exact inventory = %+v, want asset-v7 in scope %+v", inventory, scope)
		}
	})

	t.Run("exact miss does not use unversioned head", func(t *testing.T) {
		commit := repositoryTestCommit("org-miss", "product-miss", "approve-unversioned", "asset-unversioned")
		commit.TargetPlatform = "shein"
		if _, err := repository.CommitApproval(ctx, commit); err != nil {
			t.Fatal(err)
		}

		_, err := repository.GetApprovedInventory(ctx, productasset.InventoryScope{
			TenantID:              "org-miss",
			ProductKey:            "product-miss",
			TargetPlatform:        "shein",
			SourceSnapshotVersion: 7,
		})
		if !errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
			t.Fatalf("exact miss error = %v, want ErrApprovedAssetsNotReady", err)
		}
	})

	t.Run("same product platform and version in another organization is excluded", func(t *testing.T) {
		commit := repositoryTestCommit("org-owner", "product-shared", "approve-owner-v7", "asset-owner-v7")
		commit.TargetPlatform = "shein"
		commit.SourceSnapshotVersion = 7
		if _, err := repository.CommitApproval(ctx, commit); err != nil {
			t.Fatal(err)
		}

		_, err := repository.GetApprovedInventory(ctx, productasset.InventoryScope{
			TenantID:              "org-other",
			ProductKey:            "product-shared",
			TargetPlatform:        "shein",
			SourceSnapshotVersion: 7,
		})
		if !errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
			t.Fatalf("cross-organization exact read error = %v, want ErrApprovedAssetsNotReady", err)
		}
	})

	for _, test := range []struct {
		name    string
		key     string
		assetID string
		payload []byte
	}{
		{name: "payload identity mismatch", key: "identity-mismatch", assetID: "asset-identity-mismatch", payload: []byte(`{"id":"other-asset","run_id":"run-1","plan_revision":2,"slot_id":"main","attempt":1,"role":"main","url":"https://cdn.example/asset-identity-mismatch.png","width":1200,"height":1200,"operations":["approve"]}`)},
		{name: "invalid persisted asset", key: "invalid-domain", assetID: "asset-invalid-domain", payload: []byte(`{"id":"asset-invalid-domain","run_id":"run-1","plan_revision":2,"slot_id":"main","attempt":1,"role":"not-a-role","url":"https://cdn.example/asset-invalid-domain.png","width":1200,"height":1200,"operations":["approve"]}`)},
	} {
		t.Run(test.name+" fails closed without an unversioned fallback", func(t *testing.T) {
			productKey := "product-corrupt-" + test.key
			versioned := repositoryTestCommit("org-corrupt", productKey, "approve-v7-"+test.key, test.assetID)
			versioned.TargetPlatform = "shein"
			versioned.SourceSnapshotVersion = 7
			if _, err := repository.CommitApproval(ctx, versioned); err != nil {
				t.Fatal(err)
			}
			unversioned := repositoryTestCommit("org-corrupt", productKey, "approve-current-"+test.key, "asset-current-"+test.key)
			unversioned.TargetPlatform = "shein"
			if _, err := repository.CommitApproval(ctx, unversioned); err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&ApprovedAssetRecord{}).
				Where("tenant_id = ? AND product_key = ? AND target_platform = ? AND source_snapshot_version = ?", "org-corrupt", productKey, "shein", 7).
				Update("payload_json", test.payload).Error; err != nil {
				t.Fatal(err)
			}

			_, err := repository.GetApprovedInventory(ctx, productasset.InventoryScope{
				TenantID:              "org-corrupt",
				ProductKey:            productKey,
				TargetPlatform:        "shein",
				SourceSnapshotVersion: 7,
			})
			if !errors.Is(err, productasset.ErrRepositoryStateInvalid) || errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
				t.Fatalf("corrupt exact read error = %v, want only ErrRepositoryStateInvalid", err)
			}
		})
	}

	t.Run("dangling exact head fails closed without an unversioned fallback", func(t *testing.T) {
		unversioned := repositoryTestCommit("org-dangling", "product-dangling", "approve-current", "asset-current")
		unversioned.TargetPlatform = "shein"
		if _, err := repository.CommitApproval(ctx, unversioned); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&ApprovedInventoryVersionHeadRecord{
			TenantID:              "org-dangling",
			ProductKey:            "product-dangling",
			TargetPlatform:        "shein",
			SourceSnapshotVersion: 7,
			ActionID:              "missing-action",
		}).Error; err != nil {
			t.Fatal(err)
		}

		_, err := repository.GetApprovedInventory(ctx, productasset.InventoryScope{
			TenantID:              "org-dangling",
			ProductKey:            "product-dangling",
			TargetPlatform:        "shein",
			SourceSnapshotVersion: 7,
		})
		if !errors.Is(err, productasset.ErrRepositoryStateInvalid) || errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
			t.Fatalf("dangling exact head error = %v, want only ErrRepositoryStateInvalid", err)
		}
	})

	t.Run("in flight database read obeys context cancellation", func(t *testing.T) {
		commit := repositoryTestCommit("org-cancel", "product-cancel", "approve-v7", "asset-v7-cancel")
		commit.TargetPlatform = "shein"
		commit.SourceSnapshotVersion = 7
		if _, err := repository.CommitApproval(ctx, commit); err != nil {
			t.Fatal(err)
		}

		locker, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = locker.Rollback() }()
		if _, err := locker.ExecContext(ctx, `LOCK TABLE product_approved_inventory_version_heads IN ACCESS EXCLUSIVE MODE`); err != nil {
			t.Fatal(err)
		}

		readCtx, cancelRead := context.WithTimeout(ctx, 100*time.Millisecond)
		started := time.Now()
		_, readErr := repository.GetApprovedInventory(readCtx, productasset.InventoryScope{
			TenantID:              "org-cancel",
			ProductKey:            "product-cancel",
			TargetPlatform:        "shein",
			SourceSnapshotVersion: 7,
		})
		cancelRead()
		if !errors.Is(readErr, context.DeadlineExceeded) || errors.Is(readErr, productasset.ErrRepositoryUnavailable) {
			t.Fatalf("blocked read error = %v, want only context.DeadlineExceeded", readErr)
		}
		if elapsed := time.Since(started); elapsed >= 2*time.Second {
			t.Fatalf("blocked read cancellation took %s, want under 2s", elapsed)
		}
	})
}

func openApprovedAssetPostgres(t *testing.T, ctx context.Context) (*gorm.DB, *sql.DB) {
	t.Helper()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("approvedassets"),
		tcpostgres.WithUsername("approvedassets"),
		tcpostgres.WithPassword("approvedassets"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, sqlDB
}

func init() {
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		_ = os.Setenv("DOCKER_HOST", "npipe:////./pipe/dockerDesktopLinuxEngine")
	}
}
