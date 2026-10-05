package einomodel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
	"task-processor/internal/authidentity"
	governed "task-processor/internal/integration/aicapability/einomodel"
	"task-processor/internal/integration/openai"
)

type readinessCredentialReader struct {
	row *openai.AIClientCredential
	err error
}

func (r readinessCredentialReader) GetCredential(context.Context, string, string, string) (*openai.AIClientCredential, error) {
	return r.row, r.err
}

func TestTitleReadinessPreservesResolverStatus(t *testing.T) {
	row := openai.AIClientCredential{TenantID: "org-1", ClientName: "title", APIKey: "synthetic-only",
		BaseURL: "https://example.org/v1", Model: "model", APIStyle: "openai-compatible", TimeoutSecond: 5, Enabled: true}
	profile := aicapability.ModelProfile{ClientName: "title", ProviderID: "synthetic", AdapterKind: "openai-compatible", ModelID: "model",
		EndpointIdentityDigest: governed.EndpointIdentityDigest(row.BaseURL), CredentialVersion: governed.CredentialVersion(row),
		RoutingPolicyVersion: "route-v1", AdapterPolicyVersion: "adapter-v1", PromptVersion: "prompt-v1", OutputSchemaVersion: "schema-v1",
		UsageMappingVersion: "usage-v1", CostPricingVersion: "cost-v1", PointTariff: aicapability.ModelPointTariff{PriceVersion: "points-v1", InputPointsPerMillionTokens: 1, OutputPointsPerMillionTokens: 1},
		Currency: "USD", InputMicrosPerMillion: 1, OutputMicrosPerMillion: 1, MaximumPromptTokens: 8192, MaximumCompletionTokens: 64,
		MaximumInputBytes: 16 << 10, MaximumOutputBytes: 16 << 10, DeadlineBound: 5 * time.Second}
	for _, test := range []struct {
		name, org string
		reader    readinessCredentialReader
		want      TextRouteReadiness
	}{
		{"policy absent", "org-2", readinessCredentialReader{row: &row}, TextRouteUnavailable},
		{"credential store unavailable", "org-1", readinessCredentialReader{err: errors.New("synthetic store outage")}, TextRouteUnavailable},
		{"credential missing", "org-1", readinessCredentialReader{}, TextRouteNeedsConfiguration},
		{"configured", "org-1", readinessCredentialReader{row: &row}, TextRouteAvailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver, err := governed.NewOrganizationRouteResolver(test.reader, map[governed.RouteKey]governed.RoutePolicy{
				{OrganizationID: "org-1", Operation: aicapability.OperationProductAgentDecision}: {
					Profile: profile, AdmittedCredentialVersion: profile.CredentialVersion, AdmittedEndpointIdentityDigest: profile.EndpointIdentityDigest},
			})
			require.NoError(t, err)
			model := &AgentTextModel{selectProfile: func(ctx context.Context, org string) (aicapability.ModelProfile, error) {
				route, err := resolver.Resolve(ctx, aicapability.TextInputIdentity{OrganizationID: org, Operation: aicapability.OperationProductAgentDecision})
				return route.Profile, err
			}, routeReadiness: func(ctx context.Context, org string) TextRouteReadiness {
				return TextRouteReadiness(resolver.Readiness(ctx, aicapability.TextInputIdentity{OrganizationID: org, Operation: aicapability.OperationProductAgentDecision}))
			}}
			ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
				TenantID: test.org, EffectiveOrganizationID: test.org, UserID: "actor", TokenExpiresAt: time.Now().Add(time.Hour)})
			require.Equal(t, test.want, model.RouteReadinessForVerifiedOrganization(ctx, test.org))
			require.Equal(t, TextRouteUnavailable, model.RouteReadinessForVerifiedOrganization(ctx, "other-org"))
		})
	}
}
