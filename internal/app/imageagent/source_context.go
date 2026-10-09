package imageagentapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"reflect"
	"sort"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/integration/httpimage"
	productimage "task-processor/internal/product/image"
	"time"
)

type ImageSetSourceReader interface {
	ReadImageSetSource(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error)
}

type ImageSetTargetReader interface {
	ResolveImageSetTarget(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput, imageagent.ImageSetPreparation) (imageagent.ImageTarget, map[string]imageagent.OfficialImagePlacement, error)
}

type SourceByteReader func(context.Context, imageagent.AuthorizedAsset, int64) ([]byte, error)

type ImageSetContextReader struct {
	sources      ImageSetSourceReader
	targets      ImageSetTargetReader
	readBytes    SourceByteReader
	maximumBytes int64
}

func NewImageSetContextReader(sources ImageSetSourceReader, targets ImageSetTargetReader, readBytes SourceByteReader, maximumBytes int64) (*ImageSetContextReader, error) {
	if sources == nil || maximumBytes <= 0 || maximumBytes > productimage.MaxInlineArtifactBytes {
		return nil, imageagent.ErrValidation
	}
	if readBytes == nil {
		client := httpimage.NewPublicImageHTTPClient()
		readBytes = func(ctx context.Context, asset imageagent.AuthorizedAsset, maximum int64) ([]byte, error) {
			return httpimage.Download(ctx, client, asset.URL, maximum)
		}
	}
	return &ImageSetContextReader{sources, targets, readBytes, maximumBytes}, nil
}

