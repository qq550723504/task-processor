package storecenter_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"task-processor/internal/storecenter"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Mutation caught: removing a required column, a composite index, or the UUID
// primary key would make the durable Organization boundary unenforceable.
func TestGormStoreRepositoryMigratesRepeatableScopedSchema(t *testing.T) {
	db := openStoreDB(t)
	if err := storecenter.AutoMigrateStoreRepository(nil); err == nil {
		t.Fatal("AutoMigrateStoreRepository(nil) error = nil, want nil-safe error")
	}
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatalf("AutoMigrateStoreRepository() error = %v", err)
	}
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatalf("repeat AutoMigrateStoreRepository() error = %v", err)
	}

	type column struct {
		Name    string `gorm:"column:name"`
		Type    string `gorm:"column:type"`
		NotNull int    `gorm:"column:notnull"`
		PK      int    `gorm:"column:pk"`
	}
	var columns []column
	if err := db.Raw("PRAGMA table_info(workbench_stores)").Scan(&columns).Error; err != nil {
		t.Fatalf("table_info error = %v", err)
	}
	byName := map[string]column{}
	for _, column := range columns {
		byName[column.Name] = column
	}
	for _, name := range []string{"id", "organization_id", "version", "lifecycle_status", "quota_allocation_id", "created_by", "updated_by", "created_at", "updated_at", "create_idempotency_key", "delete_operation_key", "identity_key", "create_request_fingerprint"} {
		column, ok := byName[name]
		if !ok || column.NotNull == 0 {
			t.Fatalf("required column %q = %#v, want present and NOT NULL", name, column)
		}
	}
	if got := byName["id"].PK; got != 1 {
		t.Fatalf("id primary-key position = %d, want 1", got)
	}
	if got := strings.ToLower(byName["delete_operation_key"].Type); got != "varchar(36)" {
		t.Fatalf("delete_operation_key type = %q, want variable-width varchar(36)", got)
	}

	assertSQLiteIndex(t, db, "workbench_stores", []string{"organization_id", "lifecycle_status", "updated_at"}, false)
	assertSQLiteIndex(t, db, "workbench_stores", []string{"organization_id", "platform", "region"}, false)
	assertSQLiteIndex(t, db, "workbench_stores", []string{"organization_id", "create_idempotency_key"}, true)
	assertSQLiteIndex(t, db, "workbench_stores", []string{"organization_id", "identity_key"}, true)
}

// Mutation caught: returning the caller's pre-insert aggregate lets database
// timestamp canonicalization make the first lifecycle Save look like an
// immutable-field forgery.
func TestGormStoreRepositoryCreateReturnsDurableTimestampForImmediateSave(t *testing.T) {
	db := openStoreDB(t)
	canonical := time.Date(2026, 8, 31, 8, 0, 0, 123456000, time.UTC)
	if err := db.Exec(`CREATE TRIGGER canonicalize_store_timestamps
		AFTER INSERT ON workbench_stores
		BEGIN
			UPDATE workbench_stores
			SET created_at = '2026-08-31 08:00:00.123456+00:00',
				updated_at = '2026-08-31 08:00:00.123456+00:00'
			WHERE id = NEW.id;
		END`).Error; err != nil {
		t.Fatalf("create timestamp canonicalization trigger: %v", err)
	}
	repo, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	incoming := canonical.Add(789 * time.Nanosecond)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-00000000010f", "00000000-0000-4000-8000-00000000020f", "00000000-0000-4000-8000-00000000030f", "North", "SG", "external-canonical", incoming)

	created, replayed, err := repo.CreateOrReplay(context.Background(), "org-a", store)
	if err != nil || replayed {
		t.Fatalf("CreateOrReplay = (%v, %t, %v), want durable new Store", created, replayed, err)
	}
	if got := created.CreatedAt(); !got.Equal(canonical) {
		t.Errorf("returned CreatedAt = %s, want durable %s", got.Format(time.RFC3339Nano), canonical.Format(time.RFC3339Nano))
	}
	if err := created.TransitionTo(storecenter.StoreStatusActive, "subject-active", created.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", created, 1); err != nil {
		t.Errorf("immediate provisioning-to-active Save error = %v", err)
	}
}

