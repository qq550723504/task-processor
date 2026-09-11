package sourceaccountregistry

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRuntimePermissionQueryRejectsCrossOwnerPrivileges(t *testing.T) {
	for _, table := range []string{
		"public.saas_tenant_subscriptions",
		"public.saas_plans",
		"public.saas_tenant_entitlements",
		"public.saas_usage_buckets",
	} {
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "TRIGGER", "REFERENCES"} {
			fragment := fmt.Sprintf("has_table_privilege(current_user, '%s', '%s')", table, privilege)
			if !strings.Contains(runtimePermissionQuery, fragment) {
				t.Fatalf("runtimePermissionQuery does not reject cross-owner %s on %s", privilege, table)
			}
		}
	}
}

func TestVerifyRuntimePermissionsRequiresExactLeastPrivilegeRole(t *testing.T) {
	for _, test := range []struct {
		name      string
		user      string
		required  bool
		forbidden bool
		wantError bool
	}{
		{name: "admitted", user: "source_account_runtime", required: true},
		{name: "missing DML", user: "source_account_runtime", required: false, wantError: true},
		{name: "delete privilege", user: "source_account_runtime", required: true, forbidden: true, wantError: true},
		{name: "owner role", user: "postgres", required: true, forbidden: true, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery(regexp.QuoteMeta(runtimePermissionQuery)).WillReturnRows(sqlmock.NewRows([]string{"current_user", "required_privileges", "forbidden_privileges"}).AddRow(test.user, test.required, test.forbidden))
			err = VerifyRuntimePermissions(context.Background(), db)
			if (err != nil) != test.wantError {
				t.Fatalf("VerifyRuntimePermissions() error = %v, wantError=%t", err, test.wantError)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
