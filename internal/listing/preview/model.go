package preview

import "time"

// Preview is the future top-level shell owned by the listing preview domain.
//
// It intentionally starts with only generic preview-shell fields so the new
// package can grow independently from the legacy ListingKit DTO surface.
type Preview struct {
	TaskID              string               `json:"task_id"`
	Status              string               `json:"status"`
	SelectedPlatform    string               `json:"selected_platform,omitempty"`
	Platforms           []string             `json:"platforms,omitempty"`
	NeedsReview         bool                 `json:"needs_review"`
	Attachment          *Attachment          `json:"attachment,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
	CompletedAt         *time.Time           `json:"completed_at,omitempty"`
	Overview            *Header              `json:"overview,omitempty"`
	RevisionHistoryMeta *RevisionHistoryMeta `json:"revision_history_meta,omitempty"`
}

// Header is the generic preview summary block shown above platform-specific
// sections.
type Header struct {
	Country       string         `json:"country,omitempty"`
	Language      string         `json:"language,omitempty"`
	SourceType    string         `json:"source_type,omitempty"`
	ImageCount    int            `json:"image_count,omitempty"`
	VariantCount  int            `json:"variant_count,omitempty"`
	StatusMessage string         `json:"status_message,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	ReviewReasons []string       `json:"review_reasons,omitempty"`
	PlatformCards []PlatformCard `json:"platform_cards,omitempty"`
}

// PlatformCard is the future preview-domain summary card for one marketplace.
type PlatformCard struct {
	Platform              string   `json:"platform"`
	Status                string   `json:"status"`
	Summary               string   `json:"summary,omitempty"`
	NeedsReview           bool     `json:"needs_review"`
	PreviewableItems      int      `json:"previewable_items"`
	ApprovedSections      int      `json:"approved_sections"`
	DeferredSections      int      `json:"deferred_sections"`
	ReviewPendingSections int      `json:"review_pending_sections"`
	PrimaryActionKey      string   `json:"primary_action_key,omitempty"`
	PrimaryCTAKind        string   `json:"primary_cta_kind,omitempty"`
	Warnings              []string `json:"warnings,omitempty"`
}
