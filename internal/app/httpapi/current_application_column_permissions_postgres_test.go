//go:build integration

package httpapi

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	"task-processor/internal/listingsubscription"
)

// Reuse the task-owned PG17 fixture and actual Run/NewCurrentApplication chain.
// Grants are only fault injections by the fixture owner; preflight never repairs.
func run1ColumnPermissionMatrix(t *testing.T, owner, source, commercial *gorm.DB, tables []string, start func(*testing.T, bool)) {
	t.Helper()
	roles := []string{"source_account_runtime", "commercial_reader"}
	privileges := []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"}
	columns := make(map[string][]string, len(tables))
	for _, table := range tables {
		var names []string
		require.NoError(t, owner.Raw(`SELECT attname FROM pg_catalog.pg_attribute
			WHERE attrelid = ?::regclass AND attnum > 0 AND NOT attisdropped ORDER BY attnum`,
			pgx.Identifier{"public", table}.Sanitize()).Scan(&names).Error)
		require.NotEmpty(t, names)
		columns[table] = names
	}
	columnOnly := func(t *testing.T, role, table, privilege string) {
		t.Helper()
		db := source
		if role == "commercial_reader" {
			db = commercial
		}
		var effective struct {
			TablePrivilege  bool
			ColumnPrivilege bool
		}
		target := pgx.Identifier{"public", table}.Sanitize()
		require.NoError(t, db.Raw(`SELECT
			pg_catalog.has_table_privilege(current_user, ?::text, ?::text) AS table_privilege,
			pg_catalog.has_any_column_privilege(current_user, ?::text, ?::text) AS column_privilege`,
			target, privilege, target, privilege).Scan(&effective).Error)
		require.False(t, effective.TablePrivilege, "fixture must isolate column-only access")
		require.True(t, effective.ColumnPrivilege, "runtime role must have effective column access")
	}
	grant := func(t *testing.T, role, table, privilege string, names []string) func() {
		t.Helper()
		quoted := make([]string, len(names))
		for i, name := range names {
			quoted[i] = pgx.Identifier{name}.Sanitize()
		}
		clause := privilege + " (" + strings.Join(quoted, ",") + ") ON TABLE " + pgx.Identifier{"public", table}.Sanitize()
		require.NoError(t, owner.Exec("GRANT "+clause+" TO "+role).Error)
		return func() { require.NoError(t, owner.Exec("REVOKE "+clause+" FROM "+role).Error) }
	}
	for _, role := range roles {
		for _, table := range tables {
			for _, privilege := range privileges {
				t.Run(role+"/"+table+"/"+privilege, func(t *testing.T) {
					name := columns[table][0]
					if table == "source_account_operations" {
						name = "resulting_management_status" // Exact review counterexample.
					}
					revoke := grant(t, role, table, privilege, []string{name})
					defer func() { revoke(); start(t, false) }()
					forbidden := !slices.Contains(run1AllowedPrivileges[role][table], privilege)
					if forbidden {
						columnOnly(t, role, table, privilege)
					}
					// Redundant grants for already-allowed table privileges are valid.
					start(t, forbidden)
				})
			}
		}
		for _, privilege := range privileges {
			t.Run(role+"/inherited/"+privilege, func(t *testing.T) {
				require.NoError(t, owner.Exec("CREATE ROLE run1_column_inherited; GRANT run1_column_inherited TO "+role).Error)
				revoke := grant(t, "run1_column_inherited", "run1_additional_fact", privilege, []string{"value"})
				defer func() {
					revoke()
					require.NoError(t, owner.Exec("REVOKE run1_column_inherited FROM "+role+"; DROP ROLE run1_column_inherited").Error)
					start(t, false)
				}()
				columnOnly(t, role, "run1_additional_fact", privilege)
				start(t, true)
			})
		}
		for table, required := range run1AllowedPrivileges[role] {
			for _, privilege := range required {
				t.Run(role+"/columns_do_not_replace_required_table/"+table+"/"+privilege, func(t *testing.T) {
					target := pgx.Identifier{"public", table}.Sanitize()
					require.NoError(t, owner.Exec("REVOKE "+privilege+" ON TABLE "+target+" FROM "+role).Error)
					revoke := grant(t, role, table, privilege, columns[table])
					defer func() {
						revoke()
						require.NoError(t, owner.Exec("GRANT "+privilege+" ON TABLE "+target+" TO "+role).Error)
						start(t, false)
					}()
					columnOnly(t, role, table, privilege)
					start(t, true)
				})
			}
		}
	}
	for _, privilege := range privileges {
		t.Run("PUBLIC/"+privilege, func(t *testing.T) {
			revoke := grant(t, "PUBLIC", "run1_additional_fact", privilege, []string{"value"})
			defer func() { revoke(); start(t, false) }()
			for _, role := range roles {
				columnOnly(t, role, "run1_additional_fact", privilege)
			}
			// Both direct verifiers are checked: source rejects first in Run.
			assert.ErrorContains(t, sourceaccountstore.VerifyRuntimePermissions(context.Background(), source), "permissions do not match")
			assert.ErrorContains(t, listingsubscription.VerifyCommercialReadSchema(context.Background(), commercial), "permissions do not match")
			start(t, true)
		})
	}
}
