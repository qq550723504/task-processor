package review

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type taskStateStore struct{ Store }

func (taskStateStore) FindAgentReviewID(_ context.Context, scope Scope, runID string) (string, bool, error) {
	if scope.Org == "B" && scope.Actor == "configured-user" && runID == "run-own" {
		return "review-own", true, nil
	}
	return "", false, nil
}

func (taskStateStore) Read(_ context.Context, scope Scope, id string) (Record, error) {
	if scope.Org == "B" && scope.Actor == "configured-user" && id == "review-own" {
		return Record{ID: id, Org: "B", Owner: "configured-user", State: "pending"}, nil
	}
	return Record{}, ErrNotFound
}

func TestOwnAgentTaskReviewStateHonorsConfiguredUserReadGrant(t *testing.T) {
	auth, err := authz.NewListingKitAuthorizer([]string{"configured-user"}, nil)
	require.NoError(t, err)
	auth.SetRolePolicyReader(taskStateRoleFixture{})
	service := &Service{store: taskStateStore{}, auth: auth}
	identity := func(user, org string) context.Context {
		return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
			TenantID: org, EffectiveOrganizationID: org, UserID: user,
			Roles: []string{authz.EnterpriseRoleKey(org, 1)}, TokenExpiresAt: time.Now().Add(time.Hour),
		})
	}
	state, found, err := service.FindAgentTaskReviewState(identity("configured-user", "B"), "run-own")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "pending", state)
	_, found, err = service.FindAgentTaskReviewState(identity("configured-user", "B"), "run-other")
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = service.FindAgentTaskReviewState(identity("other-user", "B"), "run-own")
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = service.FindAgentTaskReviewState(identity("configured-user", "A"), "run-own")
	require.NoError(t, err)
	require.False(t, found)
	_, _, err = service.FindAgentReview(identity("configured-user", "B"), "run-own")
	require.ErrorIs(t, err, ErrForbidden, "Task read must not grant Review access")
}

type taskStateRoleFixture struct{}

func (taskStateRoleFixture) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	r := map[string][]string{}
	for _, k := range keys {
		if k == authz.EnterpriseRoleKey(org, 1) {
			r[k] = []string{"tasks"}
		}
	}
	return r, nil
}