// Mutation caught: treating all same-key requests as a replay would silently
// attach a different Store or allocation to a completed request.
func TestGormStoreRepositoryCreateReplaysOnlyTheImmutableCreationRequest(t *testing.T) {
	repo := newStoreRepository(t)
	first := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000101", "00000000-0000-4000-8000-000000000201", "00000000-0000-4000-8000-000000000301", "North", "SG", "external-1", time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC))
	stored, replayed, err := repo.CreateOrReplay(context.Background(), "org-a", first)
	if err != nil || replayed {
		t.Fatalf("first CreateOrReplay = (%v, %t, %v), want stored/new", stored, replayed, err)
	}

	replayedStore, replayed, err := repo.CreateOrReplay(context.Background(), "org-a", first)
	if err != nil || !replayed || replayedStore.ID() != first.ID() {
		t.Fatalf("exact retry = (%v, %t, %v), want original replay", replayedStore, replayed, err)
	}
	pristineRetry, err := storecenter.RehydrateStore(first.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.TransitionTo(storecenter.StoreStatusActive, "subject-update", first.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", first, 1); err != nil {
		t.Fatal(err)
	}

	changed := first.Snapshot()
	changed.Name, changed.Region, changed.Version = "Renamed", "MY", changed.Version+1
	changed.UpdatedBy, changed.UpdatedAt = "subject-update", changed.UpdatedAt.Add(time.Minute)
	edited, err := storecenter.RehydrateStore(changed)
	if err != nil {
		t.Fatalf("RehydrateStore(edit) error = %v", err)
	}
	if err := repo.Save(context.Background(), "org-a", edited, 2); err != nil {
		t.Fatalf("Save(edit) error = %v", err)
	}

	replayedStore, replayed, err = repo.CreateOrReplay(context.Background(), "org-a", pristineRetry)
	if err != nil || !replayed || replayedStore.Name() != "Renamed" || replayedStore.Region() != "MY" {
		t.Fatalf("retry after legitimate edit = (%v, %t, %v), want current durable replay", replayedStore, replayed, err)
	}

	different := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000102", first.CreateIdempotencyKey(), "00000000-0000-4000-8000-000000000302", "Other", "SG", "external-2", first.CreatedAt())
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", different); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("different same-key request error = %v, want ErrAlreadyExists", err)
	}
	if got, err := repo.Get(context.Background(), "org-a", first.ID()); err != nil || got.Name() != "Renamed" {
		t.Fatalf("conflict mutated existing row: Get = (%v, %v)", got, err)
	}
}

func TestGormStoreRepositoryCreateFingerprintIgnoresMutableAndDeleteFields(t *testing.T) {
	repo := newStoreRepository(t)
	original := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000118", "00000000-0000-4000-8000-000000000218", "00000000-0000-4000-8000-000000000318", "Original", "SG", "external-18", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", original); err != nil {
		t.Fatal(err)
	}
	if err := original.TransitionTo(storecenter.StoreStatusActive, "operator", original.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", original, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := original.EditBasic("Renamed", "MY", "editor", original.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", original, 2); err != nil {
		t.Fatal(err)
	}
	deleteKey := "00000000-0000-4000-8000-000000000918"
	if err := original.BeginDelete(deleteKey, "admin", original.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", original, 3); err != nil {
		t.Fatal(err)
	}
	pristine := newPersistenceStore(t, "org-a", original.ID(), original.CreateIdempotencyKey(), original.QuotaAllocationID(), "Original", "SG", "external-18", original.CreatedAt())
	replayed, existing, err := repo.CreateOrReplay(context.Background(), "org-a", pristine)
	if err != nil || !existing || replayed.LifecycleStatus() != storecenter.StoreStatusDeleting || replayed.DeleteOperationKey() != deleteKey || replayed.Name() != "Renamed" {
		t.Fatalf("CreateOrReplay after edit/delete = %#v, %v, %v", replayed, existing, err)
	}
}

