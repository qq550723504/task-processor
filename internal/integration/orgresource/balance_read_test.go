package orgresourceadapter

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/ledger/orgresource"
)

func TestReadBalancesReflectsOwnerGrantWithoutWrites(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo, _ := NewGormRepository(db, TransactionConfig{})
	service, _ := orgresource.NewService(repo, &mutableEligibilityVerifier{approval: orgresource.WelcomeGrantApproval{OrganizationID: "org-a", EvidenceID: "welcome-a"}}, allowWelcomeGrantAuthorizer{})
	if _, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), orgresource.GrantWelcomeStoreRenewalPeriodInput{OrganizationID: "org-a", OperationID: "welcome-a", Principal: orgresource.Principal{ID: "worker", Kind: orgresource.PrincipalTrustedProvisioning}}); err != nil {
		t.Fatal(err)
	}
	var writes int
	db.Callback().Create().Before("gorm:create").Register("balance-no-create", func(_ *gorm.DB) { writes++ })
	db.Callback().Update().Before("gorm:update").Register("balance-no-update", func(_ *gorm.DB) { writes++ })
	db.Callback().Delete().Before("gorm:delete").Register("balance-no-delete", func(_ *gorm.DB) { writes++ })
	before := time.Now().UTC()
	got, err := repo.ReadBalances(context.Background(), "org-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.OrganizationID != "org-a" || got.ObservedAt.Before(before) || len(got.Resources) != 3 {
		t.Fatalf("snapshot=%+v", got)
	}
	first := got.Resources[0]
	if first.State != "recorded" || first.Available == nil || *first.Available != "1" || first.Debt == nil || *first.Debt != "0" {
		t.Fatalf("grant not reflected: %+v", first)
	}
	for _, value := range got.Resources[1:] {
		if value.State != "not_recorded" || value.Available != nil || value.UpdatedAt != nil {
			t.Fatalf("invented zero: %+v", value)
		}
	}
	other, err := repo.ReadBalances(context.Background(), "org-b")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range other.Resources {
		if value.State != "not_recorded" {
			t.Fatalf("cross-org read: %+v", value)
		}
	}
	if writes != 0 {
		t.Fatalf("read performed %d writes", writes)
	}
	for _, table := range []string{"saas_organization_resource_operations", "saas_organization_resource_events", "saas_organization_resource_audit_logs"} {
		assertTableCount(t, db, table, 1)
	}
}

func TestReadBalancesZeroExactIntegersDebtAndReserved(t *testing.T) {
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rows := []organizationResourceBucketRow{
		{OrganizationID: "org-a", ResourceType: "store_renewal_period", UpdatedAt: now},
		{OrganizationID: "org-a", ResourceType: "ai_point", Reserved: 9007199254740993, Consumed: 4, UpdatedAt: now},
		{OrganizationID: "org-a", ResourceType: "data_row", Available: 9223372036854775807, UpdatedAt: now},
		{OrganizationID: "org-b", ResourceType: "data_row", Available: 7, UpdatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&organizationResourceDebtRow{OrganizationID: "org-a", ResourceType: "ai_point", Amount: 12, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repo, _ := NewGormRepository(db, TransactionConfig{})
	got, err := repo.ReadBalances(context.Background(), "org-a")
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"0", "0", "9223372036854775807"} {
		value := got.Resources[i]
		if value.State != "recorded" || value.Available == nil || *value.Available != want {
			t.Fatalf("resource %d: %+v", i, value)
		}
	}
	if *got.Resources[1].Reserved != "9007199254740993" || *got.Resources[1].Consumed != "4" || *got.Resources[1].Debt != "12" {
		t.Fatal(got.Resources[1])
	}
	again, err := repo.ReadBalances(context.Background(), "org-a")
	if err != nil || !reflect.DeepEqual(got.Resources, again.Resources) {
		t.Fatalf("read altered reservation: %+v %v", again, err)
	}
}

func TestReadBalancesRejectsCorruptFactsAndDependencyFailures(t *testing.T) {
	for _, setup := range []string{"missing_tables", "orphan_debt", "negative", "debt_with_available", "invalid_timestamp"} {
		t.Run(setup, func(t *testing.T) {
			db := openSQLiteStore(t)
			if setup != "missing_tables" {
				// Deliberately unconstrained fixture models corruption, never production repair.
				for _, statement := range []string{"CREATE TABLE saas_organization_resource_buckets (organization_id TEXT,resource_type TEXT,available BIGINT,reserved BIGINT,consumed BIGINT,updated_at DATETIME)", "CREATE TABLE saas_organization_resource_debts (organization_id TEXT,resource_type TEXT,amount BIGINT)"} {
					if err := db.Exec(statement).Error; err != nil {
						t.Fatal(err)
					}
				}
				if setup == "orphan_debt" {
					db.Exec("INSERT INTO saas_organization_resource_debts VALUES ('org-a','ai_point',1)")
				} else {
					available := int64(0)
					if setup == "negative" {
						available = -1
					}
					if setup == "debt_with_available" {
						available = 1
					}
					updated := "2026-09-28T00:00:00Z"
					if setup == "invalid_timestamp" {
						updated = "bad"
					}
					if err := db.Exec("INSERT INTO saas_organization_resource_buckets VALUES ('org-a','ai_point',?,0,0,?)", available, updated).Error; err != nil {
						t.Fatal(err)
					}
					if setup == "debt_with_available" {
						db.Exec("INSERT INTO saas_organization_resource_debts VALUES ('org-a','ai_point',1)")
					}
				}
			}
			repo, _ := NewGormRepository(db, TransactionConfig{})
			if _, err := repo.ReadBalances(context.Background(), "org-a"); !errors.Is(err, orgresource.ErrBalanceUnavailable) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	db := openSQLiteStore(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo, _ := NewGormRepository(db, TransactionConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.ReadBalances(ctx, "org-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	if _, err := repo.ReadBalances(context.Background(), ""); !errors.Is(err, orgresource.ErrInvalidInput) {
		t.Fatalf("invalid org error=%v", err)
	}
}
