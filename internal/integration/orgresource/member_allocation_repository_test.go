package orgresourceadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"task-processor/internal/ledger/orgresource"
)

func TestMemberAllocationConservesResourceAndReplaysOriginalReceipt(t *testing.T) {
	for _, resource := range []orgresource.ResourceType{orgresource.ResourceStoreRenewalPeriod, orgresource.ResourceDataRow} {
		t.Run(string(resource), func(t *testing.T) {
			db := openSQLiteStore(t)
			if err := AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: string(resource), Available: 100}).Error; err != nil {
				t.Fatal(err)
			}
			repo, err := NewGormMemberAllocationRepository(db, TransactionConfig{})
			if err != nil {
				t.Fatal(err)
			}
			allocate := orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: "membership-a", ActorID: "admin-a", OperationID: "allocate-a", ResourceType: resource, Action: orgresource.MemberResourceAllocate, Quantity: 10}
			first, err := repo.Transfer(context.Background(), allocate)
			if err != nil {
				t.Fatal(err)
			}
			if first.Position.Free != 10 || first.Position.Version != 1 || first.Unallocated != 90 || first.Allocated != 10 {
				t.Fatalf("allocation: %#v", first)
			}
			if first.Position.UpdatedAt.Location() != time.UTC {
				t.Fatalf("original receipt time is not canonical UTC: %v", first.Position.UpdatedAt)
			}
			reclaim := allocate
			reclaim.Action, reclaim.OperationID, reclaim.Quantity, reclaim.ExpectedVersion = orgresource.MemberResourceReclaim, "reclaim-a", 3, 1
			second, err := repo.Transfer(context.Background(), reclaim)
			if err != nil {
				t.Fatal(err)
			}
			if second.Position.Free != 7 || second.Position.Version != 2 || second.Unallocated != 93 || second.Allocated != 7 {
				t.Fatalf("reclaim: %#v", second)
			}
			replay, err := repo.Transfer(context.Background(), allocate)
			if err != nil {
				t.Fatal(err)
			}
			if !replay.Replayed || replay.Position != first.Position || replay.Unallocated != first.Unallocated {
				t.Fatalf("replay changed original receipt: %#v", replay)
			}
			changed := allocate
			changed.Quantity++
			if _, err := repo.Transfer(context.Background(), changed); !errors.Is(err, orgresource.ErrIdempotencyKeyConflict) {
				t.Fatalf("different payload: %v", err)
			}
			if _, err := repo.Transfer(context.Background(), orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: "membership-a", ActorID: "admin-a", OperationID: "stale", ResourceType: resource, Action: orgresource.MemberResourceAllocate, Quantity: 1, ExpectedVersion: 1}); !errors.Is(err, orgresource.ErrMemberResourceVersionConflict) {
				t.Fatalf("stale version: %v", err)
			}
			position, err := repo.ReadPosition(context.Background(), "org-a", "membership-a", resource)
			if err != nil {
				t.Fatal(err)
			}
			if position.Free != 7 || position.Version != 2 {
				t.Fatalf("failed command changed balance: %#v", position)
			}
			assertTableCount(t, db, "saas_organization_resource_operations", 2)
			assertTableCount(t, db, "saas_organization_resource_events", 2)
		})
	}
}

func TestMemberResourcePositionCanonicalTimeSurvivesReceiptJSON(t *testing.T) {
	// Gorm's clock can carry the host location and monotonic metadata.
	// These cannot be part of the domain receipt's replay identity.
	for _, observed := range []time.Time{time.Now(), time.Date(2026, 9, 29, 8, 0, 0, 123456000, time.FixedZone("test", 8*60*60))} {
		position := memberResourcePosition(memberResourcePositionRow{UpdatedAt: observed})
		if position.UpdatedAt.Location() != time.UTC || !position.UpdatedAt.Equal(observed) {
			t.Fatalf("receipt did not retain the instant as UTC: %v", position.UpdatedAt)
		}
		encoded, err := json.Marshal(position)
		if err != nil {
			t.Fatal(err)
		}
		var replay orgresource.MemberResourcePosition
		if err := json.Unmarshal(encoded, &replay); err != nil {
			t.Fatal(err)
		}
		if replay != position {
			t.Fatalf("receipt representation changed after JSON replay")
		}
	}
}

