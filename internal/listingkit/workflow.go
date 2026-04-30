package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
)

const sdsDesignSyncTimeout = 130 * time.Second

func (s *service) runWorkflow(ctx context.Context, task *Task) (*ListingKitResult, error) {
	result := initResult(task)
	enableAssetGeneration := shouldGenerateAssets(task.Request)

	var canonical *productenrich.CanonicalProduct
	if shouldUseStudioCatalogCanonical(task) {
		canonical = buildStudioFallbackCanonicalProduct(task)
		if canonical == nil {
			return result, fmt.Errorf("failed to build SDS studio product")
		}
		markChildTask(result, "sds_catalog_product", "", string(TaskStatusCompleted), "")
	} else {
		productTask, err := s.productSvc.CreateGenerateTask(productenrich.WithInlineTaskExecution(ctx), toProductGenerateRequest(task))
		if err != nil {
			markChildTask(result, "product_enrich", "", string(TaskStatusFailed), err.Error())
			return result, fmt.Errorf("failed to create product task: %w", err)
		}
		markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusPending), "")

		productJSON, err := s.productSvc.ProcessProduct(ctx, productTask)
		if err != nil {
			markChildTask(result, "product_enrich", productTask.ID, string(TaskStatusFailed), err.Error())
			if !shouldUseStudioProductFallback(task) {
				return result, fmt.Errorf("product enrichment failed: %w", err)
			}
			canonical = buildStudioFallbackCanonicalProduct(task)
			if canonical == nil {
				return result, fmt.Errorf("product enrichment failed: %w", err)
			}
			appendWarning(result, "product enrichment failed, studio fallback canonical product used: "+err.Error())
		} else {
			markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusCompleted), "")
			canonical = productenrich.BuildCanonicalProduct(productTask.Request, productJSON)
		}
	}
	result.CanonicalProduct = canonical
	result.CatalogProduct = catalog.BuildProduct(canonical)
	result.AssetBundle = asset.BuildBundle(canonical, nil)
	result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)

	var imageResult *productimage.ImageProcessResult
	if shouldProcessImages(task.Request) && s.imageSvc != nil {
		imageTask, imageErr := s.imageSvc.CreateProcessTask(productimage.WithInlineTaskExecution(ctx), toImageProcessRequest(task))
		if imageErr != nil {
			markChildTask(result, "product_image", "", string(TaskStatusFailed), imageErr.Error())
			appendWarning(result, "image processing skipped: "+imageErr.Error())
		} else {
			markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusPending), "")
			imageResult, imageErr = s.imageSvc.ProcessImages(ctx, imageTask)
			if imageErr != nil {
				markChildTask(result, "product_image", imageTask.ID, string(TaskStatusFailed), imageErr.Error())
				appendWarning(result, "image processing failed: "+imageErr.Error())
			} else {
				markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusCompleted), "")
				result.ImageAssets = imageResult
				result.AssetBundle = asset.BuildBundle(canonical, imageResult)
				result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
				s.syncSDSDesign(ctx, task, result, imageResult)
			}
		}
	}
	if imageResult == nil && shouldRunStudioInline(task.Request) && shouldRenderSheinSizeImagesWithSDS(task.Request) {
		s.syncSDSDesignFromRemote(ctx, task, result)
	}
	var sdsOptions *SDSSyncOptions
	if task.Request.Options != nil {
		sdsOptions = task.Request.Options.SDS
	}
	if applySDSSyncMetadataToCanonical(canonical, result.SDSSync, sdsOptions) {
		result.CatalogProduct = catalog.BuildProduct(canonical)
		result.AssetBundle = asset.BuildBundle(canonical, nil)
		result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
	}

	inventory := asset.BuildInventory(task.ID, result.AssetBundle)
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, canonical)
	baseRecipes := baselineGenerationRecipes()
	var generationPlan *assetgeneration.Result
	var persistedGenerationTasks []assetgeneration.Task
	if inventory != nil {
		if s.assetRepo != nil {
			if err := s.assetRepo.SaveInventory(ctx, inventory); err != nil {
				appendWarning(result, "asset inventory persistence failed: "+err.Error())
			}
		}
		if enableAssetGeneration && s.assetGenerator != nil && len(baseRecipes) > 0 {
			execution, _ := s.assetGenerator.Execute(ctx, assetgeneration.Request{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Recipes:   append([]assetrecipe.AssetRecipe(nil), baseRecipes...),
			})
			if execution != nil && len(execution.Assets) > 0 {
				inventory.Records = append(inventory.Records, execution.Assets...)
				inventory.Summary = rebuildInventorySummary(inventory)
				result.AssetBundle = rebuildBundleWithGeneratedAssets(result.AssetBundle, execution.Assets)
				if s.assetRepo != nil {
					_ = s.assetRepo.SaveInventory(ctx, inventory)
				}
			}
		}
		if enableAssetGeneration && s.assetGenerator != nil && s.assetRecipeResolver != nil {
			generationPlan, _ = s.assetGenerator.Plan(ctx, assetgeneration.Request{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Recipes:   flattenRecipes(recipesByPlatform),
			})
			if generationPlan != nil && len(generationPlan.Tasks) > 0 {
				dispatchResult, _ := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
					TaskID:    task.ID,
					Product:   result.CatalogProduct,
					Inventory: inventory,
					Tasks:     generationPlan.Tasks,
				})
				if dispatchResult != nil {
					generationPlan.Tasks = cloneGenerationTasks(dispatchResult.Tasks)
					persistedGenerationTasks = mergeGenerationTasks(persistedGenerationTasks, dispatchResult.Tasks)
					if len(dispatchResult.Assets) > 0 {
						inventory.Records = append(inventory.Records, dispatchResult.Assets...)
						inventory.Summary = rebuildInventorySummary(inventory)
						result.AssetBundle = rebuildBundleWithGeneratedAssets(result.AssetBundle, dispatchResult.Assets)
						if s.assetRepo != nil {
							_ = s.assetRepo.SaveInventory(ctx, inventory)
						}
					}
				}
			}
		}
		result.AssetInventorySummary = inventory.Summary
		if result.AssetInventorySummary != nil {
			result.AssetInventorySummary.RecipeCount = len(baseRecipes) + len(flattenRecipes(recipesByPlatform))
		}
	}

	final := s.assembler.Assemble(task, canonical, imageResult)
	final.CatalogProduct = result.CatalogProduct
	final.AssetBundle = result.AssetBundle
	final.AssetInventorySummary = result.AssetInventorySummary
	final.SDSSync = result.SDSSync
	final.ChildTasks = append([]ChildTaskState(nil), result.ChildTasks...)
	s.applyDefaultSheinPricing(final.Shein)
	if shouldUseSDSOfficialImages(task.Request) {
		if !applySelectedSDSImagesToShein(final.Shein, task.Request, task.Request.ImageURLs) {
			applySDSTemplateImagesToShein(final.Shein, final.SDSSync, task.Request.ImageURLs)
		}
		applySheinSizeReferenceImages(final.Shein, resolveSheinSizeReferenceImages(task.Request, final.SDSSync))
	}
	if shouldUseSheinStudioAIImages(task.Request) {
		applySheinStudioAIImagesToShein(final.Shein, task.Request, final.SDSSync)
	}
	if final.Summary == nil {
		final.Summary = &GenerationSummary{}
	}
	final.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, result.Summary.Warnings...))
	if inventory != nil {
		if enableAssetGeneration {
			attachPlatformImageBundles(final, inventory, recipesByPlatform, generationPlan, s.assetBundleBuilder)
		}
		pendingTasks := collectPlatformGenerationTasks(final)
		if enableAssetGeneration && s.assetGenerator != nil && len(pendingTasks) > 0 {
			dispatchResult, _ := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Tasks:     pendingTasks,
			})
			if dispatchResult != nil {
				if len(dispatchResult.Assets) > 0 {
					inventory.Records = append(inventory.Records, dispatchResult.Assets...)
					inventory.Summary = rebuildInventorySummary(inventory)
					result.AssetBundle = rebuildBundleWithGeneratedAssets(result.AssetBundle, dispatchResult.Assets)
					final.AssetBundle = result.AssetBundle
					final.AssetInventorySummary = inventory.Summary
					if s.assetRepo != nil {
						_ = s.assetRepo.SaveInventory(ctx, inventory)
					}
				}
				attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchResult.Tasks}, s.assetBundleBuilder)
				persistedGenerationTasks = mergeGenerationTasks(persistedGenerationTasks, dispatchResult.Tasks)
			}
		}
		decorateListingKitResultGeneration(final, persistedGenerationTasks)
		if s.assetRepo != nil && len(persistedGenerationTasks) > 0 {
			if err := s.assetRepo.SaveGenerationTasks(ctx, task.ID, persistedGenerationTasks); err != nil {
				appendWarning(final, "asset generation task persistence failed: "+err.Error())
			}
		}
	}
	syncAssetRenderPreviews(final)
	return final, nil
}

