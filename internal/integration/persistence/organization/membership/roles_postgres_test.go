//go:build integration

package membership

import (
	"context"
	"net/url"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authz"
	domain "task-processor/internal/organization/membership"
)

func testRolePolicyReader(t *testing.T, ctx context.Context, owner *gorm.DB, dsn string, writer *Repository) {
	require.NoError(t, owner.Exec(`CREATE ROLE organization_role_policy_reader LOGIN PASSWORD 'test-only-policy-password';
 REVOKE CREATE,TEMPORARY ON DATABASE membership_test FROM PUBLIC;
 REVOKE CREATE ON SCHEMA public FROM PUBLIC;
 GRANT CONNECT ON DATABASE membership_test TO organization_role_policy_reader;
 GRANT USAGE ON SCHEMA public TO organization_role_policy_reader;
 GRANT SELECT ON public.organization_role_slots,public.organization_roles TO organization_role_policy_reader;`).Error)
	connection, err := url.Parse(dsn)
	require.NoError(t, err)
	connection.User = url.UserPassword(RolePolicyReaderRole, "test-only-policy-password")
	db, err := gorm.Open(postgres.Open(connection.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	reader, err := NewRolePolicyReader(ctx, db, writer.projectID)
	require.NoError(t, err)
	tx, err := writer.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, InstallRoleSlotsTx(ctx, tx, writer.projectID, "reader-org"))
	require.NoError(t, tx.Commit())
	scope := domain.OperationScope{ProjectID: writer.projectID, OrganizationID: "reader-org", ActorID: "admin"}
	role, err := writer.MutateRole(ctx, scope, uuid.NewString(), domain.RoleMutation{Name: "图片操作", Modules: []string{"images"}})
	require.NoError(t, err)
	modules, err := reader.RoleModules(ctx, scope.OrganizationID, []string{role.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"images"}, modules[role.ID])
	_, err = writer.MutateRole(ctx, scope, uuid.NewString(), domain.RoleMutation{RoleID: role.ID, Modules: []string{}, ExpectedVersion: role.Version})
	require.NoError(t, err)
	modules, err = reader.RoleModules(ctx, scope.OrganizationID, []string{role.ID})
	require.NoError(t, err)
	require.Empty(t, modules[role.ID])
	require.Error(t, db.Exec(`UPDATE public.organization_roles SET name='forbidden'`).Error)
	require.NoError(t, owner.Exec(`GRANT INSERT ON public.organization_roles TO organization_role_policy_reader`).Error)
	_, err = NewRolePolicyReader(ctx, db, writer.projectID)
	require.Error(t, err, "broader credentials must be refused")
	require.NoError(t, owner.Exec(`REVOKE INSERT ON public.organization_roles FROM organization_role_policy_reader`).Error)
}

func testEnterpriseRoles(t *testing.T, ctx context.Context, r *Repository) {
	tx, err := r.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, InstallRoleSlotsTx(ctx, tx, r.projectID, "roles-a"))
	require.NoError(t, InstallRoleSlotsTx(ctx, tx, r.projectID, "roles-b"))
	require.NoError(t, tx.Commit())
	scope := domain.OperationScope{ProjectID: r.projectID, OrganizationID: "roles-a", ActorID: "admin"}
	key := uuid.NewString()
	input := domain.RoleMutation{Name: " 商品运营 ", Modules: []string{"acquisition"}}
	role, err := r.MutateRole(ctx, scope, key, input)
	require.NoError(t, err)
	require.Equal(t, "商品运营", role.Name)
	require.EqualValues(t, 1, role.Version)
	repeated, err := r.MutateRole(ctx, scope, key, input)
	require.NoError(t, err)
	require.Equal(t, role, repeated)
	_, err = r.MutateRole(ctx, scope, key, domain.RoleMutation{Name: "另一角色", Modules: input.Modules})
	require.ErrorIs(t, err, domain.ErrConflict)
	_, err = r.MutateRole(ctx, scope, uuid.NewString(), input)
	require.ErrorIs(t, err, domain.ErrConflict)
	policies, err := r.RoleModules(ctx, "roles-b", []string{role.ID})
	require.NoError(t, err)
	require.Empty(t, policies)
	policies, err = r.RoleModules(ctx, "roles-a", []string{authz.EnterpriseRoleKey("roles-a", 64)})
	require.NoError(t, err)
	require.Empty(t, policies)
	updated, err := r.MutateRole(ctx, scope, uuid.NewString(), domain.RoleMutation{RoleID: role.ID, Modules: []string{}, ExpectedVersion: role.Version})
	require.NoError(t, err)
	require.EqualValues(t, 2, updated.Version)
	_, err = r.MutateRole(ctx, scope, uuid.NewString(), domain.RoleMutation{RoleID: role.ID, Modules: input.Modules, ExpectedVersion: 1})
	require.ErrorIs(t, err, domain.ErrConflict)
	reopened := &Repository{db: r.db, projectID: r.projectID}
	items, remaining, err := reopened.Roles(ctx, "roles-a")
	require.NoError(t, err)
	require.Equal(t, []domain.RoleDefinition{updated}, items)
	require.Equal(t, 63, remaining)
	var wg sync.WaitGroup
	results := make(chan domain.RoleDefinition, 2)
	failures := make(chan error, 2)
	for _, name := range []string{"客服", "运营"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			item, err := r.MutateRole(ctx, scope, uuid.NewString(), domain.RoleMutation{Name: name, Modules: []string{"members"}})
			if err != nil {
				failures <- err
			} else {
				results <- item
			}
		}(name)
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	ids := map[string]bool{}
	for item := range results {
		require.False(t, ids[item.ID])
		ids[item.ID] = true
	}
	require.Len(t, ids, 2)
	_, err = r.MutateRole(ctx, scope, uuid.NewString(), domain.RoleMutation{Name: "越权", Modules: []string{"platform_admin"}})
	require.ErrorIs(t, err, domain.ErrInvalidRequest)
}
