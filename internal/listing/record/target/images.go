package target

import (
	"context"
	"golang.org/x/sync/errgroup"
	"task-processor/internal/marketplace/shein/goods"
	"task-processor/internal/product/asset"
)

type TargetImageReader interface {
	Probe(context.Context, asset.ApprovedAsset, int) (goods.OfficialImageObservation, error)
}

// Probe all unique selected asset/type pairs before validating the full draft.
// This performs only bounded public image reads; remote transforms occur later.
func ProbeTargetImages(ctx context.Context, reader TargetImageReader, draft goods.OfficialDraftInput, inventory asset.ApprovedAssetInventory) ([]goods.OfficialImageObservation, error) {
	if ctx == nil || reader == nil {
		return nil, ErrUnavailable
	}
	type imageKey struct {
		id  string
		typ int
	}
	type selectedImage struct {
		key   imageKey
		value asset.ApprovedAsset
	}
	selected := []selectedImage{}
	seen := map[imageKey]bool{}
	for _, slot := range draft.Images {
		if slot.Type != 1 && slot.Type != 2 && slot.Type != 5 && slot.Type != 6 && slot.Type != 7 {
			continue
		}
		key := imageKey{slot.AssetID, slot.Type}
		if seen[key] {
			continue
		}
		seen[key] = true
		for _, value := range inventory.Assets {
			if value.ID == slot.AssetID {
				selected = append(selected, selectedImage{key, value})
				break
			}
		}
	}
	if len(selected) > 200 {
		return nil, ErrTooLarge
	}
	observations := make([]goods.OfficialImageObservation, len(selected))
	group, bounded := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i, image := range selected {
		group.Go(func() error {
			value, err := reader.Probe(bounded, image.value, image.key.typ)
			if err != nil {
				return err
			}
			if value.AssetID != image.key.id || value.SourceURL != image.value.URL || value.Type != image.key.typ || value.RemoteURL != "" || value.ResponseHash != "" || !goods.ValidImageObservation(value) {
				return ErrNotReady
			}
			observations[i] = value
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return observations, nil
}
