package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

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
const sdsDesignSyncExtraPollCap = 24

func sdsDesignSyncTimeoutForVariantCount(targetCount int) time.Duration {
	if targetCount <= 1 {
		return sdsDesignSyncTimeout
	}
	extraPolls := (targetCount - 1) * 8
	if extraPolls > sdsDesignSyncExtraPollCap {
		extraPolls = sdsDesignSyncExtraPollCap
	}
	return sdsDesignSyncTimeout + time.Duration(extraPolls)*5*time.Second
}

func (s *service) runWorkflow(ctx context.Context, task *Task) (*ListingKitResult, error) {
	result := initResult(task)
	recorder := newWorkflowRecorder(result)
	enableAssetGeneration := shouldGenerateAssets(task.Request)
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/workflow",
		"task_id":   task.ID,
	})

	var canonical *productenrich.CanonicalProduct
	if shouldUseStudioCatalogCanonical(task) {
		stage := recorder.Start("sds_catalog_product", "")
		canonical = buildStudioFallbackCanonicalProduct(task)
		if canonical == nil {
			stage.Fail("sds_catalog_product_failed", "Failed to build SDS studio product", "")
			recorder.FinalizeSummary()
			return result, fmt.Errorf("failed to build SDS studio product")
		}
		markChildTask(result, "sds_catalog_product", "", string(TaskStatusCompleted), "")
		stage.Complete()
	} else {
		if cached, ok, cacheErr := s.getCachedCanonicalProduct(ctx, task); cacheErr != nil {
			log.WithError(cacheErr).Warn("canonical product cache lookup failed; running product enrich")
		} else if ok {
			stage := recorder.Start("product_enrich", "")
			canonical = cached
			markChildTask(result, "product_enrich", "", string(productenrich.TaskStatusCompleted), "")
			stage.Complete()
			log.WithFields(logrus.Fields{
				"title": func() string {
					if canonical == nil {
						return ""
					}
					return canonical.Title
				}(),
			}).Info("reused cached canonical product for listing kit workflow")
		}
		if canonical == nil {
			stage := recorder.Start("product_enrich", "")
			productTask, err := s.productSvc.CreateGenerateTask(productenrich.WithInlineTaskExecution(ctx), toProductGenerateRequest(task))
			if err != nil {
				markChildTask(result, "product_enrich", "", string(TaskStatusFailed), err.Error())
				stage.Fail("product_task_creation_failed", "Product enrichment task creation failed", err.Error())
				recorder.FinalizeSummary()
				return result, fmt.Errorf("failed to create product task: %w", err)
			}
			stage.SetTaskID(productTask.ID)
			markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusPending), "")

			productJSON, err := s.productSvc.ProcessProduct(ctx, productTask)
			if err != nil {
				markChildTask(result, "product_enrich", productTask.ID, string(TaskStatusFailed), err.Error())
				if !shouldUseStudioProductFallback(task) {
					stage.Fail("product_enrich_failed", "Product enrichment failed", err.Error())
					recorder.FinalizeSummary()
					return result, fmt.Errorf("product enrichment failed: %w", err)
				}
				canonical = buildStudioFallbackCanonicalProduct(task)
				if canonical == nil {
					stage.Fail("product_enrich_failed", "Product enrichment failed", err.Error())
					recorder.FinalizeSummary()
					return result, fmt.Errorf("product enrichment failed: %w", err)
				}
				appendWarning(result, "product enrichment failed, studio fallback canonical product used: "+err.Error())
				stage.Degrade("product_enrich_studio_fallback", "Product enrichment failed; studio fallback canonical product used", err.Error())
			} else {
				markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusCompleted), "")
				stage.Complete()
				canonical = productenrich.BuildCanonicalProduct(productTask.Request, productJSON)
				if cacheErr := s.saveCanonicalProductCache(ctx, task, canonical); cacheErr != nil {
					log.WithError(cacheErr).Warn("canonical product cache save failed")
				}
				log.WithFields(logrus.Fields{
					"child_task_id": productTask.ID,
					"title": func() string {
						if canonical == nil {
							return ""
						}
						return canonical.Title
					}(),
				}).Info("product enrichment completed for listing kit workflow")
			}
		}
	}
	result.CanonicalProduct = canonical
	result.CatalogProduct = catalog.BuildProduct(canonical)
	result.AssetBundle = asset.BuildBundle(canonical, nil)
	result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
	log.WithFields(logrus.Fields{
		"has_canonical": canonical != nil,
		"image_count": func() int {
			if canonical == nil {
				return 0
			}
			return len(canonical.Images)
		}(),
		"variant_count": func() int {
			if canonical == nil {
				return 0
			}
			return len(canonical.Variants)
		}(),
	}).Info("canonical product prepared for listing kit workflow")

	var imageResult *productimage.ImageProcessResult
	if shouldProcessImages(task.Request) && s.imageSvc != nil {
		stage := recorder.Start("product_image", "")
		imageTask, imageErr := s.imageSvc.CreateProcessTask(productimage.WithInlineTaskExecution(ctx), toImageProcessRequest(task))
		if imageErr != nil {
			markChildTask(result, "product_image", "", string(TaskStatusFailed), imageErr.Error())
			appendWarning(result, "image processing skipped: "+imageErr.Error())
			stage.Degrade("image_processing_skipped", "Image processing skipped", imageErr.Error())
		} else {
			stage.SetTaskID(imageTask.ID)
			markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusPending), "")
			imageResult, imageErr = s.imageSvc.ProcessImages(ctx, imageTask)
			if imageErr != nil {
				markChildTask(result, "product_image", imageTask.ID, string(TaskStatusFailed), imageErr.Error())
				appendWarning(result, "image processing failed: "+imageErr.Error())
				stage.Degrade("image_processing_failed", "Image processing failed", imageErr.Error())
			} else {
				markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusCompleted), "")
				stage.Complete()
				result.ImageAssets = imageResult
				result.AssetBundle = asset.BuildBundle(canonical, imageResult)
				result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
				s.syncSDSDesign(ctx, task, result, imageResult, recorder)
			}
		}
	}
	if imageResult == nil && shouldRunStudioInline(task.Request) && shouldRenderSheinSizeImagesWithSDS(task.Request) {
		log.Info("starting remote SDS design sync for listing kit workflow")
		s.syncSDSDesignFromRemote(ctx, task, result, recorder)
		log.WithFields(logrus.Fields{
			"sds_status": func() string {
				if result.SDSSync == nil {
					return ""
				}
				return result.SDSSync.Status
			}(),
			"sds_error": func() string {
				if result.SDSSync == nil {
					return ""
				}
				return result.SDSSync.Error
			}(),
		}).Info("finished remote SDS design sync for listing kit workflow")
	}
	var sdsOptions *SDSSyncOptions
	if task.Request.Options != nil {
		sdsOptions = task.Request.Options.SDS
	}
	if applySDSSyncMetadataToCanonical(canonical, result.SDSSync, sdsOptions) {
		result.CatalogProduct = catalog.BuildProduct(canonical)
		result.AssetBundle = asset.BuildBundle(canonical, nil)
		result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
		log.Info("applied SDS sync metadata to canonical product")
	}

	inventory := asset.BuildInventory(task.ID, result.AssetBundle)
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, canonical)
	baseRecipes := baselineGenerationRecipes()
	var generationPlan *assetgeneration.Result
	var persistedGenerationTasks []assetgeneration.Task
	if inventory != nil {
		inventoryStage := recorder.Start("asset_inventory", "")
		if s.assetRepo != nil {
			if err := s.assetRepo.SaveInventory(ctx, inventory); err != nil {
				appendWarning(result, "asset inventory persistence failed: "+err.Error())
				inventoryStage.Degrade("asset_inventory_persistence_failed", "Asset inventory persistence failed", err.Error())
			} else {
				inventoryStage.Complete()
			}
		} else {
			inventoryStage.Skip()
		}
		if enableAssetGeneration && s.assetGenerator != nil && len(baseRecipes) > 0 {
			stage := recorder.Start("asset_generation_baseline", "")
			execution, execErr := s.assetGenerator.Execute(ctx, assetgeneration.Request{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Recipes:   append([]assetrecipe.AssetRecipe(nil), baseRecipes...),
			})
			if execErr != nil {
				stage.Degrade("asset_generation_baseline_execute_failed", "Baseline asset generation failed", execErr.Error())
			} else {
				stage.Complete()
			}
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
			stage := recorder.Start("asset_generation_platform", "")
			var planErr error
			generationPlan, planErr = s.assetGenerator.Plan(ctx, assetgeneration.Request{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Recipes:   flattenRecipes(recipesByPlatform),
			})
			if planErr != nil {
				stage.Degrade("asset_generation_platform_plan_failed", "Platform asset generation planning failed", planErr.Error())
			}
			if generationPlan != nil && len(generationPlan.Tasks) > 0 {
				dispatchResult, dispatchErr := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
					TaskID:    task.ID,
					Product:   result.CatalogProduct,
					Inventory: inventory,
					Tasks:     generationPlan.Tasks,
				})
				if dispatchErr != nil {
					stage.Degrade("asset_generation_platform_dispatch_failed", "Platform asset generation dispatch failed", dispatchErr.Error())
				}
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
			if stage.IsRunning() {
				stage.Complete()
			}
		}
		result.AssetInventorySummary = inventory.Summary
		if result.AssetInventorySummary != nil {
			result.AssetInventorySummary.RecipeCount = len(baseRecipes) + len(flattenRecipes(recipesByPlatform))
		}
	}

	log.Info("starting listing kit assembler")
	assemblerStage := recorder.Start("assembler", "")
	final := s.assembler.Assemble(task, canonical, imageResult)
	assemblerStage.Complete()
	log.WithFields(logrus.Fields{
		"has_shein":   final != nil && final.Shein != nil,
		"has_summary": final != nil && final.Summary != nil,
	}).Info("listing kit assembler completed")
	final.CatalogProduct = result.CatalogProduct
	final.CanonicalProduct = canonical
	final.AssetBundle = result.AssetBundle
	final.AssetInventorySummary = result.AssetInventorySummary
	final.SDSSync = result.SDSSync
	final.ChildTasks = append([]ChildTaskState(nil), result.ChildTasks...)
	final.WorkflowStages = append([]WorkflowStage(nil), result.WorkflowStages...)
	final.WorkflowIssues = append([]WorkflowIssue(nil), result.WorkflowIssues...)
	s.applyDefaultSheinPricing(final.Shein)
	if shouldUseSDSOfficialImages(task.Request) {
		if !applySelectedSDSImagesToShein(final.Shein, task.Request, task.Request.ImageURLs) {
			applySDSTemplateImagesToShein(final.Shein, final.SDSSync, task.Request.ImageURLs, sdsOptions)
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
	sheinReviewStage := newWorkflowRecorder(final).Start("shein_review", "")
	applySheinInspectionReviewToSummary(final)
	if final.Summary != nil && final.Summary.NeedsReview {
		for _, reason := range reviewReasonsFromResult(final) {
			newWorkflowRecorder(final).AddIssue(WorkflowIssueSeverityReview, "shein_review", "shein_review_required", reason, "")
		}
	}
	sheinReviewStage.Complete()
	applySheinVariantImageCoverageGuard(task, final.Shein)
	if inventory != nil {
		if enableAssetGeneration {
			attachPlatformImageBundles(final, inventory, recipesByPlatform, generationPlan, s.assetBundleBuilder)
		}
		pendingTasks := collectPlatformGenerationTasks(final)
		if enableAssetGeneration && s.assetGenerator != nil && len(pendingTasks) > 0 {
			deferredStage := newWorkflowRecorder(final).Start("asset_generation_platform", "")
			dispatchResult, dispatchErr := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Tasks:     pendingTasks,
			})
			if dispatchErr != nil {
				deferredStage.Degrade("asset_generation_platform_deferred_dispatch_failed", "Deferred platform asset generation dispatch failed", dispatchErr.Error())
			}
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
			if dispatchErr == nil {
				deferredStage.Complete()
			}
		}
		decorateListingKitResultGeneration(final, persistedGenerationTasks)
		if s.assetRepo != nil && len(persistedGenerationTasks) > 0 {
			if err := s.assetRepo.SaveGenerationTasks(ctx, task.ID, persistedGenerationTasks); err != nil {
				appendWarning(final, "asset generation task persistence failed: "+err.Error())
				newWorkflowRecorder(final).AddIssue(WorkflowIssueSeverityWarning, "asset_generation_platform", "asset_generation_task_persistence_failed", "Asset generation task persistence failed", err.Error())
			}
		}
	}
	newWorkflowRecorder(final).FinalizeSummary()
	log.Info("synchronizing listing kit asset render previews")
	syncAssetRenderPreviews(final)
	log.WithFields(logrus.Fields{
		"task_id":      task.ID,
		"needs_review": final.Summary != nil && final.Summary.NeedsReview,
		"warning_count": func() int {
			if final.Summary == nil {
				return 0
			}
			return final.Summary.WarningCount
		}(),
	}).Info("listing kit workflow finished assembling result")
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

