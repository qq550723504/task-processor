package currentapplication

import (
	"encoding/json"
	"testing"

	governed "task-processor/internal/integration/aicapability/einomodel"
)

func TestAIWorkbenchConversationOnlyNeedsNoModel(t *testing.T) {
	cfg := agentRuntimeConfig()
	cfg.ProductAgent.Enabled = false
	cfg.CommercialOwnerDatabase = nil
	var w AIWorkbenchConfig
	if err := json.Unmarshal([]byte(`{"enabled":true,"conversationOnly":true,"allowedOrganizationIds":["org"]}`), &w); err != nil {
		t.Fatal(err)
	}
	w.Database = cfg.ProductAgent.Database
	w.Database.User = "ai_workbench_runtime"
	if err := w.validate(cfg); err != nil {
		t.Fatalf("model-free conversation management rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		change func(*AIWorkbenchConfig)
	}{
		{"implicit-full", func(v *AIWorkbenchConfig) { v.ConversationOnly = false }},
		{"empty-organizations", func(v *AIWorkbenchConfig) { v.AllowedOrganizationIDs = nil }},
		{"duplicate-organizations", func(v *AIWorkbenchConfig) { v.AllowedOrganizationIDs = []string{"org", "org"} }},
		{"invalid-organization", func(v *AIWorkbenchConfig) { v.AllowedOrganizationIDs = []string{" org"} }},
		{"wrong-owner", func(v *AIWorkbenchConfig) { v.Database.Database = "other" }},
		{"wrong-role", func(v *AIWorkbenchConfig) { v.Database.User = "product_agent_runtime" }},
		{"unbounded-pool", func(v *AIWorkbenchConfig) { v.Database.MaxConnections = 9 }},
		{"model-policy", func(v *AIWorkbenchConfig) { v.PlanningTextPolicies = map[string]governed.RoutePolicy{"org": {}} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := w
			test.change(&copy)
			if copy.validate(cfg) == nil {
				t.Fatal("unsafe conversation configuration admitted")
			}
		})
	}
	cfg.ProductAgent.Enabled = true
	if w.validate(cfg) == nil {
		t.Fatal("conversation-only mode admitted alongside execution")
	}
}

func TestAIWorkbenchRejectsSharedProductAgentRuntimeRole(t *testing.T) {
	cfg := agentRuntimeConfig()
	planning := cfg.ProductAgent.TextPolicies["org"]
	planning.Profile.ClientName = "chat"
	planning.Profile.PromptVersion = "ai-workbench-chat-plan-v1"
	planning.Profile.OutputSchemaVersion = "ai-workbench-plan-decision-v1"
	planning.Profile.MaximumCompletionTokens = 4096
	planning.Profile.MaximumInputBytes = 128 << 10
	cfg.AIWorkbench = &AIWorkbenchConfig{Enabled: true, Database: cfg.ProductAgent.Database,
		PlanningTextPolicies: map[string]governed.RoutePolicy{"org": planning}}
	cfg.AIWorkbench.Database.User = "ai_workbench_runtime"
	if err := cfg.AIWorkbench.validate(cfg); err != nil {
		t.Fatalf("distinct runtime role rejected: %v", err)
	}
	cfg.ProductAgent.Database.User = cfg.AIWorkbench.Database.User
	if err := cfg.AIWorkbench.validate(cfg); err == nil {
		t.Fatal("shared Product Agent and AI Workbench runtime role admitted")
	}
}
