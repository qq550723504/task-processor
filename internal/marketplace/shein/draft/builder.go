// Package draft builds local, incomplete SHEIN drafts without remote resolvers.
package draft

import (
	"context"
	"encoding/json"
	"task-processor/internal/listing/record"
	"task-processor/internal/product/catalog"
	"task-processor/internal/publishing/shein"
)

type Builder struct{}

func (Builder) Build(ctx context.Context, snapshot catalog.ProductSnapshot, input record.Input) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	product := catalog.ProjectCanonical(snapshot)
	// Source imagery is not ApprovedAsset. No source-image fallback is allowed.
	product.Images = nil
	for i := range product.Variants {
		product.Variants[i].Images = nil
	}
	// Background intentionally contains no brand authorization, resolver, or AI
	// context values. Computation is synchronous/bounded; cancellation surrounds it.
	pkg := shein.NewAssembler(shein.AssemblerConfig{}).Build(&shein.BuildRequest{Context: context.Background(), Country: input.Country, Language: input.Language}, product)
	raw, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	if _, err = shein.DecodePersistedPackageStrict(raw); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}
