package localagent

import (
	"time"

	"task-processor/internal/product/sourcing"
)

// Actor is the authenticated tenant/user context used by the local-agent API.
// It deliberately contains no 1688 account or target-store identity.
type Actor struct {
	TenantID string
	UserID   string
}

type JobState string

const (
	JobPending   JobState = "pending"
	JobClaimed   JobState = "claimed"
	JobSucceeded JobState = "succeeded"
	JobFailed    JobState = "failed"
)

type FailureKind string

const (
	FailureBrowser    FailureKind = "browser"
	FailureNavigation FailureKind = "navigation"
	FailureChallenge  FailureKind = "challenge"
	FailureExtraction FailureKind = "extraction"
	FailureUnknown    FailureKind = "unknown"
)

type Failure struct {
	Kind    FailureKind `json:"kind"`
	Message string      `json:"message"`
}

// EnvelopeSummary is the bounded success evidence exposed to the local CLI.
// The full envelope remains an in-process handoff value and is not retained
// or returned over the terminal HTTP response.
type EnvelopeSummary struct {
	SourceKey    string `json:"source_key"`
	SourceURL    string `json:"source_url"`
	ProductID    string `json:"product_id"`
	Title        string `json:"title"`
	AssetCount   int    `json:"asset_count"`
	VariantCount int    `json:"variant_count"`
	SupplierName string `json:"supplier_name"`
	Price        string `json:"price"`
}

// Job is the public server-side state. Execution tokens are intentionally
// omitted and are only returned in Claim.
type Job struct {
	ID              string                   `json:"id"`
	TenantID        string                   `json:"tenant_id"`
	URL             string                   `json:"url"`
	State           JobState                 `json:"state"`
	ExpiresAt       time.Time                `json:"expires_at"`
	LeaseExpiresAt  time.Time                `json:"lease_expires_at,omitempty"`
	Envelope        *sourcing.SourceEnvelope `json:"envelope,omitempty"`
	EnvelopeSummary *EnvelopeSummary         `json:"envelope_summary,omitempty"`
	Failure         *Failure                 `json:"failure,omitempty"`
}

type Claim struct {
	Job            Job    `json:"job"`
	ExecutionToken string `json:"execution_token"`
}
