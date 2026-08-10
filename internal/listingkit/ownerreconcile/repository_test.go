package ownerreconcile

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRepositoryDryRunAggregatesUniqueCandidateWithoutWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator`
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-1", int64(4)))

	repository := Repository{Queryer: db, Inventory: []TableSpec{{
		Table: "listing_store", TenantDomain: TenantDomainLegacyNumeric, Query: query,
		Columns:          []string{"tenant_id", "creator", "row_count"},
		CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}},
	}}}
	report, err := repository.DryRun(context.Background(), []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.AutoRows != 4 || report.Summary.UnresolvedRows != 0 || len(report.Findings) != 0 {
		t.Fatalf("report = %+v, want four auto rows and no findings", report)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryDryRunReportsConflictsWithoutLeakingCandidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `SELECT tenant_id, own_creator, task_creator, store_creator, COUNT(*) FROM listing_product_import_mapping WHERE owner_user_id IS NULL GROUP BY tenant_id, own_creator, task_creator, store_creator`
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "own_creator", "task_creator", "store_creator", "row_count"}).AddRow("tenant-1", "legacy-1", "legacy-2", "", int64(3)))

	repository := Repository{Queryer: db, Inventory: []TableSpec{{
		Table: "listing_product_import_mapping", TenantDomain: TenantDomainLegacyNumeric, Query: query,
		Columns:          []string{"tenant_id", "own_creator", "task_creator", "store_creator", "row_count"},
		CandidateColumns: []CandidateColumn{{Name: "own_creator", Source: "creator"}, {Name: "task_creator", Source: "task"}, {Name: "store_creator", Source: "store"}},
	}}}
	report, err := repository.DryRun(context.Background(), []LegacyIdentity{
		{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"},
		{TenantID: "tenant-1", LegacyUserID: "legacy-2", Subject: "subject-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.AutoRows != 0 || report.Summary.UnresolvedRows != 3 || len(report.Findings) != 1 || report.Findings[0].Reason != "conflicting_candidates" {
		t.Fatalf("report = %+v, want one redacted conflict finding", report)
	}
	fingerprint, err := report.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint == "" {
		t.Fatal("expected report fingerprint")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryDryRunRejectsNonSelectInventoryAndPreservesCancellation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	unsafe := Repository{Queryer: db, Inventory: []TableSpec{{Table: "listing_store", Query: "UPDATE listing_store SET owner_user_id = 'x'"}}}
	if _, err := unsafe.DryRun(context.Background(), nil); err == nil {
		t.Fatal("expected non-SELECT inventory to fail closed")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	repository := Repository{Queryer: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: "SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator",
		Columns: []string{"tenant_id", "creator", "row_count"},
	}}}
	if _, err := repository.DryRun(cancelled, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

var _ Queryer = (*sql.DB)(nil)

func TestLoadLegacyIdentitiesValidatesOrganizationAndUserMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	orgQuery := `SELECT org_id::text, convert_from(value, 'UTF8') AS legacy_tenant_id FROM projections.org_metadata2 WHERE key = 'yudao_tenant_id'`
	mock.ExpectQuery(regexp.QuoteMeta(orgQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"org_id", "legacy_tenant_id"}).AddRow("org-1", "tenant-1"))
	userQuery := `SELECT user_id::text, resource_owner::text, max(convert_from(value, 'UTF8')) FILTER (WHERE key = 'yudao_user_id') AS legacy_user_id, max(convert_from(value, 'UTF8')) FILTER (WHERE key = 'yudao_tenant_id') AS legacy_tenant_id FROM projections.user_metadata5 WHERE key IN ('yudao_user_id', 'yudao_tenant_id') GROUP BY user_id, resource_owner`
	mock.ExpectQuery(regexp.QuoteMeta(userQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "resource_owner", "legacy_user_id", "legacy_tenant_id"}).AddRow("subject-1", "org-1", "legacy-1", "tenant-1"))

	identities, err := LoadLegacyIdentities(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(identities) != 1 || identities[0] != (LegacyIdentity{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}) {
		t.Fatalf("identities = %+v, want one verified mapping", identities)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLegacyIdentitiesFailsClosedOnTenantMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT org_id::text, convert_from(value, 'UTF8') AS legacy_tenant_id FROM projections.org_metadata2 WHERE key = 'yudao_tenant_id'`)).
		WillReturnRows(sqlmock.NewRows([]string{"org_id", "legacy_tenant_id"}).AddRow("org-1", "tenant-1"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id::text, resource_owner::text, max(convert_from(value, 'UTF8')) FILTER (WHERE key = 'yudao_user_id') AS legacy_user_id, max(convert_from(value, 'UTF8')) FILTER (WHERE key = 'yudao_tenant_id') AS legacy_tenant_id FROM projections.user_metadata5 WHERE key IN ('yudao_user_id', 'yudao_tenant_id') GROUP BY user_id, resource_owner`)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "resource_owner", "legacy_user_id", "legacy_tenant_id"}).AddRow("subject-1", "org-1", "legacy-1", "tenant-2"))

	if _, err := LoadLegacyIdentities(context.Background(), db); err == nil {
		t.Fatal("expected tenant mismatch to fail closed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
