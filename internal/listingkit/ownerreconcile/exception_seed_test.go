package ownerreconcile

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestValidateApprovedExceptionReportRejectsWrongConfirmation(t *testing.T) {
	report := Report{
		ReportFingerprint: "deadbeefdead",
		Summary:           ReportSummary{FindingGroups: 1, UnresolvedRows: 1},
		Findings: []Finding{{
			Table: "listing_store", TenantFingerprint: "sha256:000000000000", OwnerFingerprint: "sha256:111111111111", Rows: 1, Reason: "unmapped_candidate",
		}},
	}
	if err := ValidateApprovedExceptionReport(report, "648cdfab03c4"); err == nil {
		t.Fatal("expected confirmation mismatch")
	}
}

func TestInsertSystemOwnedExceptionsUsesOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	report := Report{
		ReportFingerprint: "648cdfab03c4",
		Findings: []Finding{{
			Table: "listing_store", TenantFingerprint: "sha256:000000000000", OwnerFingerprint: "sha256:111111111111", Rows: 1, Reason: "unmapped_candidate",
		}},
	}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO listingkit_owner_scope_system_owned_exceptions
    (table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason, active)
VALUES ($1, $2, $3, $4, $5, TRUE)
ON CONFLICT (table_name, tenant_fingerprint, candidate_fingerprint) DO NOTHING`)).
		WithArgs("listing_store", "sha256:000000000000", "sha256:111111111111", "648cdfab03c4", "approved current orphaned owner").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if got, err := InsertSystemOwnedExceptions(context.Background(), db, report, "approved current orphaned owner"); err != nil || got != 1 {
		t.Fatalf("insert = (%d, %v), want one inserted row", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateApprovedExceptionReportRejectsNonUnmappedFinding(t *testing.T) {
	report := Report{
		ReportFingerprint: "648cdfab03c4",
		Summary:           ReportSummary{FindingGroups: 1, UnresolvedRows: 1},
		Findings: []Finding{{
			Table: "listing_store", TenantFingerprint: "sha256:000000000000", OwnerFingerprint: "sha256:111111111111", Rows: 1, Reason: "conflicting_candidates",
		}},
	}
	if err := ValidateApprovedExceptionReport(report, "648cdfab03c4"); err == nil || !strings.Contains(err.Error(), "approved") {
		t.Fatalf("err = %v, want approved report validation error", err)
	}
}
