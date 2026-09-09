// Package draft builds local, incomplete SHEIN drafts without remote resolvers.
package draft

import (
	"context"
	"encoding/json"
	"task-processor/internal/listing/record"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/canonical"
	"task-processor/internal/publishing/shein"
)

type Builder struct{}

func (Builder) Build(ctx context.Context, snapshot catalog.ProductSnapshot, inventory productasset.ApprovedAssetInventory, input record.Input) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	product := catalog.ProjectCanonical(snapshot)
	// Source imagery is not ApprovedAsset. Replace it completely with the exact
	// version-bound approved inventory supplied by Product/Asset.
	product.Images = make([]canonical.Image, 0, len(inventory.Assets))
	for _, approved := range inventory.Assets {
		product.Images = append(product.Images, canonical.Image{URL: approved.URL, Role: string(approved.Role)})
	}
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
	if len(raw) > shein.MaxPersistedPackageBytes {
		return nil, record.ErrTooLarge
	}
	if _, err = shein.DecodePersistedPackageStrict(raw); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}
