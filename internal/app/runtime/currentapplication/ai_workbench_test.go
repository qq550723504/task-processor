package currentapplication

import (
	"testing"

	governed "task-processor/internal/integration/aicapability/einomodel"
)

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
