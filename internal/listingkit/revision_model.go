package listingkit

import (
	"time"
)

type ApplyRevisionRequest struct {
	Platform              string                `json:"platform"`
	Actor                 string                `json:"actor,omitempty"`
	Reason                string                `json:"reason,omitempty"`
	RestoreFromRevisionID string                `json:"restore_from_revision_id,omitempty"`
	Amazon                *AmazonRevisionInput  `json:"amazon,omitempty"`
	Shein                 *SheinRevisionInput   `json:"shein,omitempty"`
	Temu                  *TemuRevisionInput    `json:"temu,omitempty"`
	Walmart               *WalmartRevisionInput `json:"walmart,omitempty"`
}

type ListingKitRevisionSummary struct {
	UpdatedAt              time.Time                          `json:"updated_at"`
	UpdatedBy              string                             `json:"updated_by,omitempty"`
	Reason                 string                             `json:"reason,omitempty"`
	Platform               string                             `json:"platform,omitempty"`
	ActionType             string                             `json:"action_type,omitempty"`
	RestoredFromRevisionID string                             `json:"restored_from_revision_id,omitempty"`
	Timeline               *ListingKitRevisionTimelineSummary `json:"timeline,omitempty"`
}

type ListingKitRevisionRecord struct {
	RevisionID             string                             `json:"revision_id,omitempty"`
	UpdatedAt              time.Time                          `json:"updated_at"`
	UpdatedBy              string                             `json:"updated_by,omitempty"`
	Reason                 string                             `json:"reason,omitempty"`
	Platform               string                             `json:"platform,omitempty"`
	ActionType             string                             `json:"action_type,omitempty"`
	RestoredFromRevisionID string                             `json:"restored_from_revision_id,omitempty"`
	Timeline               *ListingKitRevisionTimelineSummary `json:"timeline,omitempty"`
	AppliedChanges         *RevisionDiffPreview               `json:"applied_changes,omitempty"`
	EditorContext          *SheinEditorContext                `json:"editor_context_snapshot,omitempty"`
	StoreResolution        *SheinStoreResolutionSummary       `json:"store_resolution,omitempty"`
}

type ListingKitRevisionTimelineSummary struct {
	Headline     string `json:"headline,omitempty"`
	Badge        string `json:"badge,omitempty"`
	RelationText string `json:"relation_text,omitempty"`
	ChangeCount  int    `json:"change_count,omitempty"`
}

type AmazonRevisionInput struct {
	Title        *string  `json:"title,omitempty"`
	Brand        *string  `json:"brand,omitempty"`
	BulletPoints []string `json:"bullet_points,omitempty"`
	Description  *string  `json:"description,omitempty"`
}

type TemuRevisionInput struct {
	GoodsName        *string           `json:"goods_name,omitempty"`
	ShortDescription *string           `json:"short_description,omitempty"`
	BulletPoints     []string          `json:"bullet_points,omitempty"`
	Images           *PlatformImageSet `json:"images,omitempty"`
	ReviewNotes      []string          `json:"review_notes,omitempty"`
}

type WalmartRevisionInput struct {
	ProductName      *string           `json:"product_name,omitempty"`
	Brand            *string           `json:"brand,omitempty"`
	ShortDescription *string           `json:"short_description,omitempty"`
	LongDescription  *string           `json:"long_description,omitempty"`
	KeyFeatures      []string          `json:"key_features,omitempty"`
	Images           *PlatformImageSet `json:"images,omitempty"`
	ReviewNotes      []string          `json:"review_notes,omitempty"`
}
