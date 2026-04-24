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
	sdsusecase "task-processor/internal/sds/usecase"
)

const sdsDesignSyncTimeout = 75 * time.Second

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
	if imageResult == nil && shouldRunStudioInline(task.Request) {
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
	applySDSTemplateImagesToShein(final.Shein, final.SDSSync, task.Request.ImageURLs)
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
		req.Options.SDS.VariantID > 0
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
		},
		ImageResult: imageResult,
	})
	if err != nil {
		result.SDSSync = &SDSSyncSummary{
			VariantID: options.VariantID,
			Status:    "failed",
			Error:     err.Error(),
		}
		s.applyLocalSDSMockupFallback(ctx, result, firstImageResultURL(imageResult), options)
		if result.SDSSync != nil && result.SDSSync.Status == "local_rendered" {
			markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "SDS render unavailable; local composite used")
		} else {
			markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		}
		appendWarning(result, "sds design sync failed: "+err.Error())
		return
	}

	if syncResult != nil && syncResult.DesignSync != nil && syncResult.DesignSync.DesignResult != nil {
		result.SDSSync = buildSDSSyncSummary(options, syncResult.DesignSync.DesignResult)
	} else {
		result.SDSSync = buildSDSSyncSummary(options, nil)
	}
	if needsLocalSDSMockupFallback(result.SDSSync, options) {
		s.applyLocalSDSMockupFallback(ctx, result, firstImageResultURL(imageResult), options)
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
		s.applyLocalSDSMockupFallback(ctx, result, imageURL, options)
		if result.SDSSync != nil && result.SDSSync.Status == "local_rendered" {
			markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "SDS render unavailable; local composite used")
		} else {
			markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		}
		appendWarning(result, "sds template render failed: "+err.Error())
		return
	}

	result.SDSSync = buildSDSSyncSummary(options, syncResult.DesignResult)
	if needsLocalSDSMockupFallback(result.SDSSync, options) {
		s.applyLocalSDSMockupFallback(ctx, result, imageURL, options)
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
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
