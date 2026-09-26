package sourcing

import (
	"context"

	"task-processor/internal/product/catalog"
)

// PublishedAcquisition is the Catalog-owned immutable snapshot selected only
// by the durable receipt of one actor-scoped acquisition operation. It is not
// a general product lookup contract.
type PublishedAcquisition struct {
	Result   AcquisitionResult
	Snapshot catalog.PublishedSnapshot
}

// PublishedAcquisitionReader exposes only the authorized exact receipt read,
// not the acquisition producer's mutation methods or concrete application.
type PublishedAcquisitionReader interface {
	ReadPublished(context.Context, string) (PublishedAcquisition, error)
}
