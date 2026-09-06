package imageagentworker

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	openai "task-processor/internal/integration/openai"
	"testing"
)

type organizationAdmissionResolver struct{ calls int }

func (r *organizationAdmissionResolver) ResolveClientConfig(context.Context, string, *openai.ClientConfig) (*openai.ResolvedClientConfig, error) {
	r.calls++
	return nil, nil
}

func TestOrganizationCredentialAdmissionChecksVerifiedScopeBeforeResolver(t *testing.T) {
	raw := &organizationAdmissionResolver{}
	admission := organizationCredentialAdmission{resolver: raw}
	for _, verified := range []authidentity.AuthenticatedIdentity{
		{},
		{TenantID: "A", UserID: "actor", HomeOrganizationID: "A"},
		{TenantID: "B", UserID: "other", EffectiveOrganizationID: "B"},
		{TenantID: "C", UserID: "actor", EffectiveOrganizationID: "C"},
	} {
		ctx := authidentity.WithAuthenticatedIdentity(context.Background(), verified)
		ctx = openai.WithIdentity(ctx, openai.Identity{TenantID: "B", UserID: "actor"})
		_, err := admission.ResolveClientConfig(ctx, "default", nil)
		require.ErrorIs(t, err, openai.ErrClientConfigurationUnavailable)
	}
	require.Zero(t, raw.calls)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "B", UserID: "actor", HomeOrganizationID: "A", EffectiveOrganizationID: "B"})
	ctx = openai.WithIdentity(ctx, openai.Identity{TenantID: "B", UserID: "actor"})
	_, err := admission.ResolveClientConfig(ctx, "default", nil)
	require.NoError(t, err)
	require.Equal(t, 1, raw.calls)
}