func (s *service) syncSDSDesign(ctx context.Context, task *Task, result *ListingKitResult, imageResult *productimage.ImageProcessResult, recorder *workflowRecorder) {
	if s.sdsSyncSvc == nil || !shouldSyncSDS(task.Request) || imageResult == nil {
		return
	}
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}

	options := task.Request.Options.SDS
	stage := recorder.Start("sds_design_sync", "")
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
		stage.Degrade("sds_design_sync_failed", "SDS design sync failed", err.Error())
		return
	}

	if syncResult != nil && syncResult.DesignSync != nil && syncResult.DesignSync.DesignResult != nil {
		result.SDSSync = buildSDSSyncSummary(options, syncResult.DesignSync.DesignResult)
	} else {
		result.SDSSync = buildSDSSyncSummary(options, nil)
	}
	if needsLocalSDSMockupFallback(result.SDSSync, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSSync, options) {
		result.SDSSync.Status = "failed"
		result.SDSSync.Error = "SDS render returned blank template"
		result.SDSSync.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
		stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	stage.Complete()
}

func (s *service) syncSDSDesignFromRemote(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) {
	if s.sdsSyncSvc == nil || task == nil || task.Request == nil || !shouldRunStudioInline(task.Request) {
		return
	}
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/sds_sync_remote",
		"task_id":   task.ID,
	})

	options := task.Request.Options.SDS
	imageURL := strings.TrimSpace(task.Request.ImageURLs[0])
	if imageURL == "" {
		log.Warn("skipping remote SDS design sync because source image URL is empty")
		return
	}
	if len(options.Variants) > 0 {
		log.WithField("variant_count", len(options.Variants)).Info("starting remote SDS variant design sync")
		s.syncSDSDesignVariantsFromRemote(ctx, task, result, imageURL, recorder)
		log.WithFields(logrus.Fields{
			"sds_status": func() string {
				if result.SDSSync == nil {
					return ""
				}
				return result.SDSSync.Status
			}(),
			"sds_error": func() string {
				if result.SDSSync == nil {
					return ""
				}
				return result.SDSSync.Error
			}(),
		}).Info("finished remote SDS variant design sync")
		return
	}
	stage := recorder.Start("sds_design_sync", "")
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
	log.WithFields(logrus.Fields{
		"variant_id":         options.VariantID,
		"parent_product_id":  options.ParentProductID,
		"prototype_group_id": options.PrototypeGroupID,
	}).Info("starting remote SDS design sync")

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
		stage.Degrade("sds_template_render_failed", "SDS template render failed", err.Error())
		log.WithError(err).Error("remote SDS design sync failed")
		return
	}

	result.SDSSync = buildSDSSyncSummary(options, syncResult.DesignResult)
	if needsLocalSDSMockupFallback(result.SDSSync, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSSync, options) {
		result.SDSSync.Status = "failed"
		result.SDSSync.Error = "SDS render returned blank template"
		result.SDSSync.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
		stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	stage.Complete()
	log.WithFields(logrus.Fields{
		"status":        result.SDSSync.Status,
		"mockup_count":  len(result.SDSSync.MockupImageURLs),
		"variant_count": len(result.SDSSync.VariantResults),
	}).Info("remote SDS design sync completed")
}

