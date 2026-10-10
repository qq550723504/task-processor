package imageagentapp

import (
	"context"
	"task-processor/internal/imageagent"
)

// Routing is selected by the server route or the immutable plan. A missing or
// denied owner never causes another source table to be tried.
type ImageSetSourceRouter struct {
	Acquisition, Supply ImageSetSourceReader
}

func (r ImageSetSourceRouter) ReadImageSetSource(ctx context.Context, identity imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	var reader ImageSetSourceReader
	switch input.ContextKind {
	case imageagent.ImageSourceAcquisition:
		reader = r.Acquisition
	case imageagent.ImageSourceSupply:
		reader = r.Supply
	default:
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	if reader == nil {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	prepared, err := reader.ReadImageSetSource(ctx, identity, input)
	if err == nil && prepared.Source.ContextKind != input.ContextKind {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	return prepared, err
}
