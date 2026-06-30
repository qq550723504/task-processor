package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMemStudioBatchTaskLinkRepositoryCreateAndLoadByCandidateKey(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryCreateAndLoadByCandidateKey(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryCreateAndLoadByCandidateKey(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryCreateAndLoadByCandidateKey(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryListByBatch(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryListByBatch(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryListByBatch(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryListByBatch(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryTenantIsolation(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryTenantIsolation(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryTenantIsolation(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryTenantIsolation(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryUniqueCandidateKey(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUniqueCandidateKey(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryUniqueCandidateKey(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUniqueCandidateKey(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryUniqueTuple(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUniqueTuple(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryUniqueTuple(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUniqueTuple(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentStores(t *testing.T) {
	t.Parallel()

	testStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentStores(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentStores(t *testing.T) {
	t.Parallel()

	testStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentStores(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentCompatibilityFingerprints(t *testing.T) {
	t.Parallel()

	testStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentCompatibilityFingerprints(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentCompatibilityFingerprints(t *testing.T) {
	t.Parallel()

	testStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentCompatibilityFingerprints(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryUpdateProjectionStatus(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUpdateProjectionStatus(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryUpdateProjectionStatus(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUpdateProjectionStatus(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryCreateUsesContextScope(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryCreateUsesContextScope(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryCreateUsesContextScope(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryCreateUsesContextScope(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryRejectsEmptyAndDuplicateID(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryRejectsEmptyAndDuplicateID(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryRejectsEmptyAndDuplicateID(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryRejectsEmptyAndDuplicateID(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryUpdatePreservesImmutableFields(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUpdatePreservesImmutableFields(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryUpdatePreservesImmutableFields(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryUpdatePreservesImmutableFields(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryClaimCandidate(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryClaimCandidate(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryClaimCandidate(t *testing.T) {
	t.Parallel()
	testStudioBatchTaskLinkRepositoryClaimCandidate(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func TestMemStudioBatchTaskLinkRepositoryClaimCandidateUpdatedAt(t *testing.T) {
	t.Parallel()

	testStudioBatchTaskLinkRepositoryClaimCandidateUpdatedAt(t, func(t *testing.T) StudioBatchTaskLinkRepository {
		t.Helper()
		return NewMemStudioBatchTaskLinkRepository()
	})
}

func TestGormStudioBatchTaskLinkRepositoryClaimCandidateUpdatedAt(t *testing.T) {
	t.Parallel()

	testStudioBatchTaskLinkRepositoryClaimCandidateUpdatedAt(t, newGormStudioBatchTaskLinkRepositoryForTest)
}

func testStudioBatchTaskLinkRepositoryCreateAndLoadByCandidateKey(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	link := studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")

	if err := repo.CreateStudioBatchTaskLink(ctx, link); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}

	got, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctx, "candidate-1")
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if got.ID != "link-1" || got.ListingKitTaskID != "task-link-1" || got.TenantID != "tenant-a" {
		t.Fatalf("got link = %+v, want created link scoped to tenant-a", got)
	}
}

func testStudioBatchTaskLinkRepositoryListByBatch(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-2", "batch-1", "item-2", "design-1", "selection-2", "candidate-2"))
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1"))
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-3", "batch-2", "item-3", "design-2", "selection-3", "candidate-3"))

	got, err := repo.ListStudioBatchTaskLinksByBatchID(ctx, "batch-1")
	if err != nil {
		t.Fatalf("ListStudioBatchTaskLinksByBatchID() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListStudioBatchTaskLinksByBatchID()) = %d, want 2", len(got))
	}
	if got[0].ID != "link-1" || got[1].ID != "link-2" {
		t.Fatalf("got link ids = [%s %s], want [link-1 link-2]", got[0].ID, got[1].ID)
	}
}

func testStudioBatchTaskLinkRepositoryTenantIsolation(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")
	linkA := studioBatchTaskLinkRecordForTest("link-a", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")
	linkB := studioBatchTaskLinkRecordForTest("link-b", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")

	mustCreateStudioBatchTaskLinkForTest(t, repo, ctxA, linkA)
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctxB, linkB)

	gotA, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctxA, "candidate-1")
	if err != nil {
		t.Fatalf("tenant-a GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	gotB, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctxB, "candidate-1")
	if err != nil {
		t.Fatalf("tenant-b GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if gotA.ID != "link-a" || gotB.ID != "link-b" {
		t.Fatalf("got tenant links = (%s, %s), want isolated records", gotA.ID, gotB.ID)
	}

	linksA, err := repo.ListStudioBatchTaskLinksByBatchID(ctxA, "batch-1")
	if err != nil {
		t.Fatalf("tenant-a ListStudioBatchTaskLinksByBatchID() error = %v", err)
	}
	if len(linksA) != 1 || linksA[0].ID != "link-a" {
		t.Fatalf("tenant-a links = %+v, want only link-a", linksA)
	}
}

func testStudioBatchTaskLinkRepositoryUniqueCandidateKey(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1"))

	duplicate := studioBatchTaskLinkRecordForTest("link-2", "batch-1", "item-2", "design-1", "selection-2", "candidate-1")
	if err := repo.CreateStudioBatchTaskLink(ctx, duplicate); err == nil {
		t.Fatal("CreateStudioBatchTaskLink() duplicate candidate key error = nil, want error")
	}
}

func testStudioBatchTaskLinkRepositoryUniqueTuple(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1"))

	duplicate := studioBatchTaskLinkRecordForTest("link-2", "batch-1", "item-1", "design-1", "selection-1", "candidate-2")
	if err := repo.CreateStudioBatchTaskLink(ctx, duplicate); err == nil {
		t.Fatal("CreateStudioBatchTaskLink() duplicate tuple error = nil, want error")
	}
}

func testStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentStores(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1"))

	storeB := studioBatchTaskLinkRecordForTest("link-2", "batch-1", "item-1", "design-1", "selection-1", "candidate-2")
	storeB.SheinStoreID = 2002
	if err := repo.CreateStudioBatchTaskLink(ctx, storeB); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink(different store) error = %v, want allowed distinct store candidate", err)
	}
}

func testStudioBatchTaskLinkRepositoryAllowsSameTupleForDifferentCompatibilityFingerprints(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1"))

	changed := studioBatchTaskLinkRecordForTest("link-2", "batch-1", "item-1", "design-1", "selection-1", "candidate-2")
	changed.CompatibilityFingerprint = "fingerprint-selection-1-with-product-size"
	if err := repo.CreateStudioBatchTaskLink(ctx, changed); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink(different compatibility fingerprint) error = %v, want allowed regenerated candidate", err)
	}
}

func testStudioBatchTaskLinkRepositoryUpdateProjectionStatus(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	link := studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, link)

	link.Status = "draft_saved"
	link.ReasonCode = "submission_saved"
	link.Message = "draft saved to SHEIN"
	link.UpdatedAt = link.UpdatedAt.Add(time.Minute)
	if err := repo.UpdateStudioBatchTaskLink(ctx, link); err != nil {
		t.Fatalf("UpdateStudioBatchTaskLink() error = %v", err)
	}

	got, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctx, "candidate-1")
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if got.Status != "draft_saved" || got.ReasonCode != "submission_saved" || got.Message != "draft saved to SHEIN" {
		t.Fatalf("got projection = (%q, %q, %q), want updated status projection", got.Status, got.ReasonCode, got.Message)
	}

	missing := *link
	missing.ID = "missing-link"
	if err := repo.UpdateStudioBatchTaskLink(ctx, &missing); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateStudioBatchTaskLink(missing) error = %v, want record not found", err)
	}
}

func testStudioBatchTaskLinkRepositoryCreateUsesContextScope(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")
	link := studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")
	link.TenantID = "tenant-b"
	link.UserID = "caller-user"

	if err := repo.CreateStudioBatchTaskLink(ctxA, link); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}

	got, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctxA, "candidate-1")
	if err != nil {
		t.Fatalf("tenant-a GetStudioBatchTaskLinkByCandidateKey() error = %v", err)
	}
	if got.TenantID != "tenant-a" || got.UserID != "" {
		t.Fatalf("got scope = (%q, %q), want context-derived tenant and user", got.TenantID, got.UserID)
	}
	if _, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctxB, "candidate-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant-b GetStudioBatchTaskLinkByCandidateKey() error = %v, want record not found", err)
	}
}

func testStudioBatchTaskLinkRepositoryRejectsEmptyAndDuplicateID(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	emptyID := studioBatchTaskLinkRecordForTest("", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")
	if err := repo.CreateStudioBatchTaskLink(ctx, emptyID); err == nil {
		t.Fatal("CreateStudioBatchTaskLink(empty id) error = nil, want error")
	}

	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1"))
	duplicateID := studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-2", "design-1", "selection-2", "candidate-2")
	if err := repo.CreateStudioBatchTaskLink(ctx, duplicateID); err == nil {
		t.Fatal("CreateStudioBatchTaskLink(duplicate id) error = nil, want error")
	}
}

func testStudioBatchTaskLinkRepositoryUpdatePreservesImmutableFields(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")
	link := studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctxA, link)

	mutated := *link
	mutated.TenantID = "tenant-b"
	mutated.UserID = "intruder"
	mutated.BatchID = "batch-2"
	mutated.ItemID = "item-2"
	mutated.DesignID = "design-2"
	mutated.SelectionID = "selection-2"
	mutated.CandidateKey = "candidate-2"
	mutated.SheinStoreID = 2002
	mutated.CompatibilityFingerprint = "fingerprint-mutated"
	mutated.ListingKitTaskID = "task-updated"
	mutated.Status = "creating"
	mutated.ReasonCode = "reserved"
	mutated.Message = "projection updated"
	mutated.UpdatedAt = mutated.UpdatedAt.Add(time.Minute)

	if err := repo.UpdateStudioBatchTaskLink(ctxA, &mutated); err != nil {
		t.Fatalf("UpdateStudioBatchTaskLink() error = %v", err)
	}

	got, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctxA, "candidate-1")
	if err != nil {
		t.Fatalf("GetStudioBatchTaskLinkByCandidateKey(original key) error = %v", err)
	}
	if got.TenantID != "tenant-a" || got.UserID != "" ||
		got.BatchID != "batch-1" || got.ItemID != "item-1" || got.DesignID != "design-1" || got.SelectionID != "selection-1" ||
		got.CandidateKey != "candidate-1" || got.SheinStoreID != 1001 || got.CompatibilityFingerprint != "fingerprint-selection-1" {
		t.Fatalf("got immutable fields = %+v, want original identity", got)
	}
	if got.ListingKitTaskID != "task-updated" || got.Status != "creating" || got.ReasonCode != "reserved" || got.Message != "projection updated" {
		t.Fatalf("got projection = %+v, want updated projection fields", got)
	}
	if _, err := repo.GetStudioBatchTaskLinkByCandidateKey(ctxB, "candidate-2"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant-b mutated lookup error = %v, want record not found", err)
	}
}

func testStudioBatchTaskLinkRepositoryClaimCandidate(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctxA := WithTenantID(context.Background(), "tenant-a")
	ctxB := WithTenantID(context.Background(), "tenant-b")
	link := studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")
	link.Status = "reserved"
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctxA, link)

	now := link.UpdatedAt.Add(time.Minute)
	got, claimed, err := repo.ClaimStudioBatchTaskCandidate(ctxA, "candidate-1", "reserved", "creating", now)
	if err != nil {
		t.Fatalf("ClaimStudioBatchTaskCandidate(first) error = %v", err)
	}
	if !claimed || got.Status != "creating" {
		t.Fatalf("first claim = (%v, %+v), want claimed creating record", claimed, got)
	}

	got, claimed, err = repo.ClaimStudioBatchTaskCandidate(ctxA, "candidate-1", "reserved", "creating", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimStudioBatchTaskCandidate(second) error = %v", err)
	}
	if claimed || got.Status != "creating" {
		t.Fatalf("second claim = (%v, %+v), want loser with existing creating record", claimed, got)
	}
	if _, _, err := repo.ClaimStudioBatchTaskCandidate(ctxB, "candidate-1", "creating", "created", now.Add(2*time.Minute)); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant claim error = %v, want record not found", err)
	}
}

func testStudioBatchTaskLinkRepositoryClaimCandidateUpdatedAt(t *testing.T, newRepo func(*testing.T) StudioBatchTaskLinkRepository) {
	t.Helper()

	repo := newRepo(t)
	ctx := WithTenantID(context.Background(), "tenant-a")
	link := studioBatchTaskLinkRecordForTest("link-1", "batch-1", "item-1", "design-1", "selection-1", "candidate-1")
	link.Status = "creating"
	mustCreateStudioBatchTaskLinkForTest(t, repo, ctx, link)

	observed := link.UpdatedAt
	firstUpdatedAt := observed.Add(time.Minute)
	got, claimed, err := repo.ClaimStudioBatchTaskCandidateUpdatedAt(ctx, "candidate-1", "creating", observed, "creating", firstUpdatedAt)
	if err != nil {
		t.Fatalf("ClaimStudioBatchTaskCandidateUpdatedAt(first) error = %v", err)
	}
	if !claimed || !got.UpdatedAt.Equal(firstUpdatedAt) {
		t.Fatalf("first CAS claim = (%v, %+v), want claimed with updated timestamp %s", claimed, got, firstUpdatedAt)
	}

	got, claimed, err = repo.ClaimStudioBatchTaskCandidateUpdatedAt(ctx, "candidate-1", "creating", observed, "creating", firstUpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimStudioBatchTaskCandidateUpdatedAt(second) error = %v", err)
	}
	if claimed || !got.UpdatedAt.Equal(firstUpdatedAt) {
		t.Fatalf("second stale CAS claim = (%v, %+v), want loser preserving first update", claimed, got)
	}
}

func newGormStudioBatchTaskLinkRepositoryForTest(t *testing.T) StudioBatchTaskLinkRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrateStudioBatchTaskLinkRepository(db); err != nil {
		t.Fatalf("AutoMigrateStudioBatchTaskLinkRepository() error = %v", err)
	}
	return NewGormStudioBatchTaskLinkRepository(db)
}

func studioBatchTaskLinkRecordForTest(id string, batchID string, itemID string, designID string, selectionID string, candidateKey string) *StudioBatchTaskLinkRecord {
	now := time.Now().UTC()
	return &StudioBatchTaskLinkRecord{
		ID:                       id,
		BatchID:                  batchID,
		ItemID:                   itemID,
		DesignID:                 designID,
		SelectionID:              selectionID,
		CompatibilityFingerprint: "fingerprint-" + selectionID,
		SheinStoreID:             1001,
		ListingKitTaskID:         "task-" + id,
		CandidateKey:             candidateKey,
		Status:                   "task_created",
		CreatedAt:                now,
		UpdatedAt:                now,
	}
}

func mustCreateStudioBatchTaskLinkForTest(t *testing.T, repo StudioBatchTaskLinkRepository, ctx context.Context, link *StudioBatchTaskLinkRecord) {
	t.Helper()

	if err := repo.CreateStudioBatchTaskLink(ctx, link); err != nil {
		t.Fatalf("CreateStudioBatchTaskLink() error = %v", err)
	}
}