// Mutation caught: accepting a rehydrated lifecycle, connection, or audit
// change at the creation boundary would let callers bypass NewStore's pristine
// provisioning state before the first durable insert or replay decision.
func TestGormStoreRepositoryRejectsNonPristineCreateSnapshots(t *testing.T) {
	repo := newStoreRepository(t)
	for index, test := range []struct {
		name   string
		mutate func(*storecenter.StoreSnapshot)
	}{
		{"active lifecycle", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.LifecycleStatus, snapshot.Version, snapshot.UpdatedBy, snapshot.UpdatedAt = storecenter.StoreStatusActive, 2, "subject-update", snapshot.UpdatedAt.Add(time.Minute)
		}},
		{"disabled lifecycle", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.LifecycleStatus, snapshot.Version, snapshot.UpdatedBy, snapshot.UpdatedAt = storecenter.StoreStatusDisabled, 3, "subject-update", snapshot.UpdatedAt.Add(2*time.Minute)
		}},
		{"deleting lifecycle", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.LifecycleStatus, snapshot.Version, snapshot.UpdatedBy, snapshot.UpdatedAt = storecenter.StoreStatusDeleting, 3, "subject-update", snapshot.UpdatedAt.Add(2*time.Minute)
			snapshot.DeleteOperationKey = "00000000-0000-4000-8000-000000000999"
		}},
		{"deleted state", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.LifecycleStatus, snapshot.Version, snapshot.UpdatedBy, snapshot.UpdatedAt = storecenter.StoreStatusDeleting, 3, "subject-update", snapshot.UpdatedAt.Add(2*time.Minute)
			snapshot.DeleteOperationKey = "00000000-0000-4000-8000-000000000998"
			deletedAt := snapshot.UpdatedAt.Add(time.Minute)
			snapshot.DeletedAt = &deletedAt
		}},
		{"connection reference", func(snapshot *storecenter.StoreSnapshot) { snapshot.ConnectionRef = "opaque-connection-ref" }},
		{"audit actor", func(snapshot *storecenter.StoreSnapshot) { snapshot.UpdatedBy = "subject-update" }},
		{"audit timestamp", func(snapshot *storecenter.StoreSnapshot) { snapshot.UpdatedAt = snapshot.UpdatedAt.Add(time.Minute) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newPersistenceStore(t, "org-a", fmt.Sprintf("00000000-0000-4000-8000-%012d", 160+index), fmt.Sprintf("00000000-0000-4000-8000-%012d", 260+index), fmt.Sprintf("00000000-0000-4000-8000-%012d", 360+index), "North", "SG", fmt.Sprintf("external-%d", index), testPersistenceTime)
			snapshot := store.Snapshot()
			test.mutate(&snapshot)
			crafted, err := storecenter.RehydrateStore(snapshot)
			if err != nil {
				t.Fatalf("RehydrateStore(crafted) error = %v", err)
			}
			if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", crafted); err == nil {
				t.Fatal("CreateOrReplay(crafted) error = nil, want pristine-state rejection")
			}
			if _, err := repo.Get(context.Background(), "org-a", snapshot.ID); !errors.Is(err, storecenter.ErrNotFound) {
				t.Fatalf("crafted create mutated row: Get error = %v, want ErrNotFound", err)
			}
		})
	}
}