func (r *ImageSetContextReader) ResolveImageSet(ctx context.Context, identity imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	if ctx == nil || r == nil || r.sources == nil || r.readBytes == nil {
		return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
	}
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.ScopeProtocol != imageagent.OrganizationScopeProtocol || identity.TenantID == "" || identity.UserID == "" || identity.MemberID == "" || verified.TenantID != identity.TenantID || verified.EffectiveOrganizationID != identity.TenantID || verified.UserID != identity.UserID || verified.EffectiveMemberID != identity.MemberID || input.ContextID == "" || input.ContextID != identity.BusinessTaskID {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	if input.Target.Platform != "product" && r.targets == nil {
		return imageagent.ImageSetPreparation{}, imageagent.ErrCommandBlocked
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	source, err := r.sources.ReadImageSetSource(ctx, identity, input)
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	if source.Source.OperationID != input.ContextID || source.Source.ProductID == "" || source.Source.OriginalPublicationID == "" || source.Source.OriginalVersion == 0 || source.Source.EffectiveVersion == 0 || source.Catalog.ProductContext.ProductID != source.Source.ProductID || source.Catalog.ProductContext.SourceSnapshotVersion != source.Source.EffectiveVersion {
		return imageagent.ImageSetPreparation{}, imageagent.ErrRevisionConflict
	}
	selected := map[string]bool{}
	for _, group := range [][]string{input.SharedOriginalIDs, input.CarouselOriginalIDs, input.DetailOriginalIDs} {
		if len(group) > agentconfig.MaxSetSourceReferences {
			return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
		}
		for _, id := range group {
			if imageagent.ValidateProvenanceAssetID(id) != nil {
				return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
			}
			selected[id] = true
		}
	}
	if len(selected) == 0 {
		return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
	}
	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	byID := map[string]imageagent.AuthorizedAsset{}
	for _, asset := range source.Catalog.Assets {
		if _, duplicate := byID[asset.ID]; duplicate {
			return imageagent.ImageSetPreparation{}, imageagent.ErrRevisionConflict
		}
		byID[asset.ID] = asset
	}
	// Resolve every reference before fetching any URL. The browser supplies IDs.
	for _, id := range ids {
		asset, ok := byID[id]
		if !ok || asset.Type != imageagent.AuthorizedAssetSource {
			return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
		}
		if _, err = httpimage.ValidatePublicHTTPSURL(asset.URL); err != nil {
			return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
		}
	}
	source.Observations = nil
	assets := make([]imageagent.AuthorizedAsset, 0, len(ids))
	var total int64
	for _, id := range ids {
		asset := byID[id]
		content, readErr := r.readBytes(ctx, asset, r.maximumBytes)
		if readErr != nil {
			return imageagent.ImageSetPreparation{}, readErr
		}
		total += int64(len(content))
		if len(content) == 0 || int64(len(content)) > r.maximumBytes || total > agentconfig.MaxSetSourceAggregateBytes {
			return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
		}
		mediaType, width, height, inspectErr := httpimage.InspectGeneratedArtifact(content)
		if inspectErr != nil || width > 10000 || height > 10000 || int64(width)*int64(height) > 20_000_000 {
			return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
		}
		if _, _, err = image.Decode(bytes.NewReader(content)); err != nil {
			return imageagent.ImageSetPreparation{}, imageagent.ErrValidation
		}
		hash := sha256.Sum256(content)
		source.Observations = append(source.Observations, imageagent.ImageSourceObservation{AssetID: id, SHA256: hex.EncodeToString(hash[:]), Bytes: int64(len(content)), Width: width, Height: height, MediaType: mediaType})
		asset.Width, asset.Height = width, height
		assets = append(assets, asset)
	}
	source.Catalog, err = imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: source.Catalog.ProductContext, Assets: assets})
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	source.Source.CatalogHash = source.Catalog.Manifest.Hash
	source.Target = imageagent.ImageTarget{Platform: "product"}
	source.OfficialPlacements = nil
	if input.Target.Platform != "product" {
		source.Target, source.OfficialPlacements, err = r.targets.ResolveImageSetTarget(ctx, identity, input, source)
		if err != nil {
			return imageagent.ImageSetPreparation{}, err
		}
	}
	if err = ctx.Err(); err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	return source, nil
}
func (r *ImageSetContextReader) RevalidateImageSet(ctx context.Context, identity imageagent.ExecutionIdentity, projection imageagent.RunProjection) error {
	if projection.Plan.Set == nil || projection.Run.TenantID != identity.TenantID || projection.Run.UserID != identity.UserID || projection.Run.MemberID != identity.MemberID || projection.Run.BusinessTaskID != identity.BusinessTaskID {
		return imageagent.ErrIdentityRequired
	}
	target := projection.Plan.Set.Target
	input := imageagent.PrepareImageSetInput{ContextID: identity.BusinessTaskID, Target: imageagent.ImageTargetSelection{Platform: target.Platform, StoreID: target.StoreID, Site: target.Site, CategoryID: target.CategoryID}, CarouselOriginalIDs: projection.Plan.SourceAssetIDs, OfficialPlacements: map[string]imageagent.OfficialImagePlacement{}}
	// Reconstruct the two bounded reference groups from immutable slot recipes.
	input.CarouselOriginalIDs = nil
	seen := map[string]bool{}
	expected := map[string]imageagent.ImageSourceObservation{}
	for _, slot := range projection.Plan.Slots {
		if slot.Recipe == nil {
			return imageagent.ErrRevisionConflict
		}
		for _, reference := range slot.Recipe.References {
			if original, ok := expected[reference.AssetID]; ok && original != reference {
				return imageagent.ErrRevisionConflict
			}
			expected[reference.AssetID] = reference
			if !seen[reference.AssetID] {
				seen[reference.AssetID] = true
				if len(input.CarouselOriginalIDs) < agentconfig.MaxSetSourceReferences {
					input.CarouselOriginalIDs = append(input.CarouselOriginalIDs, reference.AssetID)
				} else {
					input.DetailOriginalIDs = append(input.DetailOriginalIDs, reference.AssetID)
				}
			}
		}
		if slot.Recipe.OfficialPlacement != nil {
			input.OfficialPlacements[slot.ID] = *slot.Recipe.OfficialPlacement
		}
	}
	fresh, err := r.ResolveImageSet(ctx, identity, input)
	if err != nil {
		return err
	}
	if fresh.Source != projection.Plan.Set.Source || fresh.Target != target || fresh.Catalog.Manifest.Hash != projection.AssetCatalog.Manifest.Hash || len(fresh.Observations) != len(expected) {
		return imageagent.ErrRevisionConflict
	}
	for _, observation := range fresh.Observations {
		if !reflect.DeepEqual(expected[observation.AssetID], observation) {
			return imageagent.ErrRevisionConflict
		}
	}
	return nil
}
