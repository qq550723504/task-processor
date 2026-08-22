package productenrich

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/aicapability"
)

const ScoreCacheIdentityVersion = 1

type ScorePromptIdentity struct {
	PromptKey     string
	PromptVersion string
	PromptScope   string
}

type GovernedScoreExecution interface {
	ScoreCacheIdentity(baseScore, inputHash string) ScoreCacheIdentity
	Invoke(context.Context, aicapability.CacheStatus) (string, error)
	InvokeValidated(context.Context, aicapability.CacheStatus, int, func(string) error) (string, error)
	RecordCacheHit(context.Context, string) error
}

type TextExecutionPreparer interface {
	PrepareText(context.Context, string, ScorePromptIdentity) (GovernedScoreExecution, error)
}

type ImageExecutionPreparer interface {
	PrepareImage(context.Context, string, string, ScorePromptIdentity) (GovernedScoreExecution, error)
}

type ScoreCacheIdentity struct {
	Version              int
	TenantID             string
	Capability           aicapability.Capability
	Operation            aicapability.Operation
	RouteMode            aicapability.RoutingMode
	RouteOutcome         aicapability.RouteOutcome
	ProviderID           string
	ModelID              string
	RoutingKey           string
	PolicyVersion        string
	ConfigurationVersion string
	PromptKey            string
	PromptVersion        string
	PromptScope          string
	BaseScore            string
	InputHash            string
}

func (identity ScoreCacheIdentity) Key() string {
	if identity.Version <= 0 {
		return ""
	}
	payload := struct {
		Version              int    `json:"version"`
		TenantID             string `json:"tenant_id"`
		Capability           string `json:"capability"`
		Operation            string `json:"operation"`
		RouteMode            string `json:"route_mode"`
		RouteOutcome         string `json:"route_outcome"`
		ProviderID           string `json:"provider_id"`
		ModelID              string `json:"model_id"`
		RoutingKey           string `json:"routing_key"`
		PolicyVersion        string `json:"policy_version"`
		ConfigurationVersion string `json:"configuration_version"`
		PromptKey            string `json:"prompt_key"`
		PromptVersion        string `json:"prompt_version"`
		PromptScope          string `json:"prompt_scope"`
		BaseScore            string `json:"base_score"`
		InputHash            string `json:"input_hash"`
	}{
		Version: identity.Version, TenantID: strings.TrimSpace(identity.TenantID), Capability: strings.TrimSpace(string(identity.Capability)),
		Operation: strings.TrimSpace(string(identity.Operation)), RouteMode: strings.TrimSpace(string(identity.RouteMode)),
		RouteOutcome: strings.TrimSpace(string(identity.RouteOutcome)), ProviderID: strings.TrimSpace(identity.ProviderID),
		ModelID: strings.TrimSpace(identity.ModelID), RoutingKey: strings.TrimSpace(identity.RoutingKey),
		PolicyVersion: strings.TrimSpace(identity.PolicyVersion), ConfigurationVersion: strings.TrimSpace(identity.ConfigurationVersion),
		PromptKey: strings.TrimSpace(identity.PromptKey), PromptVersion: strings.TrimSpace(identity.PromptVersion),
		PromptScope: strings.TrimSpace(identity.PromptScope), BaseScore: strings.TrimSpace(identity.BaseScore), InputHash: strings.TrimSpace(identity.InputHash),
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(serialized)
	return fmt.Sprintf("llm_score:governed:v%d:%s", identity.Version, hex.EncodeToString(digest[:]))
}