func (s *service) applyDefaultSheinPricing(pkg *sheinpub.Package) {
	if pkg == nil || (pkg.Pricing != nil && pkg.Pricing.Ready) {
		return
	}
	var overrides map[string]float64
	if pkg.FinalDraft != nil {
		overrides = pkg.FinalDraft.ManualPriceOverrides
	}
	review := buildSheinPricingReview(pkg, s.currentSheinPricingRule(), overrides)
	applySheinPricingReview(pkg, review)
}

func initResult(task *Task) *ListingKitResult {
	if task == nil || task.Request == nil {
		return &ListingKitResult{Summary: &GenerationSummary{}}
	}
	return &ListingKitResult{
		TaskID:    task.ID,
		Status:    string(TaskStatusProcessing),
		Platforms: append([]string(nil), task.Request.Platforms...),
		Country:   task.Request.Country,
		Language:  task.Request.Language,
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
		Summary: &GenerationSummary{
			SourceType: detectSourceType(task.Request),
			ImageCount: len(task.Request.ImageURLs),
		},
	}
}

func toProductGenerateRequest(task *Task) *productenrich.GenerateRequest {
	if task == nil || task.Request == nil {
		return &productenrich.GenerateRequest{}
	}
	return &productenrich.GenerateRequest{
		ImageURLs:  append([]string(nil), task.Request.ImageURLs...),
		Text:       task.Request.Text,
		ProductURL: task.Request.ProductURL,
	}
}

