package ownerreconcile

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresExceptionStoreLoadsActiveRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `SELECT table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason
FROM listingkit_owner_scope_system_owned_exceptions
WHERE active = TRUE
ORDER BY table_name, tenant_fingerprint, candidate_fingerprint`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{
		"table_name", "tenant_fingerprint", "candidate_fingerprint", "report_fingerprint", "reason",
	}).AddRow("listing_store", "sha256:000000000000", "sha256:111111111111", "648cdfab03c4", "legacy orphaned owner"))

	store := NewPostgresExceptionStore(db)
	got, err := store.ListActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Table != "listing_store" || got[0].ReportFingerprint != "648cdfab03c4" {
		t.Fatalf("exceptions = %+v, want one redacted exception", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresExceptionStoreRejectsInvalidRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `SELECT table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason
FROM listingkit_owner_scope_system_owned_exceptions
WHERE active = TRUE
ORDER BY table_name, tenant_fingerprint, candidate_fingerprint`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{
		"table_name", "tenant_fingerprint", "candidate_fingerprint", "report_fingerprint", "reason",
	}).AddRow("", "sha256:000000000000", "sha256:111111111111", "648cdfab03c4", "legacy orphaned owner"))

	_, err = NewPostgresExceptionStore(db).ListActive(context.Background())
	if err == nil || !strings.Contains(err.Error(), "exception registry") {
		t.Fatalf("err = %v, want sanitized exception registry error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresExceptionStoreSanitizesQueryErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := `SELECT table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason
FROM listingkit_owner_scope_system_owned_exceptions
WHERE active = TRUE
ORDER BY table_name, tenant_fingerprint, candidate_fingerprint`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(errors.New("password=secret tenant=private"))

	_, err = NewPostgresExceptionStore(db).ListActive(context.Background())
	if err == nil || strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "private") {
		t.Fatalf("err = %v, want sanitized registry error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