// Mutation caught: omitting Organization ID from either unique key would make
// unrelated enterprises collide on a client operation or marketplace identity.
func TestGormStoreRepositoryCreatesOrganizationsIndependentlyAndRejectsLocalIdentityDuplicates(t *testing.T) {
	repo := newStoreRepository(t)
	first := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000111", "00000000-0000-4000-8000-000000000211", "00000000-0000-4000-8000-000000000311", "North", "SG", "ExactOpaqueID", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", first); err != nil {
		t.Fatal(err)
	}
	otherOrg := newPersistenceStore(t, "org-b", "00000000-0000-4000-8000-000000000112", first.CreateIdempotencyKey(), "00000000-0000-4000-8000-000000000312", "North", "SG", "ExactOpaqueID", testPersistenceTime)
	if _, replayed, err := repo.CreateOrReplay(context.Background(), "org-b", otherOrg); err != nil || replayed {
		t.Fatalf("other Organization create = (%t, %v), want new", replayed, err)
	}

	duplicateIdentity := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000113", "00000000-0000-4000-8000-000000000213", "00000000-0000-4000-8000-000000000313", "Different name", "SG", "ExactOpaqueID", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", duplicateIdentity); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("duplicate identity error = %v, want ErrAlreadyExists", err)
	}

	blankOne := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000114", "00000000-0000-4000-8000-000000000214", "00000000-0000-4000-8000-000000000314", "No external", "SG", "", testPersistenceTime)
	blankTwo := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000115", "00000000-0000-4000-8000-000000000215", "00000000-0000-4000-8000-000000000315", "No external", "SG", "", testPersistenceTime)
	for _, store := range []*storecenter.Store{blankOne, blankTwo} {
		if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
			t.Fatalf("blank external identity create error = %v", err)
		}
	}
}

// Mutation caught: unscoped reads/counts or unstable ordering would expose
// another Organization's data or make pagination inconsistent.
func TestGormStoreRepositoryListsOnlyMatchingOrganizationFiltersAndPage(t *testing.T) {
	repo := newStoreRepository(t)
	base := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	for _, in := range []struct {
		id, key, allocation, org, external string
		at                                 time.Time
	}{
		{"00000000-0000-4000-8000-000000000121", "00000000-0000-4000-8000-000000000221", "00000000-0000-4000-8000-000000000321", "org-a", "a", base},
		{"00000000-0000-4000-8000-000000000122", "00000000-0000-4000-8000-000000000222", "00000000-0000-4000-8000-000000000322", "org-a", "b", base},
		{"00000000-0000-4000-8000-000000000123", "00000000-0000-4000-8000-000000000223", "00000000-0000-4000-8000-000000000323", "org-a", "c", base.Add(time.Minute)},
		{"00000000-0000-4000-8000-000000000124", "00000000-0000-4000-8000-000000000224", "00000000-0000-4000-8000-000000000324", "org-b", "d", base.Add(2 * time.Minute)},
	} {
		store := newPersistenceStore(t, in.org, in.id, in.key, in.allocation, "Store "+in.external, "SG", in.external, in.at)
		if _, _, err := repo.CreateOrReplay(context.Background(), in.org, store); err != nil {
			t.Fatal(err)
		}
	}
	page, err := repo.List(context.Background(), "org-a", storecenter.StoreListQuery{Page: 0, PageSize: 0})
	if err != nil || page.Total != 3 || len(page.Stores) != 3 {
		t.Fatalf("default list = (%d, %d, %v), want 3 scoped stores", page.Total, len(page.Stores), err)
	}
	want := []string{"00000000-0000-4000-8000-000000000123", "00000000-0000-4000-8000-000000000121", "00000000-0000-4000-8000-000000000122"}
	for i, id := range want {
		if page.Stores[i].ID() != id {
			t.Fatalf("ordered ID[%d] = %q, want %q", i, page.Stores[i].ID(), id)
		}
	}
	capped, err := repo.List(context.Background(), "org-a", storecenter.StoreListQuery{Page: 1, PageSize: 500, Platform: storecenter.PlatformShein, Status: storecenter.StoreStatusProvisioning})
	if err != nil || capped.Total != 3 || len(capped.Stores) != 3 {
		t.Fatalf("filtered capped list = (%d, %d, %v), want 3", capped.Total, len(capped.Stores), err)
	}
	second, err := repo.List(context.Background(), "org-a", storecenter.StoreListQuery{Page: 2, PageSize: 2})
	if err != nil || len(second.Stores) != 1 || second.Stores[0].ID() != want[2] {
		t.Fatalf("second page = (%v, %v), want final stable row", second.Stores, err)
	}
	_, err = repo.List(context.Background(), "org-a", storecenter.StoreListQuery{Page: int(^uint(0) >> 1), PageSize: 100})
	if err == nil || !strings.Contains(err.Error(), "page offset overflows") {
		t.Fatalf("overflowing page error = %v, want page offset overflow", err)
	}
}

