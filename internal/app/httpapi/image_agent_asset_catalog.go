package httpapi

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
)

type listingTaskSource interface {
	GetTask(context.Context, string) (*listingkit.Task, error)
}

type listingKitAuthorizedAssetCatalog struct{ tasks listingTaskSource }

func newListingKitAuthorizedAssetCatalog(tasks listingTaskSource) imageagent.AuthorizedAssetCatalog {
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
	assets := sourceAssetsFromTask(task)
	if len(assets) == 0 {
		return imageagent.AssetCatalog{}, fmt.Errorf("business task has no authorized source assets")
	}
	return imageagent.AssetCatalog{Assets: assets}, nil
}

func sourceAssetsFromTask(task *listingkit.Task) []imageagent.AuthorizedAsset {
	bundle := taskAssetBundle(task)
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
			label := "Source image"
			if len(item.Labels) > 0 && strings.TrimSpace(item.Labels[0]) != "" {
				label = strings.TrimSpace(item.Labels[0])
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

func taskAssetBundle(task *listingkit.Task) *asset.Bundle {
	if task == nil || task.Result == nil {
		return nil
	}
	if task.Result.StandardProductSnapshot != nil && task.Result.StandardProductSnapshot.AssetBundle != nil {
		return task.Result.StandardProductSnapshot.AssetBundle
	}
	return task.Result.AssetBundle
}
