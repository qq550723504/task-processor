package listingsubscription

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCommercialPermissionQueryRejectsSourceAccountPrivileges(t *testing.T) {
	for _, table := range []string{
		"public.source_account_resources",
		"public.source_account_operations",
	} {
		if !strings.Contains(commercialReadPermissionQuery, table) {
			t.Fatalf("commercialReadPermissionQuery does not inspect cross-owner table %s", table)
		}
	}
}

func TestVerifyCommercialReadSchemaUsesOnlyBoundedReadProbes(t *testing.T) {
	db, mock, cleanup := commercialSchemaMock(t)
	defer cleanup()

	mock.ExpectBegin()
	for _, query := range commercialReadSchemaQueries {
		mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"ready"}))
	}
	mock.ExpectQuery(regexp.QuoteMeta(commercialReadPermissionQuery)).WillReturnRows(sqlmock.NewRows([]string{"current_user", "required_privileges", "forbidden_privileges"}).AddRow("commercial_reader", true, false))
	mock.ExpectRollback()

	if err := VerifyCommercialReadSchema(context.Background(), db); err != nil {
		t.Fatalf("VerifyCommercialReadSchema() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCommercialReadSchemaRejectsOverprivilegedRole(t *testing.T) {
	db, mock, cleanup := commercialSchemaMock(t)
	defer cleanup()
	mock.ExpectBegin()
	for _, query := range commercialReadSchemaQueries {
		mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"ready"}))
	}
	mock.ExpectQuery(regexp.QuoteMeta(commercialReadPermissionQuery)).WillReturnRows(sqlmock.NewRows([]string{"current_user", "required_privileges", "forbidden_privileges"}).AddRow("commercial_reader", true, true))
	mock.ExpectRollback()
	if err := VerifyCommercialReadSchema(context.Background(), db); err == nil {
		t.Fatal("overprivileged commercial role was accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCommercialReadSchemaRejectsMissingReadBoundary(t *testing.T) {
	db, mock, cleanup := commercialSchemaMock(t)
	defer cleanup()

	want := errors.New("permission denied for table saas_plans")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(commercialReadSchemaQueries[0])).WillReturnRows(sqlmock.NewRows([]string{"ready"}))
	mock.ExpectQuery(regexp.QuoteMeta(commercialReadSchemaQueries[1])).WillReturnError(want)
	mock.ExpectRollback()

	err := VerifyCommercialReadSchema(context.Background(), db)
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("VerifyCommercialReadSchema() error = %v, want wrapped %v", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCommercialReadSchemaRejectsNilDatabase(t *testing.T) {
	if err := VerifyCommercialReadSchema(context.Background(), nil); err == nil {
		t.Fatal("nil database was accepted")
	}
}

func commercialSchemaMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatal(err)
	}
	return db, mock, func() { _ = sqlDB.Close() }
}