// Mutation caught: probing by ID without Organization or treating a missing
// conditional write as always a conflict would leak another enterprise's row.
func TestGormStoreRepositoryScopesGetsAndClassifiesVersionedWrites(t *testing.T) {
	repo := newStoreRepository(t)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000131", "00000000-0000-4000-8000-000000000231", "00000000-0000-4000-8000-000000000331", "North", "SG", "external", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(context.Background(), "org-b", store.ID()); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("cross-org Get error = %v, want ErrNotFound", err)
	}

	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatalf("Save current version error = %v", err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); !errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("stale Save error = %v, want ErrVersionConflict", err)
	}
	if err := repo.Save(context.Background(), "org-b", store, 1); err == nil {
		t.Fatal("Save with Organization argument/snapshot mismatch error = nil, want rejection")
	}
	foreignSnapshot := store.Snapshot()
	foreignSnapshot.OrganizationID = "org-b"
	foreignStore, err := storecenter.RehydrateStore(foreignSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-b", foreignStore, 1); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("cross-org-shaped Save error = %v, want ErrNotFound", err)
	}
	bad := store.Snapshot()
	bad.Version = 9
	invalid, err := storecenter.RehydrateStore(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", invalid, 2); err == nil {
		t.Fatal("Save with non-successor snapshot version error = nil, want error")
	}
}

// Mutation caught: trusting a publicly rehydrated snapshot would let an edit
// replace durable external/quota/audit identity or regress lifecycle state.
func TestGormStoreRepositoryRejectsCraftedImmutableOrIllegalLifecycleSave(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*storecenter.StoreSnapshot)
	}{
		{"version and provenance only", func(snapshot *storecenter.StoreSnapshot) {}},
		{"external identity", func(snapshot *storecenter.StoreSnapshot) { snapshot.ExternalStoreID = "forged-external" }},
		{"quota allocation", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.QuotaAllocationID = "00000000-0000-4000-8000-000000000337"
		}},
		{"create key", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.CreateIdempotencyKey = "00000000-0000-4000-8000-000000000237"
		}},
		{"created actor", func(snapshot *storecenter.StoreSnapshot) { snapshot.CreatedBy = "forged-creator" }},
		{"created time", func(snapshot *storecenter.StoreSnapshot) { snapshot.CreatedAt = snapshot.CreatedAt.Add(-time.Minute) }},
		{"connection reference", func(snapshot *storecenter.StoreSnapshot) { snapshot.ConnectionRef = "new-opaque-ref" }},
		{"lifecycle and edit combined", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.LifecycleStatus, snapshot.Name = storecenter.StoreStatusDisabled, "Combined edit"
		}},
		{"lifecycle regression", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.LifecycleStatus = storecenter.StoreStatusProvisioning
		}},
		{"stale update time", func(snapshot *storecenter.StoreSnapshot) {
			snapshot.UpdatedAt = snapshot.UpdatedAt.Add(-2 * time.Minute)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newStoreRepository(t)
			store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000136", "00000000-0000-4000-8000-000000000236", "00000000-0000-4000-8000-000000000336", "North", "SG", "external", testPersistenceTime)
			if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
				t.Fatal(err)
			}
			if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
				t.Fatal(err)
			}
			before, err := repo.Get(context.Background(), "org-a", store.ID())
			if err != nil {
				t.Fatal(err)
			}
			snapshot := before.Snapshot()
			snapshot.Version++
			snapshot.UpdatedBy = "subject-candidate"
			snapshot.UpdatedAt = snapshot.UpdatedAt.Add(time.Minute)
			test.mutate(&snapshot)
			crafted, err := storecenter.RehydrateStore(snapshot)
			if err != nil {
				t.Fatalf("RehydrateStore(crafted) error = %v", err)
			}
			if err := repo.Save(context.Background(), "org-a", crafted, before.Version()); err == nil {
				t.Fatal("Save(crafted) error = nil, want immutable/lifecycle rejection")
			}
			after, err := repo.Get(context.Background(), "org-a", store.ID())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after.Snapshot(), before.Snapshot()) {
				t.Fatalf("rejected Save mutated durable store: got %#v, want %#v", after.Snapshot(), before.Snapshot())
			}
		})
	}
}

