package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/integration/httpimage"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/catalog"
	productimage "task-processor/internal/product/image"
)

type imageAgentTaskSource interface {
	GetTask(context.Context, string) (*listingkit.Task, error)
}

type imageDimensionResolver interface {
	Resolve(context.Context, string) (int, int, error)
}

type listingKitAuthorizedAssetCatalog struct {
	tasks      imageAgentTaskSource
	dimensions imageDimensionResolver
}

func newImageAgentAuthorizedAssetCatalog(tasks imageAgentTaskSource, resolvers ...imageDimensionResolver) imageagent.AuthorizedAssetCatalog {
	var dimensions imageDimensionResolver
	if len(resolvers) > 0 {
		dimensions = resolvers[0]
	}
	return &listingKitAuthorizedAssetCatalog{tasks: tasks, dimensions: dimensions}
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
	return imageAgentCatalogFromTaskTargetSelectionWithResolver(ctx, task, scope.TargetPlatform, scope.PrimarySourceAssetID, [][]string{scope.StyleReferenceIDs}, c.dimensions)
}

func imageAgentCatalogFromTask(task *listingkit.Task, selectedStyleIDs ...[]string) (imageagent.AssetCatalog, error) {
	return imageAgentCatalogFromTaskTarget(task, "", selectedStyleIDs...)
}

func imageAgentCatalogFromTaskTarget(task *listingkit.Task, targetPlatform string, selectedStyleIDs ...[]string) (imageagent.AssetCatalog, error) {
	return imageAgentCatalogFromTaskTargetSelection(task, targetPlatform, "", selectedStyleIDs...)
}

func imageAgentCatalogFromTaskTargetSelection(task *listingkit.Task, targetPlatform, selectedSourceID string, selectedStyleIDs ...[]string) (imageagent.AssetCatalog, error) {
	return imageAgentCatalogFromTaskTargetSelectionWithResolver(context.Background(), task, targetPlatform, selectedSourceID, selectedStyleIDs, nil)
}

func imageAgentCatalogFromTaskTargetSelectionWithResolver(ctx context.Context, task *listingkit.Task, targetPlatform, selectedSourceID string, selectedStyleIDs [][]string, resolver imageDimensionResolver) (imageagent.AssetCatalog, error) {
	targetPlatform = strings.ToLower(strings.TrimSpace(targetPlatform))
	if targetPlatform != "" && targetPlatform != "all" {
		if task == nil || task.Request == nil {
			return imageagent.AssetCatalog{}, fmt.Errorf("business task platform selection is required")
		}
		selected := len(task.Request.Platforms) == 0 // legacy tasks omitted the field and meant all platforms
		for _, platform := range task.Request.Platforms {
			if strings.ToLower(strings.TrimSpace(platform)) == targetPlatform {
				selected = true
				break
			}
		}
		if !selected {
			return imageagent.AssetCatalog{}, fmt.Errorf("%w: target platform %q is not selected by business task", imageagent.ErrValidation, targetPlatform)
		}
	}
	snapshot, err := taskCatalogSnapshot(task)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	assets := authorizedAssetsFromCatalogImages(snapshot.Images)
	assets, err = selectAuthorizedAssets(assets, selectedSourceID, firstStringSlice(selectedStyleIDs), len(selectedStyleIDs) > 0)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	assets = resolveAuthorizedAssetDimensions(ctx, assets, resolver)
	if len(assets) == 0 {
		return imageagent.AssetCatalog{}, fmt.Errorf("business task has no authorized source assets")
	}
	productKey := strings.TrimSpace(task.Request.ProductKey)
	contextRef := imageagent.ProductContextRef{
		ProductID: productKey, Title: snapshot.Title,
		ProductType:           strings.Join(snapshot.CategoryPath, " / "),
		SourceSnapshotVersion: task.SourceSnapshotVersion,
		Attributes:            catalogAttributes(snapshot.Attributes),
	}
	normalized, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{
		Assets: assets, ProductContext: contextRef,
	})
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	sort.SliceStable(normalized.Assets, func(i, j int) bool {
		return normalized.Assets[i].Type == imageagent.AuthorizedAssetSource && normalized.Assets[j].Type == imageagent.AuthorizedAssetStyle
	})
	normalized.Manifest.Hash = imageagent.CatalogSnapshotHash(normalized.Assets, normalized.ProductContext)
	return normalized, nil
}

func taskCatalogSnapshot(task *listingkit.Task) (*catalog.ProductSnapshot, error) {
	if task == nil || task.Request == nil || task.Result == nil || task.Result.StandardProductSnapshot == nil || task.Result.StandardProductSnapshot.CatalogProduct == nil {
		return nil, fmt.Errorf("business task product snapshot is required")
	}
	return task.Result.StandardProductSnapshot.CatalogProduct, nil
}

