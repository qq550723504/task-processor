package review

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type unavailableNoticePolicy struct{}

func (unavailableNoticePolicy) Authorize(string, []string, string) bool { return false }
func (unavailableNoticePolicy) IsTenantAdmin(string, []string) bool     { return false }
func (unavailableNoticePolicy) AuthorizeScoped(context.Context, string, string, []string, string) (bool, error) {
	return false, authz.ErrRolePolicyUnavailable
}

func TestNoticeAuthorizationKeepsRolePolicyUnavailable(t *testing.T) {
	service := &Service{auth: unavailableNoticePolicy{}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)})
	_, _, err := service.NotificationFacts(ctx, "", 20)
	require.ErrorIs(t, err, ErrUnavailable)
	_, _, err = service.FindAgentTaskReviewMetadata(ctx, "own-run")
	require.ErrorIs(t, err, ErrUnavailable)
}