// Mutation caught: a caller must not be able to manufacture a second durable
// delete transition merely by advancing version/provenance on an already
// deleting aggregate with the same operation key.
func TestGormStoreRepositoryRejectsDeletingSameKeyVersionOnlySave(t *testing.T) {
	repo := newStoreRepository(t)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-00000000013a", "00000000-0000-4000-8000-00000000023a", "00000000-0000-4000-8000-00000000033a", "North", "SG", "external-delete-bump", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginDelete("00000000-0000-4000-8000-00000000043a", "subject-delete", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 2); err != nil {
		t.Fatal(err)
	}

	before, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := before.Snapshot()
	snapshot.Version++
	snapshot.UpdatedBy = "subject-forged"
	snapshot.UpdatedAt = snapshot.UpdatedAt.Add(time.Minute)
	crafted, err := storecenter.RehydrateStore(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", crafted, before.Version()); err == nil {
		t.Fatal("Save(deleting same-key version-only snapshot) error = nil, want rejection")
	}
	after, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Snapshot(), before.Snapshot()) {
		t.Fatalf("rejected Save mutated durable store: got %#v, want %#v", after.Snapshot(), before.Snapshot())
	}
}

// Mutation caught: omitting a mutable field from the update map or overwriting
// durable immutable creation fields breaks the persisted aggregate round trip.
func TestGormStoreRepositoryRoundTripsEveryLiveAggregateField(t *testing.T) {
	repo := newStoreRepository(t)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000137", "00000000-0000-4000-8000-000000000237", "00000000-0000-4000-8000-000000000337", "North", "SG", "external", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatal(err)
	}
	snapshot := store.Snapshot()
	snapshot.Name, snapshot.Region = "Renamed", "MY"
	snapshot.Version++
	snapshot.UpdatedBy, snapshot.UpdatedAt = "subject-edit", snapshot.UpdatedAt.Add(time.Minute)
	edited, err := storecenter.RehydrateStore(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", edited, 2); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Snapshot(), snapshot) {
		t.Fatalf("round-tripped snapshot = %#v, want %#v", got.Snapshot(), snapshot)
	}
}

