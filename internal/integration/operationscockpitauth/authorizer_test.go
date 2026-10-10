package operationscockpitauth

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	c "task-processor/internal/operationscockpit"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

type resolverFixture struct {
	id    authidentity.AuthenticatedIdentity
	calls int
}

type roleModulesFixture map[string]map[string][]string

func (f roleModulesFixture) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, key := range keys {
		if modules, ok := f[org][key]; ok {
			result[key] = modules
		}
	}
	return result, nil
}

func TestCockpitConfiguredAdminUsesNativeScopedStoreAccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, storecenter.AutoMigrateStoreRepository(db))
	records, err := storecenter.NewGormStoreRepository(db)
	require.NoError(t, err)
	storeIDs := map[string]string{}
	for _, org := range []string{"org-a", "org-b"} {
		storeIDs[org] = uuid.NewString()
		store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: storeIDs[org], OrganizationID: org, ActorSubject: "fixture-owner", Name: "Store", Platform: "shein", Region: "SG", CreateIdempotencyKey: uuid.NewString(), OccurredAt: time.Now().UTC()})
		require.NoError(t, err)
		_, _, err = records.CreateOrReplay(context.Background(), org, store)
		require.NoError(t, err)
	}
	role := authz.EnterpriseRoleKey("org-a", 1)
	modules := roleModulesFixture{"org-a": {role: {"overview-stores"}}}
	identity := authidentity.AuthenticatedIdentity{UserID: "configured-platform-user", TenantID: "org-a", EffectiveOrganizationID: "org-a", EffectiveMemberID: "member-a", TokenExpiresAt: time.Now().Add(time.Hour), Roles: []string{role}}
	resolver := &resolverFixture{id: identity}
	scope := c.Scope{OrganizationID: "org-a", ActorID: identity.UserID}
	ctx, cancel := context.WithTimeout(authidentity.WithAuthenticatedIdentity(context.Background(), identity), time.Minute)
	defer cancel()
	for _, configured := range []bool{true, false} {
		name := "regular_member_without_grants"
		var admins []string
		if configured {
			name, admins = "configured_admin", []string{identity.UserID}
		}
		t.Run(name, func(t *testing.T) {
			policy, err := authz.NewListingKitAuthorizer(admins, nil)
			require.NoError(t, err)
			policy.SetRolePolicyReader(modules)
			a, err := New(resolver, policy, db)
			require.NoError(t, err)
			bound, err := a.Bind(ctx, "Bearer fixture-token")
			require.NoError(t, err)
			access, err := a.Current(bound, scope)
			require.NoError(t, err)
			require.True(t, access.StoresRead)
			require.True(t, access.FactsWrite)
			stores, err := a.ListStores(bound, scope)
			require.NoError(t, err)
			if configured {
				require.Len(t, stores, 1)
				require.Equal(t, storeIDs["org-a"], stores[0].ID)
				require.NoError(t, a.ReadStores(bound, scope, []string{storeIDs["org-a"]}))
			} else {
				require.Empty(t, stores)
				require.ErrorIs(t, a.ReadStores(bound, scope, []string{storeIDs["org-a"]}), c.ErrForbidden)
			}
			require.ErrorIs(t, a.ReadStores(bound, scope, []string{storeIDs["org-b"]}), c.ErrForbidden)
			tx := db.Begin()
			require.NoError(t, tx.Error)
			defer tx.Rollback()
			err = a.LockStores(bound, tx, scope, []string{storeIDs["org-a"]})
			if configured {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, c.ErrForbidden)
			}
			require.ErrorIs(t, a.LockStores(bound, tx, scope, []string{storeIDs["org-b"]}), c.ErrForbidden)
			if configured {
				modules["org-a"][role] = nil
				defer func() { modules["org-a"][role] = []string{"overview-stores"} }()
				access, err = a.Current(bound, scope)
				require.NoError(t, err)
				require.False(t, access.StoresRead)
				require.False(t, access.FactsWrite)
			}
		})
	}
}

func (r *resolverFixture) Resolve(_ context.Context, policy httproute.OrganizationAccessPolicy, input workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	if policy != httproute.OrganizationAccessPolicyLiveWrite || input.BearerToken != "fixture-token" {
		return r.id, errors.New("fresh boundary absent")
	}
	r.calls++
	return r.id, nil
}
func TestCockpitEveryReadUsesFreshOriginalMembership(t *testing.T) {
	original := authidentity.AuthenticatedIdentity{UserID: "actor-a", TenantID: "org-a", EffectiveOrganizationID: "org-a", EffectiveMemberID: "member-a", TokenExpiresAt: time.Now().Add(time.Hour), Roles: []string{"listingkit_admin"}}
	resolver := &resolverFixture{id: original}
	a, err := New(resolver, authz.DefaultListingKitAuthorizer(), &gorm.DB{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(authidentity.WithAuthenticatedIdentity(context.Background(), original), time.Minute)
	defer cancel()
	ctx, err = a.Bind(ctx, "Bearer fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	scope := c.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	access, err := a.Current(ctx, scope)
	if err != nil || !access.GoalsManage {
		t.Fatalf("current manager denied: %+v %v", access, err)
	}
	resolver.id.Roles = []string{"listingkit_viewer"}
	access, err = a.Current(ctx, scope)
	if err != nil || access.GoalsManage || access.GoalsRead {
		t.Fatalf("stale JWT role retained: %+v %v", access, err)
	}
	resolver.id.EffectiveMemberID = "rejoined-member"
	if _, err = a.Current(ctx, scope); !errors.Is(err, c.ErrForbidden) {
		t.Fatalf("rejoined member inherited session: %v", err)
	}
	resolver.id = original
	resolver.id.EffectiveOrganizationID = "org-b"
	if _, err = a.Current(ctx, scope); !errors.Is(err, c.ErrForbidden) {
		t.Fatalf("cross-org response accepted: %v", err)
	}
	if resolver.calls != 4 {
		t.Fatalf("fresh calls %d", resolver.calls)
	}
	if _, err = a.Current(context.Background(), scope); !errors.Is(err, c.ErrForbidden) {
		t.Fatalf("unbound identity accepted: %v", err)
	}
}