func authorizedAssetsFromCatalogImages(images []catalog.Image) []imageagent.AuthorizedAsset {
	return buildAuthorizedAssetsFromCatalogImages(images)
}

func authorizedAssetsFromCatalogImagesWithResolver(ctx context.Context, images []catalog.Image, resolver imageDimensionResolver) []imageagent.AuthorizedAsset {
	return resolveAuthorizedAssetDimensions(ctx, buildAuthorizedAssetsFromCatalogImages(images), resolver)
}

func buildAuthorizedAssetsFromCatalogImages(images []catalog.Image) []imageagent.AuthorizedAsset {
	assets := make([]imageagent.AuthorizedAsset, 0, len(images))
	for index, item := range images {
		url, err := imageagent.ValidateSafeImageURL(item.URL)
		if err != nil {
			continue
		}
		assetType := imageagent.AuthorizedAssetSource
		if strings.EqualFold(strings.TrimSpace(item.Role), "style") {
			assetType = imageagent.AuthorizedAssetStyle
		}
		assets = append(assets, imageagent.AuthorizedAsset{
			ID: "catalog-image-" + strconv.Itoa(index+1), Type: assetType,
			URL: url, SourceURL: url, DisplayURL: url, Label: "Product image",
			Width: item.Width, Height: item.Height,
		})
	}
	return assets
}

const catalogImageDimensionProbeTimeout = 10 * time.Second
const catalogImageDimensionProbeBudget = 30 * time.Second
const catalogImageDimensionProbeMaxCount = 32

func resolveAuthorizedAssetDimensions(ctx context.Context, assets []imageagent.AuthorizedAsset, resolver imageDimensionResolver) []imageagent.AuthorizedAsset {
	if resolver == nil || len(assets) == 0 {
		return assets
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, catalogImageDimensionProbeBudget)
	defer cancel()
	probes := 0
	for index := range assets {
		if assets[index].Width > 0 && assets[index].Height > 0 {
			continue
		}
		if probes >= catalogImageDimensionProbeMaxCount || probeCtx.Err() != nil {
			break
		}
		width, height, err := resolver.Resolve(probeCtx, assets[index].URL)
		probes++
		if err == nil {
			assets[index].Width = width
			assets[index].Height = height
		}
	}
	return assets
}

type publicImageDimensionResolver struct {
	client  *http.Client
	maxSize int64
}

func newPublicImageDimensionResolver() imageDimensionResolver {
	return publicImageDimensionResolver{client: httpimage.NewPublicImageHTTPClient(), maxSize: productimage.MaxInlineArtifactBytes}
}

func (r publicImageDimensionResolver) Resolve(ctx context.Context, rawURL string) (int, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, catalogImageDimensionProbeTimeout)
	defer cancel()
	content, err := httpimage.Download(probeCtx, r.client, rawURL, r.maxSize)
	if err != nil {
		return 0, 0, err
	}
	_, width, height, err := httpimage.InspectGeneratedArtifact(content)
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

func selectAuthorizedAssets(assets []imageagent.AuthorizedAsset, selectedSourceID string, selectedStyleIDs []string, styleSelectionProvided bool) ([]imageagent.AuthorizedAsset, error) {
	selectedSourceID = strings.TrimSpace(selectedSourceID)
	selectedStyles := make(map[string]struct{}, len(selectedStyleIDs))
	for _, id := range selectedStyleIDs {
		if id = strings.TrimSpace(id); id != "" {
			selectedStyles[id] = struct{}{}
		}
	}
	out := make([]imageagent.AuthorizedAsset, 0, len(assets))
	foundSource := selectedSourceID == ""
	foundStyles := make(map[string]struct{}, len(selectedStyles))
	for _, item := range assets {
		if item.Type == imageagent.AuthorizedAssetSource {
			if selectedSourceID == "" || item.ID == selectedSourceID {
				out = append(out, item)
				foundSource = true
			}
			continue
		}
		if !styleSelectionProvided {
			out = append(out, item)
			continue
		}
		if _, ok := selectedStyles[item.ID]; ok {
			out = append(out, item)
			foundStyles[item.ID] = struct{}{}
		}
	}
	if !foundSource {
		return nil, fmt.Errorf("%w: unknown source asset %q", imageagent.ErrValidation, selectedSourceID)
	}
	for id := range selectedStyles {
		if _, ok := foundStyles[id]; !ok {
			return nil, fmt.Errorf("%w: unknown style asset %q", imageagent.ErrValidation, id)
		}
	}
	return out, nil
}

func firstStringSlice(values [][]string) []string {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func catalogAttributes(attributes []catalog.Attribute) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	out := make(map[string]string, len(attributes))
	for _, attribute := range attributes {
		if name := strings.TrimSpace(attribute.Name); name != "" {
			out[name] = strings.TrimSpace(attribute.Value)
		}
	}
	return out
}