// Mutation caught: allowing direct delete or hard deletion would bypass the
// lifecycle/retention boundary and lose the deletion version/timestamp.
func TestGormStoreRepositorySoftDeletesOnlyDeletingRows(t *testing.T) {
	db := openStoreDB(t)
	repo, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000141", "00000000-0000-4000-8000-000000000241", "00000000-0000-4000-8000-000000000341", "North", "SG", "external", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(context.Background(), "org-a", store.ID(), 1); !errors.Is(err, storecenter.ErrInvalidTransition) {
		t.Fatalf("SoftDelete active/provisioning error = %v, want ErrInvalidTransition", err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginDelete("00000000-0000-4000-8000-000000000941", "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 2); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(context.Background(), "org-b", store.ID(), 3); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("cross-org SoftDelete error = %v, want ErrNotFound", err)
	}
	if err := repo.SoftDelete(context.Background(), "org-a", store.ID(), 2); !errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("stale SoftDelete error = %v, want ErrVersionConflict", err)
	}
	if err := repo.SoftDelete(context.Background(), "org-a", store.ID(), 3); err != nil {
		t.Fatalf("SoftDelete deleting row error = %v", err)
	}
	if _, err := repo.Get(context.Background(), "org-a", store.ID()); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("Get deleted row error = %v, want ErrNotFound", err)
	}
	page, err := repo.List(context.Background(), "org-a", storecenter.StoreListQuery{})
	if err != nil || page.Total != 0 {
		t.Fatalf("List after delete = (%d, %v), want zero", page.Total, err)
	}
	var row struct {
		Version   int64
		DeletedAt *time.Time `gorm:"column:deleted_at"`
	}
	if err := db.Table("workbench_stores").Unscoped().Select("version, deleted_at").Where("organization_id = ? AND id = ?", "org-a", store.ID()).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Version != 4 || row.DeletedAt == nil || row.DeletedAt.IsZero() {
		t.Fatalf("soft-deleted row = %#v, want version 4 and timestamp", row)
	}
}

// Mutation caught: choosing a deletion wall-clock earlier than a future
// durable update makes the soft-deleted record fail Store rehydration rules.
func TestGormStoreRepositorySoftDeleteNeverBackdatesFutureDurableUpdate(t *testing.T) {
	db := openStoreDB(t)
	repo, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000148", "00000000-0000-4000-8000-000000000248", "00000000-0000-4000-8000-000000000348", "North", "SG", "external", future)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-update", future.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginDelete("00000000-0000-4000-8000-000000000948", "subject-update", future.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 2); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(context.Background(), "org-a", store.ID(), 3); err != nil {
		t.Fatal(err)
	}
	var row struct {
		UpdatedAt time.Time `gorm:"column:updated_at"`
		DeletedAt time.Time `gorm:"column:deleted_at"`
		Version   int64
	}
	if err := db.Table("workbench_stores").Unscoped().Select("updated_at, deleted_at, version").Where("organization_id = ? AND id = ?", "org-a", store.ID()).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.DeletedAt.Before(future.Add(2*time.Minute)) || row.UpdatedAt != row.DeletedAt || row.Version != 4 {
		t.Fatalf("soft delete row = %#v, want durable-update-or-later timestamp and version 4", row)
	}
}

// Mutation caught: resolving uniqueness only among active rows would leak a
// SQLite constraint error when a deleted row still owns the durable key.
func TestGormStoreRepositoryClassifiesDeletedRowIdentityCollisions(t *testing.T) {
	repo := newStoreRepository(t)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000146", "00000000-0000-4000-8000-000000000246", "00000000-0000-4000-8000-000000000346", "North", "SG", "external", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginDelete("00000000-0000-4000-8000-000000000946", "subject-update", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 2); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(context.Background(), "org-a", store.ID(), 3); err != nil {
		t.Fatal(err)
	}
	duplicate := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000147", "00000000-0000-4000-8000-000000000247", "00000000-0000-4000-8000-000000000347", "New", "SG", "external", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", duplicate); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("deleted-row identity collision error = %v, want ErrAlreadyExists", err)
	}
}