func toImageProcessRequest(task *Task) *productimage.ImageProcessRequest {
	if task == nil || task.Request == nil {
		return &productimage.ImageProcessRequest{}
	}
	marketplace := detectImageMarketplace(task.Request)
	var scene *productimage.SceneGenerationOptions
	if task.Request.Options != nil {
		scene = task.Request.Options.Scene.Clone()
	}
	return &productimage.ImageProcessRequest{
		ProductURL:  task.Request.ProductURL,
		ImageURLs:   append([]string(nil), task.Request.ImageURLs...),
		Text:        task.Request.Text,
		Marketplace: marketplace,
		Country:     task.Request.Country,
		Scene:       scene,
	}
}

func shouldProcessImages(req *GenerateRequest) bool {
	return req != nil && req.Options != nil && req.Options.ProcessImages &&
		(len(req.ImageURLs) > 0 || strings.TrimSpace(req.ProductURL) != "")
}

func shouldGenerateAssets(req *GenerateRequest) bool {
	return req != nil && req.Options != nil && req.Options.ProcessImages
}

func shouldSyncSDS(req *GenerateRequest) bool {
	return req != nil &&
		req.Options != nil &&
		req.Options.SDS != nil &&
		(req.Options.SDS.VariantID > 0 || len(req.Options.SDS.Variants) > 0)
}

