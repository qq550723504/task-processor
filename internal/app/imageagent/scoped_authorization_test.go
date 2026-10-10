package imageagentapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"testing"
	"time"
)

type imageScopeFixture struct {
	scope  collection.Scope
	denied bool
	calls  int
}

func (f *imageScopeFixture) AuthorizeImageExecution(_ context.Context, s collection.Scope) error {
	f.calls++
	f.scope = s
	if f.denied {
		return collection.ErrForbidden
	}
	return nil
}
func TestImageScopeAuthorizationPinsTheOriginalMemberBeforeOwnerRead(t *testing.T) {
	live := &imageScopeFixture{}
	a := ScopedImageAuthorizer{Live: live}
	s := ImagePublicationScopeAuthorizer{Live: live}
	id := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "source"}
	base, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx := authidentity.WithAuthenticatedIdentity(base, authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	require.NoError(t, a.AuthorizeExecution(ctx, id))
	scope, err := s.Authorize(ctx)
	require.NoError(t, err)
	require.Equal(t, sourcing.PublicationScope{OrganizationID: "org", ActorID: "actor"}, scope)
	require.Equal(t, collection.Scope{OrganizationID: "org", ActorID: "actor", MemberID: "member"}, live.scope)
	id.MemberID = "replacement"
	before := live.calls
	require.ErrorIs(t, a.AuthorizeExecution(ctx, id), imageagent.ErrIdentityRequired)
	require.Equal(t, before, live.calls)
	live.denied = true
	_, err = s.Authorize(ctx)
	require.ErrorIs(t, err, collection.ErrForbidden)
	_, err = s.Authorize(base)
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)
}