// Mutation caught: replacing the version guard with an unconditional update
// permits both concurrent edits to report success and silently overwrites one.
func TestGormStoreRepositoryConcurrentSavesHaveOneWinner(t *testing.T) {
	db := openStoreDB(t)
	repo, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000151", "00000000-0000-4000-8000-000000000251", "00000000-0000-4000-8000-000000000351", "North", "SG", "external", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	left, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	right, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []*storecenter.Store{left, right} {
		if err := candidate.TransitionTo(storecenter.StoreStatusActive, "subject-update", candidate.UpdatedAt().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	errorsOut := make(chan error, 2)
	var group sync.WaitGroup
	for _, candidate := range []*storecenter.Store{left, right} {
		group.Add(1)
		go func(candidate *storecenter.Store) {
			defer group.Done()
			<-start
			errorsOut <- repo.Save(context.Background(), "org-a", candidate, 1)
		}(candidate)
	}
	close(start)
	group.Wait()
	close(errorsOut)
	var successes, conflicts int
	for err := range errorsOut {
		if err == nil {
			successes++
		} else if errors.Is(err, storecenter.ErrVersionConflict) {
			conflicts++
		} else {
			t.Fatalf("concurrent Save error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent saves = %d success, %d conflicts, want 1/1", successes, conflicts)
	}
}

// Mutation caught: validating a stale candidate's lifecycle or timestamp
// before its durable version returns an invalid-transition error instead of
// the optimistic-concurrency conflict the caller must recover from.
func TestGormStoreRepositoryPrioritizesVersionConflictOverStaleLifecycleValidation(t *testing.T) {
	repo := newStoreRepository(t)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000156", "00000000-0000-4000-8000-000000000256", "00000000-0000-4000-8000-000000000356", "North", "SG", "external", testPersistenceTime)
	if _, _, err := repo.CreateOrReplay(context.Background(), "org-a", store); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject-active", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatal(err)
	}
	winner, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	stale, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	if err := winner.BeginDelete("00000000-0000-4000-8000-000000000956", "subject-winner", winner.UpdatedAt().Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", winner, 2); err != nil {
		t.Fatal(err)
	}
	if err := stale.TransitionTo(storecenter.StoreStatusDisabled, "subject-stale", stale.UpdatedAt().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), "org-a", stale, 2); !errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("stale Save error = %v, want ErrVersionConflict", err)
	}
	got, err := repo.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	if got.LifecycleStatus() != storecenter.StoreStatusDeleting || got.Version() != 3 || got.UpdatedBy() != "subject-winner" {
		t.Fatalf("stale Save mutated winner: got status=%q version=%d actor=%q", got.LifecycleStatus(), got.Version(), got.UpdatedBy())
	}
}

var testPersistenceTime = time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)

func newStoreRepository(t *testing.T) *storecenter.GormStoreRepository {
	t.Helper()
	repo, err := storecenter.NewGormStoreRepository(openStoreDB(t))
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func openStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stores.sqlite")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", filepath.ToSlash(path))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open SQLite sql handle = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		t.Fatal(err)
	}
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func newPersistenceStore(t *testing.T, organizationID, id, key, allocation, name, region, external string, at time.Time) *storecenter.Store {
	t.Helper()
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: id, OrganizationID: organizationID, ActorSubject: "subject-create", Name: name, Platform: "shein", Region: region, ExternalStoreID: external, CreateIdempotencyKey: key, QuotaAllocationID: allocation, OccurredAt: at})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

func assertSQLiteIndex(t *testing.T, db *gorm.DB, table string, wantColumns []string, wantUnique bool) {
	t.Helper()
	type index struct {
		Name   string `gorm:"column:name"`
		Unique int    `gorm:"column:unique"`
	}
	var indexes []index
	if err := db.Raw("PRAGMA index_list(" + table + ")").Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		var columns []struct {
			Name string `gorm:"column:name"`
		}
		if err := db.Raw("PRAGMA index_info(" + index.Name + ")").Scan(&columns).Error; err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(columns))
		for i := range columns {
			got[i] = columns[i].Name
		}
		if fmt.Sprint(got) == fmt.Sprint(wantColumns) && (index.Unique == 1) == wantUnique {
			return
		}
	}
	sort.Strings(wantColumns)
	t.Fatalf("index with columns %v and unique=%t not found", wantColumns, wantUnique)
}
