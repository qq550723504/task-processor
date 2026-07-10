package a1688

import (
	"context"
	"fmt"

	"task-processor/internal/listingkit"
	"task-processor/internal/product/sourcehandoff"
	"task-processor/internal/product/sourcing"
)

// ListingKitTaskInput carries one normalized 1688 crawler result plus the
// caller/task context needed to prepare the existing ListingKit create request.
type ListingKitTaskInput struct {
	Source sourcing.Alibaba1688SourceEnvelopeInput

	TenantID           string
	UserID             string
	Platforms          []string
	Country            string
	Language           string
	SheinStoreID       int64
	TargetCategoryHint string
	Options            *listingkit.GenerateOptions
}

// ListingKitTaskHandoff is the prepared handoff payload for one 1688 item. It
// exposes the envelope and request for logging/review before task creation.
type ListingKitTaskHandoff struct {
	Envelope sourcing.SourceEnvelope
	Request  listingkit.GenerateRequest
}

// PrepareListingKitTaskHandoff normalizes a 1688 result into a source envelope
// and existing ListingKit GenerateRequest without creating a task.
func PrepareListingKitTaskHandoff(input ListingKitTaskInput) (*ListingKitTaskHandoff, error) {
	envelope := sourcing.Alibaba1688SourceEnvelope(input.Source)
	if err := validateAlibaba1688ListingKitEnvelope(envelope); err != nil {
		return &ListingKitTaskHandoff{Envelope: envelope}, err
	}
	request := sourcehandoff.GenerateRequestFromEnvelope(sourcehandoff.ListingKitRequestInput{
		Envelope:           envelope,
		TenantID:           input.TenantID,
		UserID:             input.UserID,
		Platforms:          input.Platforms,
		Country:            input.Country,
		Language:           input.Language,
		SheinStoreID:       input.SheinStoreID,
		TargetCategoryHint: input.TargetCategoryHint,
		Options:            input.Options,
	})
	return &ListingKitTaskHandoff{Envelope: envelope, Request: request}, nil
}

// CreateListingKitTask normalizes one 1688 result, prepares a GenerateRequest,
// and delegates task creation to the existing ListingKit create boundary.
func CreateListingKitTask(ctx context.Context, creator sourcehandoff.GenerateTaskCreator, input ListingKitTaskInput) (*listingkit.Task, *ListingKitTaskHandoff, error) {
	if creator == nil {
		return nil, nil, fmt.Errorf("listingkit generate task creator is required")
	}
	handoff, err := PrepareListingKitTaskHandoff(input)
	if err != nil {
		return nil, handoff, err
	}
	task, err := creator.CreateGenerateTask(ctx, &handoff.Request)
	if err != nil {
		return nil, handoff, err
	}
	return task, handoff, nil
}

func validateAlibaba1688ListingKitEnvelope(envelope sourcing.SourceEnvelope) error {
	for _, warning := range envelope.Warnings {
		switch warning.Code {
		case "missing_product", "source_error", "missing_source_id":
			if warning.Message != "" {
				return fmt.Errorf("1688 source cannot create listingkit task: %s", warning.Message)
			}
			return fmt.Errorf("1688 source cannot create listingkit task: %s", warning.Code)
		}
	}
	if envelope.ProductCandidate.Title == "" && len(envelope.AssetCandidates) == 0 {
		return fmt.Errorf("1688 source cannot create listingkit task: missing title and assets")
	}
	return nil
}
