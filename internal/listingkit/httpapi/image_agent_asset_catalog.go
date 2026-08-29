package httpapi

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
)

// ImageAgentTaskSource is the minimal ListingKit task read port required to
// snapshot run-authorized source assets.
type ImageAgentTaskSource interface {
	GetTask(context.Context, string) (*listingkit.Task, error)
}

type listingKitAuthorizedAssetCatalog struct{ tasks ImageAgentTaskSource }

// NewImageAgentAuthorizedAssetCatalog keeps ListingKit task ownership and
// canonical asset translation inside the ListingKit HTTP adapter boundary.
func NewImageAgentAuthorizedAssetCatalog(tasks ImageAgentTaskSource) imageagent.AuthorizedAssetCatalog {
	return &listingKitAuthorizedAssetCatalog{tasks: tasks}
}

func (c *listingKitAuthorizedAssetCatalog) Resolve(ctx context.Context, scope imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || strings.TrimSpace(scope.TenantID) == "" || identity.TenantID != scope.TenantID || strings.TrimSpace(scope.OwnerUserID) == "" || identity.UserID != scope.OwnerUserID || strings.TrimSpace(scope.BusinessTaskID) == "" {
		return imageagent.AssetCatalog{}, fmt.Errorf("verified task ownership is required")
	}
	if c == nil || c.tasks == nil {
		return imageagent.AssetCatalog{}, fmt.Errorf("listing task source is unavailable")
	}
	task, err := c.tasks.GetTask(ctx, scope.BusinessTaskID)
	if err != nil {
		return imageagent.AssetCatalog{}, fmt.Errorf("load business task: %w", err)
	}
	if task == nil || strings.TrimSpace(task.ID) != strings.TrimSpace(scope.BusinessTaskID) || strings.TrimSpace(task.TenantID) != identity.TenantID || listingkit.ResolveTaskUserID(task) != identity.UserID {
		return imageagent.AssetCatalog{}, fmt.Errorf("business task is not owned by verified tenant")
	}
	return imageAgentCatalogFromTask(task, scope.StyleReferenceIDs)
}

func imageAgentCatalogFromTask(task *listingkit.Task, selectedStyleIDs ...[]string) (imageagent.AssetCatalog, error) {
	if task == nil || task.Result == nil || task.Result.StandardProductSnapshot == nil {
		return imageagent.AssetCatalog{}, fmt.Errorf("business task standard product snapshot is required")
	}
	if len(task.Result.AssetBundlesByTarget) > 0 {
		return imageagent.AssetCatalog{}, fmt.Errorf("%w: target-keyed asset bundles require explicit image-agent target authorization", imageagent.ErrValidation)
	}
	snapshot := task.Result.StandardProductSnapshot
	var styles []string
	if len(selectedStyleIDs) > 0 {
		styles = selectedStyleIDs[0]
	}
	assets, err := authorizedAssetsFromBundle(snapshot.AssetBundle, styles)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	if len(assets) == 0 {
		return imageagent.AssetCatalog{}, fmt.Errorf("business task has no authorized source assets")
	}
	context := imageagent.ProductContextRef{ProductID: strings.TrimSpace(task.ID)}
	if product := snapshot.CatalogProduct; product != nil {
		providerContext := assetgeneration.BuildProductContext(product)
		context.Title = providerContext.Title
		context.ProductType = providerContext.ProductType
		context.Attributes = providerContext.Attributes
	}
	normalized, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{Assets: assets, ProductContext: context})
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	// Keep source material first for callers that render the catalog by role;
	// the service boundary re-normalizes the immutable snapshot before storage.
	sort.SliceStable(normalized.Assets, func(i, j int) bool {
		return normalized.Assets[i].Type == imageagent.AuthorizedAssetSource && normalized.Assets[j].Type == imageagent.AuthorizedAssetStyle
	})
	normalized.Manifest.Hash = imageagent.CatalogSnapshotHash(normalized.Assets, normalized.ProductContext)
	return normalized, nil
}

func authorizedAssetsFromBundle(bundle *asset.Bundle, selectedStyleIDs []string) ([]imageagent.AuthorizedAsset, error) {
	selected := make(map[string]struct{}, len(selectedStyleIDs))
	for _, rawID := range selectedStyleIDs {
		if id := strings.TrimSpace(rawID); id != "" {
			selected[id] = struct{}{}
		}
	}
	assets := sourceAssetsFromBundle(bundle)
	if bundle == nil {
		return assets, nil
	}
	for index := range bundle.Assets {
		item := &bundle.Assets[index]
		if item.Kind == asset.KindSourceImage {
			if _, err := displayLabel(item); err != nil {
				return nil, err
			}
		}
	}
	seen := make(map[string]struct{}, len(assets))
	for _, item := range assets {
		seen[item.ID] = struct{}{}
	}
	for id := range selected {
		if _, isSource := seen[id]; isSource {
			return nil, fmt.Errorf("%w: source asset cannot be selected as a style", imageagent.ErrValidation)
		}
		var found *asset.Asset
		for index := range bundle.Assets {
			if strings.TrimSpace(bundle.Assets[index].ID) == id {
				found = &bundle.Assets[index]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("%w: unknown style asset %q", imageagent.ErrValidation, id)
		}
		if strings.TrimSpace(found.URL) == "" {
			return nil, fmt.Errorf("%w: style asset %q has no URL", imageagent.ErrValidation, id)
		}
		url, err := imageagent.ValidateSafeImageURL(found.URL)
		if err != nil {
			return nil, fmt.Errorf("%w: style asset %q URL is unsafe", imageagent.ErrValidation, id)
		}
		label, err := displayLabel(found)
		if err != nil {
			return nil, err
		}
		assets = append(assets, imageagent.AuthorizedAsset{ID: id, Type: imageagent.AuthorizedAssetStyle, URL: url, SourceURL: url, DisplayURL: url, Label: label, Width: found.Width, Height: found.Height})
	}
	return assets, nil
}

func sourceAssetsFromBundle(bundle *asset.Bundle) []imageagent.AuthorizedAsset {
	if bundle != nil {
		var out []imageagent.AuthorizedAsset
		for _, item := range bundle.Assets {
			if item.Kind != asset.KindSourceImage || strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.URL) == "" {
				continue
			}
			url, err := imageagent.ValidateSafeImageURL(item.URL)
			if err != nil {
				continue
			}
			sourceURL := strings.TrimSpace(item.SourceURL)
			if sourceURL == "" {
				sourceURL = url
			} else if sourceURL, err = imageagent.ValidateSafeImageURL(sourceURL); err != nil {
				continue
			}
			label, err := displayLabel(&item)
			if err != nil {
				continue
			}
			// ProductImage slot execution needs only canonical URLs and dimensions.
			// Task metadata is intentionally not copied into the run authorization
			// snapshot because the canonical asset contract does not classify it as
			// safe provider input.
			out = append(out, imageagent.AuthorizedAsset{ID: strings.TrimSpace(item.ID), Type: imageagent.AuthorizedAssetSource, URL: url, SourceURL: sourceURL, DisplayURL: url, Label: label, Width: item.Width, Height: item.Height})
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func displayLabel(item *asset.Asset) (string, error) {
	label := "Source image"
	if item != nil && len(item.Labels) > 0 && strings.TrimSpace(item.Labels[0]) != "" {
		label = strings.TrimSpace(item.Labels[0])
	}
	if utf8.RuneCountInString(label) > 256 {
		label = string([]rune(label)[:256])
	}
	return label, nil
}
