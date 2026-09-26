package orgresourceadapter

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestResourceRuntimeRequiresExistingOwnerRoleAndExactOwnedPrivileges(t *testing.T) {
	for _, test := range []struct {
		name, role                 string
		required, forbidden, valid bool
	}{
		{"owner runtime", "commercial_owner_runtime", true, false, true},
		{"old token role", "commercial_runtime", true, false, false},
		{"schema owner", "postgres", true, false, false},
		{"missing monthly grants", "commercial_owner_runtime", false, false, false},
		{"owned delete or DDL", "commercial_owner_runtime", true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			pool, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = pool.Close() })
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{})
			require.NoError(t, err)
			mock.ExpectQuery(regexp.QuoteMeta(resourceRuntimePermissionQuery)).WillReturnRows(sqlmock.NewRows([]string{"role_name", "required", "forbidden"}).AddRow(test.role, test.required, test.forbidden))
			err = VerifyRuntimePermissions(context.Background(), db)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
