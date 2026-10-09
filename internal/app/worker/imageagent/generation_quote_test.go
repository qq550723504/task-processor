package imageagentworker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	openai "task-processor/internal/integration/openai"
)

func TestImageSetQuoteUsesOriginalProviderResolutionWithoutCallingProvider(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	resolver := &generationConfigTestResolver{resolved: &openai.ResolvedClientConfig{CacheKey: "db:1:1:image_gpt_image_2", Config: &openai.ClientConfig{APIKey: "controlled-only", APIStyle: "grsai", Model: "gpt-image-2.5", BaseURL: server.URL, Timeout: time.Minute}}}
	reader := imageGenerationQuoteReader{factory: generationProviderFactory{resolver: resolver, price: config.ImageAgentGenerationConfig{PriceVersion: "price", PointsPerImage: 12}}}
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "source"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	quote, err := reader.ReadImageGenerationQuote(ctx, identity)
	require.NoError(t, err)
	require.True(t, quote.Valid())
	require.EqualValues(t, 12, quote.Points)
	_, metadata, err := reader.factory.resolve(ctx)
	require.NoError(t, err)
	require.Equal(t, metadata.ConfigurationVersion, quote.ConfigurationVersion)
	require.Equal(t, metadata.CredentialReference, quote.CredentialReference)
	identity.MemberID = "another-member"
	_, err = reader.ReadImageGenerationQuote(ctx, identity)
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	_, err = reader.ReadImageGenerationQuote(context.Background(), identity)
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	require.Zero(t, requests)
	_, err = NewImageGenerationQuoteReader(nil, reader.factory.price)
	require.Error(t, err)
}
