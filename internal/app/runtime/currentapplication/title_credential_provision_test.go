package currentapplication

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "modernc.org/sqlite"
	"task-processor/internal/integration/openai"
)

func TestTitleCredentialProvisionWritesOnlyTheAdmittedOrganizationRow(t *testing.T) {
	cfg := agentRuntimeConfig()
	cfg.ProductAgent.Enabled = false
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&openai.AIClientCredential{}); err != nil {
		t.Fatal(err)
	}
	writer := openai.NewGormCredentialResolver(db)
	if err := writer.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "org", UserID: "member", ClientName: "text", APIKey: "member-key", BaseURL: "https://member.example.test/v1", Model: "member-model", APIStyle: "openai-compatible", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	writerDB := cfg.ProductAgent.Database
	writerDB.User = "title_credential_writer"
	request := TitleCredentialProvision{Action: "upsert", OrganizationID: "org", ClientName: "text", APIKey: "organization-key", BaseURL: "https://grsaiapi.com/v1", Model: "gemini-2.5-flash", APIStyle: "grsai", TimeoutSecond: 3, WriterDatabase: writerDB}
	result, err := SaveTitleCredential(context.Background(), cfg, request, db)
	if err != nil || result.ProviderID != "grsai" || result.Route == nil || result.Route.ModelID != "gemini-2.5-flash" || result.Route.ConfigurationVersion == "" {
		t.Fatalf("provision result = %+v, %v", result, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil || strings.Contains(string(encoded), "organization-key") || strings.Contains(string(encoded), writerDB.Password) {
		t.Fatalf("provision result exposed a secret: %v", err)
	}
	org, err := writer.GetCredential(context.Background(), "org", "", "text")
	if err != nil || org == nil || org.APIKey != "organization-key" || !org.Enabled {
		t.Fatalf("organization row = %+v, %v", org, err)
	}
	member, err := writer.GetCredential(context.Background(), "org", "member", "text")
	if err != nil || member == nil || member.APIKey != "member-key" {
		t.Fatalf("member row changed = %+v, %v", member, err)
	}
	wrong := request
	wrong.WriterDatabase.Database = "other_database"
	if _, err := SaveTitleCredential(context.Background(), cfg, wrong, db); err == nil {
		t.Fatal("writer connection to another database accepted")
	}
	wrong = request
	wrong.OrganizationID = "other-org"
	if _, err := SaveTitleCredential(context.Background(), cfg, wrong, db); err == nil {
		t.Fatal("organization outside allowlist accepted")
	}
	wrong = request
	wrong.WriterDatabase.User = cfg.ProductAgent.Database.User
	if _, err := SaveTitleCredential(context.Background(), cfg, wrong, db); err == nil {
		t.Fatal("serving role used for credential writes")
	}
	wrong = request
	wrong.BaseURL = "https://another-provider.example.test/v1"
	if _, err := SaveTitleCredential(context.Background(), cfg, wrong, db); err == nil {
		t.Fatal("endpoint outside operator admission profile accepted")
	}
	if _, err := SaveTitleCredential(context.Background(), cfg, TitleCredentialProvision{Action: "disable", OrganizationID: "org", ClientName: "image", WriterDatabase: writerDB}, db); err == nil {
		t.Fatal("disable escaped title client scope")
	}
	_, err = SaveTitleCredential(context.Background(), cfg, TitleCredentialProvision{Action: "disable", OrganizationID: "org", ClientName: "text", WriterDatabase: writerDB}, db)
	if err != nil {
		t.Fatal(err)
	}
	org, err = writer.GetCredential(context.Background(), "org", "", "text")
	if err != nil || org == nil || org.Enabled {
		t.Fatalf("disable did not persist = %+v, %v", org, err)
	}
	selected, err := openai.NewOrganizationOnlyCredentialResolver(db).ResolveClientConfig(openai.WithTenantID(context.Background(), "org"), "text", nil)
	if selected != nil || !errors.Is(err, openai.ErrClientConfigurationUnavailable) {
		t.Fatalf("disabled organization fell back to member: %+v, %v", selected, err)
	}
	malformed := request
	malformed.APIStyle = "gemini"
	policy := cfg.ProductAgent.TextPolicies["org"]
	policy.APIStyle = "gemini"
	cfg.ProductAgent.TextPolicies["org"] = policy
	if _, err := SaveTitleCredential(context.Background(), cfg, malformed, db); err == nil {
		t.Fatal("unsupported protocol was reported as provisioned")
	}
	org, err = writer.GetCredential(context.Background(), "org", "", "text")
	if err != nil || org == nil || org.Enabled || org.APIStyle != "grsai" {
		t.Fatalf("failed route resolution committed a credential change: %+v, %v", org, err)
	}
}

func TestTitleCredentialProvisionAdmitsOnlyFrozenGoogleInteractionsRoute(t *testing.T) {
	cfg := agentRuntimeConfig()
	cfg.ProductAgent.Enabled = false
	policy := cfg.ProductAgent.TextPolicies["org"]
	policy.ProviderID, policy.Endpoint, policy.APIStyle = "google", "https://generativelanguage.googleapis.com", "google-interactions"
	policy.OutputLimitField, policy.ThinkingLevel, policy.ReasoningEffort = "max_output_tokens", "low", ""
	policy.OutputWindowTokens = int64(policy.MaximumOutputTokens)
	policy.AdmittedRoute.ProviderID, policy.AdmittedRoute.ModelID = "google", "gemini-3.8-flash"
	cfg.ProductAgent.TextPolicies["org"] = policy
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&openai.AIClientCredential{}); err != nil {
		t.Fatal(err)
	}
	writerDB := cfg.ProductAgent.Database
	writerDB.User = "title_credential_writer"
	request := TitleCredentialProvision{Action: "upsert", OrganizationID: "org", ClientName: "text", APIKey: "synthetic-only", BaseURL: policy.Endpoint, Model: "gemini-3.8-flash", APIStyle: "google-interactions", TimeoutSecond: 3, WriterDatabase: writerDB}
	// The admission route must be the manager's exact effective credential version.
	// An operator policy with an unrelated frozen version does not authorize use.
	if _, err := SaveTitleCredential(context.Background(), cfg, request, db); err != nil {
		t.Fatal(err)
	}
	selected, err := openai.NewOrganizationOnlyCredentialResolver(db).ResolveClientConfig(openai.WithTenantID(context.Background(), "org"), "text", nil)
	if err != nil || selected.Config.APIStyle != "google-interactions" {
		t.Fatalf("Google organization credential missing: %+v %v", selected, err)
	}
	bad := request
	bad.BaseURL = "https://other.example.test"
	if _, err := SaveTitleCredential(context.Background(), cfg, bad, db); err == nil {
		t.Fatal("alternate Google endpoint admitted")
	}
	bad.BaseURL = "http://127.0.0.1:8080"
	policy.Endpoint = bad.BaseURL
	cfg.ProductAgent.TextPolicies["org"] = policy
	if _, err := SaveTitleCredential(context.Background(), cfg, bad, db); err == nil {
		t.Fatal("local Google endpoint admitted by credential provisioning")
	}
	selected, err = openai.NewOrganizationOnlyCredentialResolver(db).ResolveClientConfig(openai.WithTenantID(context.Background(), "org"), "text", nil)
	if err != nil || selected.Config.BaseURL != "https://generativelanguage.googleapis.com" {
		t.Fatalf("rejected local route changed organization credential: %+v %v", selected, err)
	}
}
