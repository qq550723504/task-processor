package currentapplication

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/aicapability"
	governed "task-processor/internal/integration/aicapability/einomodel"
	"task-processor/internal/integration/googleinteractions"
	"task-processor/internal/integration/openai"
)

func TestGoogleTitleProvisionIsOfficialVersionedAndNotPlanner(t *testing.T) {
	cfg := agentRuntimeConfig()
	cfg.ProductAgent.Enabled = false
	p := cfg.ProductAgent.TextPolicies["org"]
	p.Profile.ProviderID, p.Profile.AdapterKind, p.Profile.ModelID = "google", "google-interactions", googleinteractions.Model
	p.Profile.AdapterPolicyVersion, p.Profile.UsageMappingVersion = googleinteractions.AdapterPolicyVersion, googleinteractions.UsageMappingVersion
	p.Profile.MaximumInputBytes, p.Profile.MaximumOutputBytes = 128<<10, 256<<10
	p.AdmittedCredentialVersion, p.AdmittedEndpointIdentityDigest = "", ""
	cfg.ProductAgent.TextPolicies["org"] = p
	writerDB := cfg.ProductAgent.Database
	writerDB.User = "credential_writer"
	input := TitleCredentialProvision{Action: "upsert", Consumer: "title", OrganizationID: "org", ClientName: p.Profile.ClientName,
		APIKey: "synthetic-only", BaseURL: googleinteractions.Origin, Model: googleinteractions.Model, APIStyle: "google-interactions", TimeoutSecond: 3, WriterDatabase: writerDB}
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&openai.AIClientCredential{}))
	result, err := SaveTitleCredential(context.Background(), cfg, input, db)
	require.NoError(t, err)
	require.Equal(t, "google", result.ProviderID)
	p.AdmittedCredentialVersion, p.AdmittedEndpointIdentityDigest = result.CredentialVersion, result.EndpointIdentityDigest
	cfg.ProductAgent.TextPolicies["org"] = p
	resolver, err := governed.NewOrganizationRouteResolver(openai.NewOrganizationOnlyCredentialResolver(db),
		map[governed.RouteKey]governed.RoutePolicy{{OrganizationID: "org", Operation: aicapability.OperationProductAgentDecision}: p})
	require.NoError(t, err)
	require.Equal(t, governed.RouteAvailable, resolver.Readiness(context.Background(), aicapability.TextInputIdentity{OrganizationID: "org", Operation: aicapability.OperationProductAgentDecision}))
	for _, endpoint := range []string{"https://other.example.test", "http://127.0.0.1:8080"} {
		bad := input
		bad.BaseURL = endpoint
		// Even a changed trusted digest cannot admit another Google origin.
		p.AdmittedEndpointIdentityDigest = governed.EndpointIdentityDigest(endpoint)
		cfg.ProductAgent.TextPolicies["org"] = p
		_, err := SaveTitleCredential(context.Background(), cfg, bad, db)
		require.Error(t, err)
	}
	row, err := openai.NewOrganizationOnlyCredentialResolver(db).GetCredential(context.Background(), "org", "", input.ClientName)
	require.NoError(t, err)
	require.Equal(t, googleinteractions.Origin, row.BaseURL, "rejected route cannot mutate the credential")
	p.AdmittedEndpointIdentityDigest = governed.EndpointIdentityDigest(googleinteractions.Origin)
	p.Profile.AdapterPolicyVersion = "unqualified"
	cfg.ProductAgent.TextPolicies["org"] = p
	require.Error(t, ValidateTitleCredentialProvision(cfg, input))
	p.Profile.AdapterPolicyVersion = googleinteractions.AdapterPolicyVersion
	cfg.ProductAgent.TextPolicies["org"] = p
	cfg.ProductAgent.Enabled = true
	p.Profile.PromptVersion, p.Profile.OutputSchemaVersion = "ai-workbench-chat-plan-v1", "ai-workbench-plan-decision-v1"
	p.Profile.MaximumCompletionTokens = 4096
	p.Profile.MaximumOutputBytes = 16 << 10
	planningDB := cfg.ProductAgent.Database
	planningDB.User, planningDB.MaxConnections = "ai_workbench_runtime", 4
	cfg.AIWorkbench = &AIWorkbenchConfig{Enabled: true, Database: planningDB, PlanningTextPolicies: map[string]governed.RoutePolicy{"org": p}}
	input.Consumer = "planning"
	require.Error(t, ValidateTitleCredentialProvision(cfg, input), "Google planning credential must not be provisioned")
	require.Error(t, cfg.AIWorkbench.validate(cfg), "Google Planner deployment must not be admitted")
}
