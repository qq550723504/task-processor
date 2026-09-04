package enrichment

import (
	"context"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type CandidateGenerator interface {
	Generate(context.Context, GenerationRequest) (Candidate, error)
}

type GenerationRequest struct {
	Snapshot catalog.ProductSnapshot
	Source   sourcing.SourceEnvelope
	Policy   PolicySnapshot
}