func TestMemberAllocationConcurrentAdministratorsCannotOverallocate(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "data_row", Available: 10}).Error; err != nil {
		t.Fatal(err)
	}
	repo, err := NewGormMemberAllocationRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	errorsSeen := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wait.Add(1)
		go func(n int) {
			defer wait.Done()
			_, err := repo.Transfer(context.Background(), orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: fmt.Sprintf("member-%d", n), ActorID: "admin", OperationID: fmt.Sprintf("allocate-%d", n), ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 10})
			errorsSeen <- err
		}(n)
	}
	wait.Wait()
	close(errorsSeen)
	success, insufficient := 0, 0
	for err := range errorsSeen {
		if err == nil {
			success++
		} else if errors.Is(err, orgresource.ErrInsufficientBalance) {
			insufficient++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || insufficient != 1 {
		t.Fatalf("success=%d insufficient=%d", success, insufficient)
	}
	var bucket organizationResourceBucketRow
	if err := db.Take(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Available != 0 || bucket.Allocated != 10 {
		t.Fatalf("overallocated: %#v", bucket)
	}
}

func TestMemberAllocationOverflowAndDatabaseBoundsRejectMintedBalance(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "data_row", Available: 1, Allocated: math.MaxInt64}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&memberResourcePositionRow{OrganizationID: "org-a", MemberID: "member-a", ResourceType: "data_row", Free: math.MaxInt64}).Error; err != nil {
		t.Fatal(err)
	}
	repo, err := NewGormMemberAllocationRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Transfer(context.Background(), orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: "member-a", ActorID: "admin", OperationID: "allocate", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 1}); !errors.Is(err, orgresource.ErrInvalidInput) {
		t.Fatalf("overflow: %v", err)
	}
	if err := db.Model(&organizationResourceBucketRow{}).Where("organization_id = ?", "org-a").Update("allocated", -1).Error; err == nil {
		t.Fatal("database accepted negative allocated balance")
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "ai_point", Allocated: 1}).Error; err == nil {
		t.Fatal("database accepted separately allocated AI points")
	}
}

func TestMemberReclaimRepaysDebtBeforeReturningUnallocatedResources(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "data_row", Available: 10}).Error; err != nil {
		t.Fatal(err)
	}
	repo, err := NewGormMemberAllocationRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	command := orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: "member-a", ActorID: "admin", OperationID: "allocate", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 10}
	if _, err := repo.Transfer(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceDebtRow{OrganizationID: "org-a", ResourceType: "data_row", Amount: 4}).Error; err != nil {
		t.Fatal(err)
	}
	command.Action, command.OperationID, command.ExpectedVersion = orgresource.MemberResourceReclaim, "reclaim", 1
	result, err := repo.Transfer(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.GrossCredit != 10 || result.DebtRepaid != 4 || result.NetCredit != 6 || result.Unallocated != 6 || result.Allocated != 0 || result.Position.Free != 0 {
		t.Fatalf("debt conservation: %#v", result)
	}
	var debt organizationResourceDebtRow
	if err := db.Take(&debt).Error; err != nil {
		t.Fatal(err)
	}
	if debt.Amount != 0 {
		t.Fatalf("remaining debt %d", debt.Amount)
	}
}

func TestMemberAllocationCannotSpendDebtOrReclaimReservedResources(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceBucketRow{OrganizationID: "org-a", ResourceType: "data_row", Allocated: 5, Reserved: 5}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&memberResourcePositionRow{OrganizationID: "org-a", MemberID: "member-a", ResourceType: "data_row", Free: 5, Reserved: 5}).Error; err != nil {
		t.Fatal(err)
	}
	repo, err := NewGormMemberAllocationRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	command := orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: "member-a", ActorID: "admin", OperationID: "reclaim", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceReclaim, Quantity: 6}
	if _, err := repo.Transfer(context.Background(), command); !errors.Is(err, orgresource.ErrInsufficientBalance) {
		t.Fatalf("reclaimed reserved units: %v", err)
	}
	if err := db.Model(&organizationResourceBucketRow{}).Where("organization_id = ?", "org-a").Update("available", 10).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceDebtRow{OrganizationID: "org-a", ResourceType: "data_row", Amount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	command.Action, command.OperationID, command.Quantity = orgresource.MemberResourceAllocate, "allocate", 1
	if _, err := repo.Transfer(context.Background(), command); !errors.Is(err, orgresource.ErrResourceDebtOutstanding) {
		t.Fatalf("allocation spent through debt: %v", err)
	}
	command.ResourceType = orgresource.ResourceAIPoint
	if _, err := repo.Transfer(context.Background(), command); !errors.Is(err, orgresource.ErrInvalidInput) {
		t.Fatalf("created separate AI balance: %v", err)
	}
	assertTableCount(t, db, "saas_organization_resource_operations", 0)
}
