package sourcehandoff

import (
	"context"
	"fmt"

	"task-processor/internal/listingkit"
	"task-processor/internal/product/sourcing"
)

// GenerateTaskCreator is the narrow ListingKit create boundary needed by
// source-envelope handoff code. listingkit.TaskLifecycleService satisfies it,
// but tests and callers can pass a smaller adapter.
type GenerateTaskCreator interface {
	CreateGenerateTask(context.Context, *listingkit.GenerateRequest) (*listingkit.Task, error)
}

// ListingKitRequestInput carries the source envelope and caller context needed
// to prepare an existing ListingKit GenerateRequest.
type ListingKitRequestInput struct {
	Envelope           sourcing.SourceEnvelope
	TenantID           string
	UserID             string
	Platforms          []string
	Country            string
	Language           string
	SheinStoreID       int64
	TargetCategoryHint string
	Options            *listingkit.GenerateOptions
}

// GenerateRequestFromEnvelope converts a normalized source envelope into the
// existing ListingKit request shape through the neutral catalog and asset fact
// handoff. It does not create tasks, submit marketplace payloads, or make
// ListingKit depend on source-specific crawler DTOs.
func GenerateRequestFromEnvelope(input ListingKitRequestInput) listingkit.GenerateRequest {
	productFacts := sourcing.CatalogProductFactsFromEnvelope(input.Envelope)
	assetFacts := sourcing.AssetFactsFromEnvelope(input.Envelope)
	return listingkit.GenerateRequestFromSourceFacts(listingkit.SourceFactsGenerateRequestInput{
		TenantID:           input.TenantID,
		UserID:             input.UserID,
		Product:            productFacts,
		Assets:             assetFacts,
		Platforms:          input.Platforms,
		Country:            input.Country,
		Language:           input.Language,
		SheinStoreID:       input.SheinStoreID,
		TargetCategoryHint: input.TargetCategoryHint,
		Options:            input.Options,
	})
}

// CreateGenerateTaskFromEnvelope prepares a GenerateRequest from a source
// envelope and then delegates creation to the existing ListingKit task create
// path. It is intentionally only an orchestration handoff.
func CreateGenerateTaskFromEnvelope(ctx context.Context, creator GenerateTaskCreator, input ListingKitRequestInput) (*listingkit.Task, error) {
	if creator == nil {
		return nil, fmt.Errorf("listingkit generate task creator is required")
	}
	request := GenerateRequestFromEnvelope(input)
	return creator.CreateGenerateTask(ctx, &request)
}
