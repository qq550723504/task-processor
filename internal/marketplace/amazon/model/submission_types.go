package model

import (
	"time"

	amazonapi "task-processor/internal/amazon/api"
)

type AmazonListingExport struct {
	ListingsAPI *AmazonListingsAPIExport `json:"listings_api,omitempty"`
}

type AmazonSubmissionReport struct {
	LastAction       string                  `json:"last_action,omitempty"`
	LastStatus       string                  `json:"last_status,omitempty"`
	LastError        string                  `json:"last_error,omitempty"`
	SubmittedAt      *time.Time              `json:"submitted_at,omitempty"`
	Preview          *AmazonSubmissionRecord `json:"preview,omitempty"`
	PreviewBeforeFix *AmazonSubmissionRecord `json:"preview_before_fix,omitempty"`
	PreviewAfterFix  *AmazonSubmissionRecord `json:"preview_after_fix,omitempty"`
	Create           *AmazonSubmissionRecord `json:"create,omitempty"`
	Update           *AmazonSubmissionRecord `json:"update,omitempty"`
	FixEvaluation    *AmazonFixEvaluation    `json:"fix_evaluation,omitempty"`
	IssueSummary     *AmazonIssueSummary     `json:"issue_summary,omitempty"`
}

type AmazonSubmissionRecord struct {
	Action      string                     `json:"action"`
	Status      string                     `json:"status,omitempty"`
	Error       string                     `json:"error,omitempty"`
	SubmittedAt time.Time                  `json:"submitted_at"`
	Response    *amazonapi.ListingResponse `json:"response,omitempty"`
}

type AmazonListingsAPIExport struct {
	SKU                      string                      `json:"sku"`
	MarketplaceID            string                      `json:"marketplace_id"`
	ProductType              string                      `json:"product_type"`
	Requirements             string                      `json:"requirements"`
	Attributes               map[string]any              `json:"attributes"`
	ValidationPreviewRequest *amazonapi.ListingRequest   `json:"validation_preview_request,omitempty"`
	CreateRequest            *amazonapi.ListingRequest   `json:"create_request,omitempty"`
	UpdateRequest            *amazonapi.ListingRequest   `json:"update_request,omitempty"`
	Patch                    *AmazonListingsPatchPayload `json:"patch,omitempty"`
}

type AmazonListingsPatchPayload struct {
	SKU     string                      `json:"sku"`
	Patches []AmazonListingsPatchAction `json:"patches"`
}

type AmazonListingsPatchAction struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}