func (s *service) syncSDSDesignVariantsFromRemote(ctx context.Context, task *Task, result *ListingKitResult, imageURL string, recorder *workflowRecorder) {
	options := task.Request.Options.SDS
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}
	representatives := representativeSDSVariantsByColor(options.Variants)
	if len(representatives) == 0 {
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")

	summaries := make([]SDSSyncSummary, 0, len(representatives))
	for _, variant := range representatives {
		stage := recorder.Start("sds_design_sync", "")
		stage.SetTaskID(strings.TrimSpace(variant.VariantSKU))
		syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeoutForVariantCount(1))
		syncResult, err := s.sdsSyncSvc.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
			Sync: sdsusecase.SyncInput{
				VariantID:        firstNonZeroInt64(variant.VariantID, options.VariantID),
				ParentProductID:  options.ParentProductID,
				PrototypeGroupID: firstNonZeroInt64(variant.PrototypeGroupID, options.PrototypeGroupID),
				DesignType:       options.DesignType,
				LayerID:          firstNonEmptyString(variant.LayerID, options.LayerID),
				FitLevel:         options.FitLevel,
				ResizeMode:       options.ResizeMode,
				BlankDesignURL:   firstNonEmptyString(variant.BlankDesignURL, options.BlankDesignURL),
			},
			Image: sdsusecase.ImageSource{
				URL:      imageURL,
				FileName: studioSDSMaterialFileName(task),
			},
		})
		cancel()
		if err != nil {
			stage.Degrade("sds_variant_render_failed", "SDS variant render failed", err.Error())
			summaries = append(summaries, SDSSyncSummary{
				VariantID:    variant.VariantID,
				ProductID:    variant.VariantID,
				VariantSKU:   strings.TrimSpace(variant.VariantSKU),
				VariantSize:  strings.TrimSpace(variant.Size),
				VariantColor: strings.TrimSpace(variant.Color),
				Status:       "failed",
				Error:        err.Error(),
			})
			continue
		}
		if syncResult == nil {
			stage.Degrade("sds_variant_render_empty", "SDS variant render returned empty result", "")
			summaries = append(summaries, SDSSyncSummary{
				VariantID:    variant.VariantID,
				ProductID:    variant.VariantID,
				VariantSKU:   strings.TrimSpace(variant.VariantSKU),
				VariantSize:  strings.TrimSpace(variant.Size),
				VariantColor: strings.TrimSpace(variant.Color),
				Status:       "failed",
				Error:        "SDS template render returned empty result",
			})
			continue
		}
		summaries = append(summaries, buildSDSVariantSyncSummaries(options, []SDSSyncVariantOption{variant}, syncResult.DesignResult)...)
		stage.Complete()
	}

	result.SDSSync = mergeSDSVariantSyncSummaries(options, summaries)
	if result.SDSSync.Status == "failed" {
		appendWarning(result, result.SDSSync.Error)
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), result.SDSSync.Error)
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_variant_render_failed", result.SDSSync.Error, "")
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
	// Compatibility shim for legacy task payloads and older UI paths.
	// New workflow state should be recorded through workflowRecorder stages/issues.
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
	// Compatibility shim for legacy summary warnings. New warnings should be
	// represented as workflow issues and surfaced through aggregate counts.
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
