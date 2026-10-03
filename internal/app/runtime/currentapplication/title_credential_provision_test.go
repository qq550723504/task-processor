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
	governed "task-processor/internal/integration/aicapability/einomodel"
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
	request := TitleCredentialProvision{Action: "upsert", Consumer: "title", OrganizationID: "org", ClientName: "text", APIKey: "organization-key", BaseURL: "https://grsaiapi.com/v1", Model: "gemini-2.5-flash", APIStyle: "grsai", TimeoutSecond: 3, WriterDatabase: writerDB}
	result, err := SaveTitleCredential(context.Background(), cfg, request, db)
	if err != nil || result.ProviderID != "grsai" || result.CredentialVersion == "" || result.EndpointIdentityDigest == "" {
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
	if _, err := SaveTitleCredential(context.Background(), cfg, TitleCredentialProvision{Action: "disable", Consumer: "title", OrganizationID: "org", ClientName: "image", WriterDatabase: writerDB}, db); err == nil {
		t.Fatal("disable escaped title client scope")
	}
	_, err = SaveTitleCredential(context.Background(), cfg, TitleCredentialProvision{Action: "disable", Consumer: "title", OrganizationID: "org", ClientName: "text", WriterDatabase: writerDB}, db)
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
	if _, err := SaveTitleCredential(context.Background(), cfg, malformed, db); err == nil {
		t.Fatal("unsupported protocol was reported as provisioned")
	}
	org, err = writer.GetCredential(context.Background(), "org", "", "text")
	if err != nil || org == nil || org.Enabled || org.APIStyle != "grsai" {
		t.Fatalf("failed route resolution committed a credential change: %+v, %v", org, err)
	}
}

func TestPlanningCredentialProvisionUsesItsOwnAdmittedRoute(t *testing.T) {
	cfg := agentRuntimeConfig()
	planning := cfg.ProductAgent.TextPolicies["org"]
	planning.Profile.ClientName = "chat"
	planning.Profile.ProviderID = "anthropic"
	planning.Profile.AdapterKind = "claude-native"
	planning.Profile.ModelID = "claude-fixture"
	planning.Profile.PromptVersion = "ai-workbench-chat-plan-v1"
	planning.Profile.OutputSchemaVersion = "ai-workbench-plan-decision-v1"
	planning.AdmittedEndpointIdentityDigest = governed.EndpointIdentityDigest("https://api.anthropic.com")
	cfg.AIWorkbench = &AIWorkbenchConfig{Enabled: true, Database: cfg.ProductAgent.Database,
		PlanningTextPolicies: map[string]governed.RoutePolicy{"org": planning}}
	cfg.AIWorkbench.Database.User = "ai_workbench_runtime"
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&openai.AIClientCredential{}); err != nil {
		t.Fatal(err)
	}
	writerDB := cfg.ProductAgent.Database
	writerDB.User = "credential_writer"
	input := TitleCredentialProvision{Action: "upsert", Consumer: "planning", OrganizationID: "org", ClientName: "chat",
		APIKey: "synthetic-key", BaseURL: "https://api.anthropic.com", Model: "claude-fixture", APIStyle: "claude-native", TimeoutSecond: 3, WriterDatabase: writerDB}
	result, err := SaveTitleCredential(context.Background(), cfg, input, db)
	if err != nil || result.ProviderID != "anthropic" || result.CredentialVersion == "" {
		t.Fatalf("planning provision = %+v, %v", result, err)
	}
	stored, err := openai.NewOrganizationOnlyCredentialResolver(db).GetCredential(context.Background(), "org", "", "chat")
	if err != nil || stored == nil || stored.APIStyle != "claude-native" {
		t.Fatalf("planning row = %+v, %v", stored, err)
	}
	input.Consumer = "title"
	if _, err := SaveTitleCredential(context.Background(), cfg, input, db); err == nil {
		t.Fatal("planning credential admitted as title route")
	}
}
