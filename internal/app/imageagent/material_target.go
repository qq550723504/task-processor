package imageagentapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"golang.org/x/sync/errgroup"
	"image"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/asset"
	productimage "task-processor/internal/product/image"
	"time"
)

// Original material has no official publishing claim. Its actual artifact is
// still validated by the existing bounded probe before the Asset transaction.
type ImageSetMaterialTargetResolver struct {
	Official           asset.ImageSetTargetResolver
	ReadBytes          SourceByteReader
	ReadGeneratedBytes GeneratedMaterialByteReader
}

func (r ImageSetMaterialTargetResolver) ResolveImageSetTarget(ctx context.Context, source asset.SourceSelection, target *asset.ImageSetTarget, selected []asset.ApprovedAsset) (asset.ImageSetTargetResolution, error) {
	if source.TargetPlatform != "product" {
		if r.Official == nil {
			return asset.ImageSetTargetResolution{}, asset.ErrRepositoryUnavailable
		}
		return r.Official.ResolveImageSetTarget(ctx, source, target, selected)
	}
	if ctx == nil || target != nil || r.ReadBytes == nil || len(selected) == 0 || len(selected) > 40 {
		return asset.ImageSetTargetResolution{}, asset.ErrInvalidApproval
	}
	for _, item := range selected {
		if item.OfficialPlacement != nil {
			return asset.ImageSetTargetResolution{}, asset.ErrInvalidApproval
		}
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	group, probeContext := errgroup.WithContext(bounded)
	group.SetLimit(4)
	for index, item := range selected {
		group.Go(func() error {
			var content []byte
			var err error
			if item.GenerationEvidence != nil {
				if r.ReadGeneratedBytes == nil {
					return asset.ErrInvalidApproval
				}
				content, err = r.ReadGeneratedBytes(probeContext, source, item, productimage.MaxInlineArtifactBytes)
			} else {
				if _, err := imageagent.ValidateSafeImageURL(item.URL); err != nil {
					return asset.ErrInvalidApproval
				}
				content, err = r.ReadBytes(probeContext, imageagent.AuthorizedAsset{ID: item.ID, URL: item.URL}, productimage.MaxInlineArtifactBytes)
			}
			if err != nil || len(content) == 0 || len(content) > productimage.MaxInlineArtifactBytes {
				return asset.ErrInvalidApproval
			}
			configuration, format, err := image.DecodeConfig(bytes.NewReader(content))
			if err != nil || (format != "jpeg" && format != "png" && format != "webp") || configuration.Width <= 0 || configuration.Height <= 0 || configuration.Width > 10000 || configuration.Height > 10000 || int64(configuration.Width)*int64(configuration.Height) > 20_000_000 {
				return asset.ErrInvalidApproval
			}
			if _, _, err = image.Decode(bytes.NewReader(content)); err != nil || probeContext.Err() != nil {
				return asset.ErrInvalidApproval
			}
			hash := sha256.Sum256(content)
			if item.GenerationEvidence != nil && (hex.EncodeToString(hash[:]) != item.GenerationEvidence.ArtifactHash || configuration.Width != item.Width || configuration.Height != item.Height) {
				return asset.ErrApprovalConflict
			}
			if item.SourceApproval == nil && (configuration.Width != item.Width || configuration.Height != item.Height) {
				return asset.ErrApprovalConflict
			}
			if item.SourceApproval != nil {
				selected[index].Width, selected[index].Height = configuration.Width, configuration.Height
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return asset.ImageSetTargetResolution{}, err
	}
	return asset.ImageSetTargetResolution{}, nil
}