func (s *service) syncSDSDesign(ctx context.Context, task *Task, result *ListingKitResult, imageResult *productimage.ImageProcessResult) {
	if s.sdsSyncSvc == nil || !shouldSyncSDS(task.Request) || imageResult == nil {
		return
	}

	options := task.Request.Options.SDS
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")

	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	syncResult, err := s.sdsSyncSvc.SyncFromImageResult(syncCtx, sdsusecase.ImageResultInput{
		Sync: sdsusecase.SyncInput{
			VariantID:        options.VariantID,
			ParentProductID:  options.ParentProductID,
			PrototypeGroupID: options.PrototypeGroupID,
			DesignType:       options.DesignType,
			LayerID:          options.LayerID,
			FitLevel:         options.FitLevel,
			ResizeMode:       options.ResizeMode,
			BlankDesignURL:   options.BlankDesignURL,
		},
		ImageResult: imageResult,
	})
	if err != nil {
		result.SDSSync = &SDSSyncSummary{
			VariantID: options.VariantID,
			Status:    "failed",
			Error:     err.Error(),
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		appendWarning(result, "sds design sync failed: "+err.Error())
		return
	}

	if syncResult != nil && syncResult.DesignSync != nil && syncResult.DesignSync.DesignResult != nil {
		result.SDSSync = buildSDSSyncSummary(options, syncResult.DesignSync.DesignResult)
	} else {
		result.SDSSync = buildSDSSyncSummary(options, nil)
	}
	if needsLocalSDSMockupFallback(result.SDSSync, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSSync, options) {
		result.SDSSync.Status = "failed"
		result.SDSSync.Error = "SDS render returned blank template"
		result.SDSSync.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
}

func (s *service) syncSDSDesignFromRemote(ctx context.Context, task *Task, result *ListingKitResult) {
	if s.sdsSyncSvc == nil || task == nil || task.Request == nil || !shouldRunStudioInline(task.Request) {
		return
	}

	options := task.Request.Options.SDS
	imageURL := strings.TrimSpace(task.Request.ImageURLs[0])
	if imageURL == "" {
		return
	}
	if len(options.Variants) > 0 {
		s.syncSDSDesignVariantsFromRemote(ctx, task, result, imageURL)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")

	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	syncResult, err := s.sdsSyncSvc.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
		Sync: sdsusecase.SyncInput{
			VariantID:        options.VariantID,
			ParentProductID:  options.ParentProductID,
			PrototypeGroupID: options.PrototypeGroupID,
			DesignType:       options.DesignType,
			LayerID:          options.LayerID,
			FitLevel:         options.FitLevel,
			ResizeMode:       options.ResizeMode,
			BlankDesignURL:   options.BlankDesignURL,
		},
		Image: sdsusecase.ImageSource{
			URL:      imageURL,
			FileName: studioSDSMaterialFileName(task),
		},
	})
	if err != nil {
		result.SDSSync = &SDSSyncSummary{
			VariantID: options.VariantID,
			Status:    "failed",
			Error:     err.Error(),
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		appendWarning(result, "sds template render failed: "+err.Error())
		return
	}

	result.SDSSync = buildSDSSyncSummary(options, syncResult.DesignResult)
	if needsLocalSDSMockupFallback(result.SDSSync, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSSync, options) {
		result.SDSSync.Status = "failed"
		result.SDSSync.Error = "SDS render returned blank template"
		result.SDSSync.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
}

func (s *service) syncSDSDesignVariantsFromRemote(ctx context.Context, task *Task, result *ListingKitResult, imageURL string) {
	options := task.Request.Options.SDS
	representatives := representativeSDSVariantsByColor(options.Variants)
	if len(representatives) == 0 {
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")

	primary := representatives[0]
	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()
	syncResult, err := s.sdsSyncSvc.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
		Sync: sdsusecase.SyncInput{
			VariantID:         firstNonZeroInt64(primary.VariantID, options.VariantID),
			RelatedVariantIDs: sdsVariantIDs(representatives),
			ParentProductID:   options.ParentProductID,
			PrototypeGroupID:  firstNonZeroInt64(primary.PrototypeGroupID, options.PrototypeGroupID),
			DesignType:        options.DesignType,
			LayerID:           firstNonEmptyString(primary.LayerID, options.LayerID),
			FitLevel:          options.FitLevel,
			ResizeMode:        options.ResizeMode,
			BlankDesignURL:    firstNonEmptyString(primary.BlankDesignURL, options.BlankDesignURL),
		},
		Image: sdsusecase.ImageSource{
			URL:      imageURL,
			FileName: studioSDSMaterialFileName(task),
		},
	})
	if err != nil {
		result.SDSSync = &SDSSyncSummary{
			VariantID:    primary.VariantID,
			VariantSKU:   strings.TrimSpace(primary.VariantSKU),
			VariantSize:  strings.TrimSpace(primary.Size),
			VariantColor: strings.TrimSpace(primary.Color),
			Status:       "failed",
			Error:        err.Error(),
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		appendWarning(result, "SDS template render failed: "+err.Error())
		return
	}

	summaries := buildSDSVariantSyncSummaries(options, representatives, syncResult.DesignResult)
	result.SDSSync = mergeSDSVariantSyncSummaries(options, summaries)
	if result.SDSSync.Status == "failed" {
		appendWarning(result, result.SDSSync.Error)
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), result.SDSSync.Error)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
}

func sdsVariantIDs(variants []SDSSyncVariantOption) []int64 {
	ids := make([]int64, 0, len(variants))
	seen := map[int64]struct{}{}
	for _, variant := range variants {
		if variant.VariantID <= 0 {
			continue
		}
		if _, ok := seen[variant.VariantID]; ok {
			continue
		}
		seen[variant.VariantID] = struct{}{}
		ids = append(ids, variant.VariantID)
	}
	return ids
}

func representativeSDSVariantsByColor(variants []SDSSyncVariantOption) []SDSSyncVariantOption {
	seen := map[string]struct{}{}
	result := make([]SDSSyncVariantOption, 0, len(variants))
	for _, variant := range variants {
		key := strings.ToLower(strings.TrimSpace(variant.Color))
		if key == "" {
			key = "__default__"
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, variant)
	}
	return result
}

func mergeSDSVariantSyncSummaries(options *SDSSyncOptions, summaries []SDSSyncSummary) *SDSSyncSummary {
	merged := &SDSSyncSummary{Status: "failed", Error: "SDS did not render any selected color variants"}
	if options != nil {
		merged.VariantID = options.VariantID
	}
	var failedColors []string
	var primary *SDSSyncSummary
	for _, summary := range summaries {
		if summary.Status == "failed" || len(summary.MockupImageURLs) == 0 {
			label := strings.TrimSpace(summary.VariantColor)
			if label == "" {
				label = strings.TrimSpace(summary.VariantSKU)
			}
			if label == "" {
				label = "unknown"
			}
			failedColors = append(failedColors, label)
			continue
		}
		if primary == nil {
			copy := summary
			primary = &copy
		}
	}
	if primary != nil {
		*merged = *primary
		merged.VariantResults = append([]SDSSyncSummary(nil), summaries...)
	}
	if len(failedColors) > 0 {
		merged.Status = "failed"
		merged.Error = "SDS render failed for selected color variants: " + strings.Join(uniqueNonEmptyStrings(failedColors), ", ")
		merged.MockupImageURLs = nil
	}
	return merged
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func studioSDSMaterialFileName(task *Task) string {
	if task == nil || strings.TrimSpace(task.ID) == "" {
		return "listingkit-studio-design.png"
	}
	taskID := strings.TrimSpace(task.ID)
	if len(taskID) > 8 {
		taskID = taskID[:8]
	}
	return fmt.Sprintf("listingkit-studio-design-%s.png", taskID)
}

func needsLocalSDSMockupFallback(summary *SDSSyncSummary, options *SDSSyncOptions) bool {
	if summary == nil || options == nil || len(options.MockupImageURLs) == 0 {
		return false
	}
	renderedCount := len(uniqueNonEmptyStrings(summary.MockupImageURLs))
	if renderedCount == 0 {
		return true
	}
	expectedCount := len(uniqueNonEmptyStrings(options.MockupImageURLs))
	return expectedCount > 1 && renderedCount < expectedCount
}

func (s *service) applyLocalSDSMockupFallback(ctx context.Context, result *ListingKitResult, sourceURL string, options *SDSSyncOptions) {
	if result == nil || options == nil || len(options.MockupImageURLs) == 0 {
		return
	}
	rendered, err := s.renderLocalSDSMockups(ctx, localSDSMockupRenderInput{
		SourceURL:        sourceURL,
		MockupImageURLs:  options.MockupImageURLs,
		BlankDesignURL:   options.BlankDesignURL,
		TemplateImageURL: options.TemplateImageURL,
		MaskImageURL:     options.MaskImageURL,
	})
	if err != nil || len(rendered) == 0 {
		if err != nil {
			appendWarning(result, "local SDS mockup render failed: "+err.Error())
		}
		return
	}
	if result.SDSSync == nil {
		result.SDSSync = &SDSSyncSummary{VariantID: options.VariantID}
	}
	result.SDSSync.MockupImageURLs = rendered
	result.SDSSync.Status = "local_rendered"
	if result.SDSSync.Error == "" {
		result.SDSSync.Error = "SDS render unavailable; used local SDS mockup composite"
	}
}

func firstImageResultURL(imageResult *productimage.ImageProcessResult) string {
	if imageResult == nil {
		return ""
	}
	for _, asset := range []*productimage.ImageAsset{
		imageResult.MainImage,
		imageResult.WhiteBgImage,
		imageResult.SubjectCutout,
	} {
		if asset != nil && strings.TrimSpace(asset.URL) != "" {
			return strings.TrimSpace(asset.URL)
		}
		if asset != nil && strings.TrimSpace(asset.SourceURL) != "" {
			return strings.TrimSpace(asset.SourceURL)
		}
	}
	for _, asset := range imageResult.GalleryImages {
		if strings.TrimSpace(asset.URL) != "" {
			return strings.TrimSpace(asset.URL)
		}
		if strings.TrimSpace(asset.SourceURL) != "" {
			return strings.TrimSpace(asset.SourceURL)
		}
	}
	return ""
}

func markChildTask(result *ListingKitResult, kind, taskID, status, errorMsg string) {
	if result == nil {
		return
	}
	for i := range result.ChildTasks {
		if result.ChildTasks[i].Kind == kind {
			result.ChildTasks[i].TaskID = taskID
			result.ChildTasks[i].Status = status
			result.ChildTasks[i].Error = errorMsg
			return
		}
	}
	result.ChildTasks = append(result.ChildTasks, ChildTaskState{Kind: kind, TaskID: taskID, Status: status, Error: errorMsg})
}

func appendWarning(result *ListingKitResult, warning string) {
	if result == nil || result.Summary == nil || strings.TrimSpace(warning) == "" {
		return
	}
	result.Summary.Warnings = append(result.Summary.Warnings, warning)
}

func detectSourceType(req *GenerateRequest) string {
	if req == nil {
		return ""
	}
	switch {
	case strings.TrimSpace(req.ProductURL) != "" && len(req.ImageURLs) > 0 && strings.TrimSpace(req.Text) != "":
		return "mixed"
	case strings.TrimSpace(req.ProductURL) != "":
		return "product_url"
	case len(req.ImageURLs) > 0 && strings.TrimSpace(req.Text) != "":
		return "images_and_text"
	case len(req.ImageURLs) > 0:
		return "images"
	case strings.TrimSpace(req.Text) != "":
		return "text"
	default:
		return "unknown"
	}
}

func detectImageMarketplace(req *GenerateRequest) string {
	if req == nil {
		return "amazon"
	}
	platforms := normalizePlatforms(req.Platforms)
	if len(platforms) == 0 {
		return "amazon"
	}
	return platforms[0]
}
