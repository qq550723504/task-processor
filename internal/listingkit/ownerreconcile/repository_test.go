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
	if len(report.Resolutions) != 1 || report.Resolutions[0].SubjectFingerprint != shortFingerprint("subject-1") || report.Resolutions[0].CandidateFingerprint != shortFingerprint("creator=legacy-1") {
		t.Fatalf("resolutions = %+v, want one redacted auto-resolution", report.Resolutions)
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

func TestRepositoryDryRunDoesNotAutoResolveUnmappedNonEmptyCandidate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := `SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-unknown", int64(2)))
	repository := Repository{Queryer: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: query, Columns: []string{"tenant_id", "creator", "row_count"},
		CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}},
	}}}
	report, err := repository.DryRun(context.Background(), []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-other", Subject: "subject-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.AutoRows != 0 || len(report.Findings) != 1 || report.Findings[0].Reason != "unmapped_candidate" {
		t.Fatalf("report = %+v, want unmapped candidate finding", report)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryDryRunSkipsOnlyPostgresUndefinedTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	missing := "SELECT tenant_id, creator, COUNT(*) FROM future_table WHERE owner_user_id IS NULL GROUP BY tenant_id, creator"
	mock.ExpectQuery(regexp.QuoteMeta(missing)).WillReturnError(postgresStateError{state: "42P01", message: "relation future_table does not exist"})
	repository := Repository{Queryer: db, Inventory: []TableSpec{{Table: "future_table", Query: missing, Columns: []string{"tenant_id", "creator", "row_count"}, CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}}}}}
	if report, err := repository.DryRun(context.Background(), nil); err != nil || report.Summary.AffectedRows != 0 {
		t.Fatalf("report = %+v, err = %v, want undefined table skipped", report, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectCandidateGroupsSkipsPostgresUndefinedTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := "SELECT tenant_id, creator, COUNT(*) FROM future_table WHERE owner_user_id IS NULL GROUP BY tenant_id, creator"
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(postgresStateError{state: "42P01", message: "relation future_table does not exist"})
	spec := TableSpec{Table: "future_table", Query: query, Columns: []string{"tenant_id", "creator", "row_count"}, CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}}}
	groups, err := collectCandidateGroups(context.Background(), db, spec, nil)
	if err != nil || len(groups) != 0 {
		t.Fatalf("groups = %+v, err = %v, want missing table skipped", groups, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryDryRunClassifiesSystemOwnedRowsOutsideOwnerScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := `SELECT tenant_id, COUNT(*) FROM listing_kit_tasks WHERE user_id IS NULL GROUP BY tenant_id`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "row_count"}).AddRow("org-1", int64(5)))
	repository := Repository{Queryer: db, Inventory: []TableSpec{{
		Table: "listing_kit_tasks", Query: query, CandidatePolicy: CandidatePolicySystemOwned,
		Columns: []string{"tenant_id", "row_count"},
	}}}
	report, err := repository.DryRun(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.UnresolvedRows != 0 || report.Summary.SystemOwnedRows != 5 || len(report.Findings) != 0 || len(report.SystemOwnedFindings) != 1 {
		t.Fatalf("report = %+v, want system-owned rows outside unresolved findings", report)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryDryRunClassifiesRowsWithoutAnyCandidateAsSystemOwned(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := `SELECT tenant_id, creator, created_by, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator, created_by`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "creator", "created_by", "row_count"}).AddRow("tenant-1", "", "", int64(9)))
	repository := Repository{Queryer: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: query, CandidatePolicy: CandidatePolicyCreatorFirst,
		Columns:          []string{"tenant_id", "creator", "created_by", "row_count"},
		CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}, {Name: "created_by", Source: "created_by"}},
	}}}
	report, err := repository.DryRun(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.UnresolvedRows != 0 || report.Summary.SystemOwnedRows != 9 || len(report.Findings) != 0 || len(report.SystemOwnedFindings) != 1 {
		t.Fatalf("report = %+v, want no-candidate rows outside unresolved findings", report)
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

type postgresStateError struct {
	state   string
	message string
}

func (err postgresStateError) Error() string    { return err.message }
func (err postgresStateError) SQLState() string { return err.state }

func TestLoadLegacyIdentitiesValidatesOrganizationAndUserMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	orgQuery := `SELECT org_id::text, convert_from(value, 'UTF8') AS legacy_tenant_id FROM projections.org_metadata2 WHERE key = 'yudao_tenant_id' AND owner_removed = false`
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
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT org_id::text, convert_from(value, 'UTF8') AS legacy_tenant_id FROM projections.org_metadata2 WHERE key = 'yudao_tenant_id' AND owner_removed = false`)).
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

