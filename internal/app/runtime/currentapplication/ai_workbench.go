package currentapplication

import (
	"errors"

	governed "task-processor/internal/integration/aicapability/einomodel"
)

// AIWorkbenchConfig uses the Product Agent's logical database with its own
// least-privilege role. Planning policies never borrow title admission.
type AIWorkbenchConfig struct {
	Enabled              bool                            `json:"enabled"`
	Database             DatabaseConfig                  `json:"database"`
	PlanningTextPolicies map[string]governed.RoutePolicy `json:"planningTextPolicies"`
}

func (w *AIWorkbenchConfig) validate(cfg *Config) error {
	if w == nil {
		return nil
	}
	if !w.Enabled {
		return w.Database.validate("aiWorkbench.database")
	}
	if cfg.ProductAgent == nil || !cfg.ProductAgent.Enabled || cfg.CommercialOwnerDatabase == nil ||
		w.Database.validate("aiWorkbench.database") != nil || w.Database.User != "ai_workbench_runtime" ||
		w.Database.User == cfg.ProductAgent.Database.User ||
		w.Database.Host != cfg.ProductAgent.Database.Host || w.Database.Port != cfg.ProductAgent.Database.Port ||
		w.Database.Database != cfg.ProductAgent.Database.Database || w.Database.MaxConnections > 8 ||
		len(w.PlanningTextPolicies) == 0 || len(w.PlanningTextPolicies) > 64 {
		return errors.New("AI Workbench requires its own scoped runtime on Product Agent database")
	}
	allowed := make(map[string]bool, len(cfg.ProductAgent.AllowedOrganizationIDs))
	for _, id := range cfg.ProductAgent.AllowedOrganizationIDs {
		allowed[id] = true
	}
	for org, policy := range w.PlanningTextPolicies {
		profile := policy.ShapeProfile()
		if !allowed[org] || profile.Validate() != nil || profile.PromptVersion != "ai-workbench-chat-plan-v1" ||
			profile.OutputSchemaVersion != "ai-workbench-plan-decision-v1" ||
			profile.Currency != cfg.ProductAgent.Currency ||
			profile.MaximumPromptTokens+profile.MaximumCompletionTokens > cfg.ProductAgent.Tokens ||
			profile.MaximumCompletionTokens > 4096 || profile.MaximumInputBytes > 128<<10 ||
			profile.MaximumOutputBytes > 16<<10 {
			return errors.New("AI Workbench planning policy invalid")
		}
		if _, err := profile.CostFor(profile.MaximumPromptTokens, profile.MaximumCompletionTokens); err != nil {
			return errors.New("AI Workbench planning price invalid")
		}
	}
	return nil
}
