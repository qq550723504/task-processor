package listingkit

import "time"

type GenerationConditionalState struct {
	DeltaToken  string `json:"delta_token,omitempty"`
	ETag        string `json:"etag,omitempty"`
	NotModified bool   `json:"not_modified,omitempty"`
	NoChanges   bool   `json:"no_changes,omitempty"`
}

type AssetGenerationActionImpact struct {
	MatchedItems   int      `json:"matched_items"`
	RetryableItems int      `json:"retryable_items"`
	Platforms      []string `json:"platforms,omitempty"`
	QualityGrades  []string `json:"quality_grades,omitempty"`
	States         []string `json:"states,omitempty"`
}

type GenerationActionAudit struct {
	RequestedActionKey string    `json:"requested_action_key,omitempty"`
	ResolvedActionKey  string    `json:"resolved_action_key,omitempty"`
	ResolutionSource   string    `json:"resolution_source,omitempty"`
	ExecutionPath      string    `json:"execution_path,omitempty"`
	ExecutedAt         time.Time `json:"executed_at,omitempty"`
}
