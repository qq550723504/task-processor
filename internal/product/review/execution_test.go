package review

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"testing"
	"time"
)

func TestCandidateExecutionAuthorizationKeepsOriginalOwnerWithoutJWT(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	denied := false
	service := &Service{executionResolver: func(ctx context.Context) (CandidateExecutionScope, error) {
		_, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
		require.False(t, authenticated)
		if denied {
			return CandidateExecutionScope{}, ErrForbidden
		}
		return CandidateExecutionScope{OrganizationID: "org", ActorID: "owner", MemberID: "original-member"}, nil
	}}
	scope, err := service.authorize(ctx, true, false)
	require.NoError(t, err)
	require.Equal(t, Scope{Org: "org", Actor: "owner"}, scope)
	_, err = service.authorize(ctx, true, true)
	require.ErrorIs(t, err, ErrForbidden)
	denied = true
	_, err = service.authorize(ctx, true, false)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = service.authorize(context.Background(), true, false)
	require.ErrorIs(t, err, ErrForbidden)
}
