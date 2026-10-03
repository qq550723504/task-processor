package einomodel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/aicapability"
	"task-processor/internal/integration/openai"
)

func TestOrganizationRouteNeverBorrowsMemberCredential(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:einomodel-org-route?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&openai.AIClientCredential{}))
	writer := openai.NewGormCredentialResolver(db)
	ctx := context.Background()
	member := openai.AIClientCredential{TenantID: "org-1", UserID: "actor-1", ClientName: "planning", APIKey: "member-secret", BaseURL: "https://example.org/v1", Model: "model", APIStyle: "openai-compatible", TimeoutSecond: 5, Enabled: true}
	require.NoError(t, writer.SaveCredential(ctx, member))
	policy := testTextProfile("https://example.org/v1")
	policy.ClientName = "planning"
	policy.ModelID = "model"
	resolver, err := NewOrganizationRouteResolver(writer, map[RouteKey]RoutePolicy{{OrganizationID: "org-1", Operation: aicapability.OperationAIWorkbenchChatPlan}: {
		Profile: policy, AdmittedCredentialVersion: "version", AdmittedEndpointIdentityDigest: endpointDigest("https://example.org/v1"),
	}})
	require.NoError(t, err)
	input := aicapability.TextInputIdentity{OrganizationID: "org-1", ActorID: "actor-1", MemberID: "member-1", Operation: aicapability.OperationAIWorkbenchChatPlan}
	_, err = resolver.Resolve(ctx, input)
	require.ErrorIs(t, err, ErrNotDispatched)

	org := member
	org.UserID, org.APIKey = "", "organization-secret"
	require.NoError(t, writer.SaveCredential(ctx, org))
	row, err := writer.GetCredential(ctx, "org-1", "", "planning")
	require.NoError(t, err)
	require.NotNil(t, row)
	version := CredentialVersion(*row)
	policy.CredentialVersion = version
	resolver, err = NewOrganizationRouteResolver(writer, map[RouteKey]RoutePolicy{{OrganizationID: "org-1", Operation: aicapability.OperationAIWorkbenchChatPlan}: {
		Profile: policy, AdmittedCredentialVersion: version, AdmittedEndpointIdentityDigest: endpointDigest(org.BaseURL),
	}})
	require.NoError(t, err)
	route, err := resolver.Resolve(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "organization-secret", route.APIKey)
	require.Equal(t, version, route.Profile.CredentialVersion)

	row.Enabled = false
	require.NoError(t, writer.SaveCredential(ctx, *row))
	_, err = resolver.Resolve(ctx, input)
	require.ErrorIs(t, err, ErrNotDispatched)
}
