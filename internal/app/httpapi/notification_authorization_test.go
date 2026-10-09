package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
	n "task-processor/internal/notificationcenter"
	"task-processor/internal/organization/membership"
	"task-processor/internal/workbenchcontext"
)

func TestNotificationModuleRejectsSupplyAssetPoolBeforeSchemaAccess(t *testing.T) {
	db := &gorm.DB{Config: &gorm.Config{Dialector: sqlite.Open(":memory:")}}
	_, err := buildNotificationModule(context.Background(), db, &config.Config{}, nil, nil, currentApplicationOptions{
		notifications: 1,
		supplyChain:   &SupplyChainDependencies{AssetDB: db},
	})
	require.ErrorContains(t, err, "notification center cannot share an owner pool")
}

type notificationResolverFailure struct{ err error }

func (r notificationResolverFailure) Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	return authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org", EffectiveMemberID: "member"}, r.err
}

type notificationVerifier struct{ err error }

func (v notificationVerifier) Verify(context.Context, string) (authidentity.AuthenticatedIdentity, error) {
	return authidentity.AuthenticatedIdentity{UserID: "actor", TokenExpiresAt: time.Now().Add(time.Hour)}, v.err
}

func TestInvitationNoticeFactoryPreservesIdentityDependencyFailures(t *testing.T) {
	for _, test := range []struct {
		name                     string
		verification, resolution error
	}{
		{"verification", context.DeadlineExceeded, nil},
		{"resolution", nil, workbenchcontext.ErrAuthorizationDependencyUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			auth := routeAuthDependencies{workbenchVerifier: notificationVerifier{test.verification}, organizationResolver: notificationResolverFailure{test.resolution}}
			factory, err := buildInvitationFactory(MembershipDependencies{ProviderOrigin: "http://127.0.0.1:9000"}, "project", auth, nil, &memberstore.Repository{}, nil, nil)
			require.NoError(t, err)
			req := httptest.NewRequest("GET", "/api/v1/workbench/notifications", nil)
			req.Header.Set("Authorization", "Bearer fixture-token")
			req = req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)}))
			flow, err := factory(req)
			require.NoError(t, err)
			_, _, err = flow.NotificationFacts(req.Context(), false, "", 20)
			require.ErrorIs(t, err, membership.ErrUnavailable)
		})
	}
}

type notificationReadRepository struct{ n.Repository }

func (notificationReadRepository) ReadStates(context.Context, n.Scope, []n.Ref) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (notificationReadRepository) Replay(context.Context, n.Scope, string, string, string) (n.Command, bool, error) {
	return n.Command{}, false, nil
}

func TestWorkbenchNotificationsDistinguishAuthorizationOutageFromDenial(t *testing.T) {
	for _, test := range []struct {
		name, coverage string
		err            error
		exact          bool
	}{
		{"dependency", "UNAVAILABLE", workbenchcontext.ErrAuthorizationDependencyUnavailable, false},
		{"deadline", "UNAVAILABLE", context.DeadlineExceeded, false},
		{"role-policy", "UNAVAILABLE", nil, false},
		{"revoked", "DENIED", workbenchcontext.ErrOrganizationAccessRevoked, true},
		{"denied", "DENIED", workbenchcontext.ErrOrganizationAccessDenied, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)})
			ctx, err := (productReviewCapabilityBinder{now: time.Now}).Bind(ctx, "Bearer fixture-token")
			require.NoError(t, err)
			a := &aiWorkbenchApplication{agent: &productAgentApplication{resolver: notificationResolverFailure{test.err}}}
			service := &n.Service{Repository: notificationReadRepository{}, Sources: a.notificationSources()}
			scope := n.Scope{Realm: "http://identity.local", Subject: "actor", OrganizationID: "org", Category: n.Business}
			list, err := service.List(ctx, scope, n.ListRequest{Filter: "all", Limit: 20})
			require.NoError(t, err)
			require.Equal(t, test.exact, list.Exact)
			require.Len(t, list.Coverage, 2)
			for _, source := range list.Coverage {
				require.Equal(t, test.coverage, source.State)
				require.Equal(t, test.exact, source.Complete)
			}
			if !test.exact {
				_, err = service.PrepareReadAll(ctx, scope, "4841d296-ef14-4c16-8d25-a7667e534feb")
				require.True(t, errors.Is(err, n.ErrUnavailable), "authorization outage cannot establish a partial snapshot")
			}
		})
	}
}