func TestLoadLegacyIdentitiesRejectsAmbiguousLegacyTenantMapping(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	orgQuery := `SELECT org_id::text, convert_from(value, 'UTF8') AS legacy_tenant_id FROM projections.org_metadata2 WHERE key = 'yudao_tenant_id' AND owner_removed = false`
	mock.ExpectQuery(regexp.QuoteMeta(orgQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"org_id", "legacy_tenant_id"}).
			AddRow("org-1", "tenant-1").AddRow("org-2", "tenant-1"))
	if _, err := LoadLegacyIdentities(context.Background(), db); err == nil {
		t.Fatal("expected ambiguous legacy tenant mapping to fail closed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryApplyUniqueRequiresExactConfirmationBeforeAnyWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := Repository{Queryer: db, Beginner: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: "SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator",
		Columns: []string{"tenant_id", "creator", "row_count"}, CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}},
		UpdateQuery:    "WITH target AS (SELECT id FROM listing_store WHERE tenant_id = $2 AND creator::text = $3 ORDER BY id LIMIT $4) UPDATE listing_store AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id",
		UpdateLimitArg: 4,
	}}, Identities: []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}}}
	if _, err := repository.ApplyUnique(context.Background(), "abc123", "different", 10); !errors.Is(err, ErrReportConfirmationMismatch) {
		t.Fatalf("error = %v, want confirmation mismatch", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryApplyUniqueRechecksReportAndUpdatesOnlyUniqueRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := "SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator"
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-1", int64(4))
	}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	base := Repository{Queryer: db, Beginner: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: query, Columns: []string{"tenant_id", "creator", "row_count"},
		CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}},
		UpdateQuery:      "WITH target AS (SELECT id FROM listing_store WHERE tenant_id = $2 AND creator::text = $3 ORDER BY id LIMIT $4) UPDATE listing_store AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id",
		UpdateLimitArg:   4,
	}}, Identities: []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}}}
	report, err := base.DryRun(context.Background(), base.Identities)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.SetFingerprint(); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("WITH target AS (SELECT id FROM listing_store WHERE tenant_id = $2 AND creator::text = $3 ORDER BY id LIMIT $4) UPDATE listing_store AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id")).WithArgs("subject-1", "tenant-1", "legacy-1", int64(4)).WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}))
	summary, err := base.ApplyUnique(context.Background(), report.ReportFingerprint, report.ReportFingerprint, 10)
	if err != nil {
		t.Fatal(err)
	}
	if summary.RowsUpdated != 4 || summary.Batches != 1 {
		t.Fatalf("summary = %+v, want four rows in one batch", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryApplyUniqueAllowsSystemOwnedRowsAfterAutoResolution(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	autoQuery := "SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator"
	systemQuery := "SELECT tenant_id, COUNT(*) FROM listing_kit_tasks WHERE user_id IS NULL GROUP BY tenant_id"
	update := "WITH target AS (SELECT id FROM listing_store WHERE tenant_id = $2 AND creator::text = $3 ORDER BY id LIMIT $4) UPDATE listing_store AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id"
	autoRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-1", int64(4))
	}
	systemRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"tenant_id", "row_count"}).AddRow("tenant-1", int64(7))
	}
	repository := Repository{Queryer: db, Beginner: db, Inventory: []TableSpec{
		{Table: "listing_store", Query: autoQuery, Columns: []string{"tenant_id", "creator", "row_count"}, CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}}, UpdateQuery: update, UpdateLimitArg: 4},
		{Table: "listing_kit_tasks", Query: systemQuery, CandidatePolicy: CandidatePolicySystemOwned, Columns: []string{"tenant_id", "row_count"}},
	}, Identities: []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}}}
	mock.ExpectQuery(regexp.QuoteMeta(autoQuery)).WillReturnRows(autoRows())
	mock.ExpectQuery(regexp.QuoteMeta(systemQuery)).WillReturnRows(systemRows())
	report, err := repository.DryRun(context.Background(), repository.Identities)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.AutoRows != 4 || report.Summary.SystemOwnedRows != 7 {
		t.Fatalf("report summary = %+v, want four auto and seven system-owned rows", report.Summary)
	}
	if err := report.SetFingerprint(); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(autoQuery)).WillReturnRows(autoRows())
	mock.ExpectQuery(regexp.QuoteMeta(systemQuery)).WillReturnRows(systemRows())
	mock.ExpectQuery(regexp.QuoteMeta(autoQuery)).WillReturnRows(autoRows())
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(update)).WithArgs("subject-1", "tenant-1", "legacy-1", int64(4)).WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(autoQuery)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}))
	mock.ExpectQuery(regexp.QuoteMeta(systemQuery)).WillReturnRows(systemRows())
	if _, err := repository.ApplyUnique(context.Background(), report.ReportFingerprint, report.ReportFingerprint, 10); err != nil {
		t.Fatalf("apply error = %v, want system-owned rows to be allowed after auto resolution", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryApplyUniqueLimitsEachUpdateBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := "SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator"
	update := "WITH target AS (SELECT id FROM listing_store WHERE tenant_id = $2 AND creator::text = $3 ORDER BY id LIMIT $4) UPDATE listing_store AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id"
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-1", int64(25))
	}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	repository := Repository{Queryer: db, Beginner: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: query, Columns: []string{"tenant_id", "creator", "row_count"}, CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}}, UpdateQuery: update, UpdateLimitArg: 4,
	}}, Identities: []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}}}
	report, err := repository.DryRun(context.Background(), repository.Identities)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.SetFingerprint(); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	mock.ExpectRollback()
	for _, affected := range []int64{10, 10, 5} {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(update)).WithArgs("subject-1", "tenant-1", "legacy-1", affected).WillReturnResult(sqlmock.NewResult(0, affected))
		mock.ExpectCommit()
	}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}))
	summary, err := repository.ApplyUnique(context.Background(), report.ReportFingerprint, report.ReportFingerprint, 10)
	if err != nil {
		t.Fatal(err)
	}
	if summary.RowsUpdated != 25 || summary.Batches != 3 {
		t.Fatalf("summary = %+v, want 25 rows in three bounded batches", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryApplyUniqueRejectsCandidateChangesAfterConfirmation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := "SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator"
	update := "WITH target AS (SELECT id FROM listing_store WHERE tenant_id = $2 AND creator::text = $3 ORDER BY id LIMIT $4) UPDATE listing_store AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id"
	first := sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-1", int64(4))
	changed := sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-2", int64(4))
	repository := Repository{Queryer: db, Beginner: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: query, Columns: []string{"tenant_id", "creator", "row_count"}, CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}}, UpdateQuery: update, UpdateLimitArg: 4,
	}}, Identities: []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}, {TenantID: "tenant-1", LegacyUserID: "legacy-2", Subject: "subject-2"}}}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-1", int64(4)))
	report, err := repository.DryRun(context.Background(), repository.Identities)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.SetFingerprint(); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(first)
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(changed)
	mock.ExpectRollback()
	if _, err := repository.ApplyUnique(context.Background(), report.ReportFingerprint, report.ReportFingerprint, 10); !errors.Is(err, ErrReportConfirmationMismatch) {
		t.Fatalf("error = %v, want confirmation mismatch", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryApplyUniqueRejectsRowsLeftByFinalRescan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := "SELECT tenant_id, creator, COUNT(*) FROM listing_store WHERE owner_user_id IS NULL GROUP BY tenant_id, creator"
	update := "WITH target AS (SELECT id FROM listing_store WHERE tenant_id = $2 AND creator::text = $3 ORDER BY id LIMIT $4) UPDATE listing_store AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id"
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"tenant_id", "creator", "row_count"}).AddRow("tenant-1", "legacy-1", int64(1))
	}
	repository := Repository{Queryer: db, Beginner: db, Inventory: []TableSpec{{
		Table: "listing_store", Query: query, Columns: []string{"tenant_id", "creator", "row_count"}, CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}}, UpdateQuery: update, UpdateLimitArg: 4,
	}}, Identities: []LegacyIdentity{{TenantID: "tenant-1", LegacyUserID: "legacy-1", Subject: "subject-1"}}}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	report, err := repository.DryRun(context.Background(), repository.Identities)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.SetFingerprint(); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(update)).WithArgs("subject-1", "tenant-1", "legacy-1", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows())
	if _, err := repository.ApplyUnique(context.Background(), report.ReportFingerprint, report.ReportFingerprint, 10); err == nil {
		t.Fatal("expected final rescan to reject a remaining blank-owner row")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

