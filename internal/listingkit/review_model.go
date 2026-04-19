package listingkit

import "time"

type GenerationReviewDecision string

const (
	GenerationReviewDecisionApprove GenerationReviewDecision = "approve"
	GenerationReviewDecisionDefer   GenerationReviewDecision = "defer"
)

type GenerationReviewRecord struct {
	TaskID          string                   `json:"task_id,omitempty"`
	Platform        string                   `json:"platform,omitempty"`
	Slot            string                   `json:"slot,omitempty"`
	Capability      string                   `json:"capability,omitempty"`
	Decision        GenerationReviewDecision `json:"decision,omitempty"`
	Status          string                   `json:"status,omitempty"`
	Message         string                   `json:"message,omitempty"`
	ReviewedAt      time.Time                `json:"reviewed_at,omitempty"`
	ReviewedBy      string                   `json:"reviewed_by,omitempty"`
	AssetID         string                   `json:"asset_id,omitempty"`
	AssetRevision   string                   `json:"asset_revision,omitempty"`
	PreviewRevision string                   `json:"preview_revision,omitempty"`
	TaskRevision    string                   `json:"task_revision,omitempty"`
	SourceActionKey string                   `json:"source_action_key,omitempty"`
}

type GenerationReviewSummary struct {
	ApprovedSections      int            `json:"approved_sections"`
	DeferredSections      int            `json:"deferred_sections"`
	ReviewPendingSections int            `json:"review_pending_sections"`
	Platforms             []string       `json:"platforms,omitempty"`
	SectionCounts         map[string]int `json:"section_counts,omitempty"`
}
