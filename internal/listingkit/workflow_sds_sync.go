package listingkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	listingworkflow "task-processor/internal/listingkit/workflow"
	"task-processor/internal/productimage"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

const sdsDesignSyncTimeout = listingworkflow.SDSDesignSyncTimeout
const sdsDesignSyncExtraPollCap = listingworkflow.SDSDesignSyncExtraPollCap

func sdsDesignSyncTimeoutForVariantCount(targetCount int) time.Duration {
	return listingworkflow.SDSDesignSyncTimeoutForVariantCount(targetCount)
}

func (s *service) syncSDSDesign(ctx context.Context, task *Task, result *ListingKitResult, imageResult *productimage.ImageProcessResult, recorder *workflowRecorder) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil || !shouldSyncSDS(task.Request) || imageResult == nil {
		return
	}
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}

	options := task.Request.Options.SDS
	stage := recorder.Start("sds_design_sync", "")
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())

	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	syncResult, err := syncService.SyncFromImageResult(syncCtx, sdsusecase.ImageResultInput{
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
		result.SDSDesignResult = &SDSSyncSummary{
			VariantID: options.VariantID,
			Status:    "failed",
			Error:     err.Error(),
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		appendWarning(result, "sds design sync failed: "+err.Error())
		finishSDSStageWithError(stage, recorder, "sds_design_sync_failed", "SDS design sync failed", err)
		ensureResultPodExecution(result, task.Request)
		return
	}

	if syncResult != nil && syncResult.DesignSync != nil && syncResult.DesignSync.DesignResult != nil {
		result.SDSDesignResult = buildSDSSyncSummary(options, syncResult.DesignSync.DesignResult)
	} else {
		result.SDSDesignResult = buildSDSSyncSummary(options, nil)
	}
	if needsLocalSDSMockupFallback(result.SDSDesignResult, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSDesignResult, options) {
		result.SDSDesignResult.Status = "failed"
		result.SDSDesignResult.Error = "SDS render returned blank template"
		result.SDSDesignResult.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
		stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
		ensureResultPodExecution(result, task.Request)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	stage.Complete()
	ensureResultPodExecution(result, task.Request)
}

func (s *service) syncSDSDesignFromRemote(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil || task == nil || task.Request == nil || !shouldRunRemoteSDSDesignSync(task.Request) {
		return
	}
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
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
				if result.SDSDesignResult == nil {
					return ""
				}
				return result.SDSDesignResult.Status
			}(),
			"sds_error": func() string {
				if result.SDSDesignResult == nil {
					return ""
				}
				return result.SDSDesignResult.Error
			}(),
		}).Info("finished remote SDS variant design sync")
		return
	}
	stage := recorder.Start("sds_design_sync", "")
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())
	log.WithFields(logrus.Fields{
		"variant_id":         options.VariantID,
		"parent_product_id":  options.ParentProductID,
		"prototype_group_id": options.PrototypeGroupID,
	}).Info("starting remote SDS design sync")

	if syncResult, handled, err := s.syncSDSDesignFromUploadedImagePath(ctx, task, imageURL, sdsusecase.SyncInput{
		VariantID:        options.VariantID,
		ParentProductID:  options.ParentProductID,
		PrototypeGroupID: options.PrototypeGroupID,
		DesignType:       options.DesignType,
		LayerID:          options.LayerID,
		FitLevel:         options.FitLevel,
		ResizeMode:       options.ResizeMode,
		BlankDesignURL:   options.BlankDesignURL,
	}); handled {
		if err != nil {
			result.SDSDesignResult = &SDSSyncSummary{
				VariantID: options.VariantID,
				Status:    "failed",
				Error:     err.Error(),
			}
			markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
			appendWarning(result, "sds template render failed: "+err.Error())
			finishSDSStageWithError(stage, recorder, "sds_template_render_failed", "SDS template render failed", err)
			log.WithError(err).Error("remote SDS design sync failed")
			ensureResultPodExecution(result, task.Request)
			return
		}
		result.SDSDesignResult = buildSDSSyncSummary(options, syncResult.DesignResult)
		if needsLocalSDSMockupFallback(result.SDSDesignResult, options) {
			appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
			recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
		}
		if sdsRenderedLooksBlank(ctx, result.SDSDesignResult, options) {
			result.SDSDesignResult.Status = "failed"
			result.SDSDesignResult.Error = "SDS render returned blank template"
			result.SDSDesignResult.MockupImageURLs = nil
			appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
			stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
			ensureResultPodExecution(result, task.Request)
			return
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
		stage.Complete()
		ensureResultPodExecution(result, task.Request)
		log.WithFields(logrus.Fields{
			"status":        result.SDSDesignResult.Status,
			"mockup_count":  len(result.SDSDesignResult.MockupImageURLs),
			"variant_count": len(result.SDSDesignResult.VariantResults),
		}).Info("remote SDS design sync completed")
		return
	}

	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	syncResult, err := syncService.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
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
		result.SDSDesignResult = &SDSSyncSummary{
			VariantID: options.VariantID,
			Status:    "failed",
			Error:     err.Error(),
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		appendWarning(result, "sds template render failed: "+err.Error())
		finishSDSStageWithError(stage, recorder, "sds_template_render_failed", "SDS template render failed", err)
		log.WithError(err).Error("remote SDS design sync failed")
		ensureResultPodExecution(result, task.Request)
		return
	}

	result.SDSDesignResult = buildSDSSyncSummary(options, syncResult.DesignResult)
	if needsLocalSDSMockupFallback(result.SDSDesignResult, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSDesignResult, options) {
		result.SDSDesignResult.Status = "failed"
		result.SDSDesignResult.Error = "SDS render returned blank template"
		result.SDSDesignResult.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
		stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
		ensureResultPodExecution(result, task.Request)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	stage.Complete()
	ensureResultPodExecution(result, task.Request)
	log.WithFields(logrus.Fields{
		"status":        result.SDSDesignResult.Status,
		"mockup_count":  len(result.SDSDesignResult.MockupImageURLs),
		"variant_count": len(result.SDSDesignResult.VariantResults),
	}).Info("remote SDS design sync completed")
}

func (s *service) syncSDSDesignFromUploadedImagePath(ctx context.Context, task *Task, imageURL string, syncInput sdsusecase.SyncInput) (*sdsworkflow.SyncResult, bool, error) {
	key, ok := uploadedListingKitImageKeyFromURL(imageURL)
	if !ok {
		return nil, false, nil
	}
	return s.syncSDSDesignFromUploadedImageKey(ctx, task, key, syncInput, sdsDesignSyncTimeout)
}

func (s *service) syncSDSDesignFromUploadedImageKey(ctx context.Context, task *Task, key string, syncInput sdsusecase.SyncInput, timeout time.Duration) (*sdsworkflow.SyncResult, bool, error) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil {
		return nil, true, fmt.Errorf("sds sync service is not configured")
	}
	if s.uploadStore == nil {
		return nil, true, fmt.Errorf("uploaded image store is not configured")
	}
	stored, err := s.uploadStore.Open(ctx, key)
	if err != nil {
		return nil, true, err
	}
	data := stored.Data
	if len(data) == 0 {
		data, err = os.ReadFile(stored.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, true, ErrUploadedImageNotFound
			}
			return nil, true, fmt.Errorf("read uploaded image: %w", err)
		}
	}
	tempDir := os.TempDir()
	fileName := strings.TrimSpace(stored.Filename)
	if fileName == "" {
		fileName = studioSDSMaterialFileName(task)
	}
	tempPattern := "listingkit-sds-*-" + filepath.Base(fileName)
	tempFile, err := os.CreateTemp(tempDir, tempPattern)
	if err != nil {
		return nil, true, fmt.Errorf("create temp uploaded image: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return nil, true, fmt.Errorf("write temp uploaded image: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, true, fmt.Errorf("close temp uploaded image: %w", err)
	}
	syncCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	syncResult, err := syncService.SyncFromLocalFile(syncCtx, sdsusecase.LocalFileInput{
		Sync: syncInput,
		File: sdsworkflow.FileSource{
			Path:        tempPath,
			FileName:    filepath.Base(fileName),
			ContentType: strings.TrimSpace(stored.ContentType),
		},
	})
	if err != nil {
		return nil, true, err
	}
	return syncResult, true, nil
}

func (s *service) syncSDSDesignVariantsFromRemote(ctx context.Context, task *Task, result *ListingKitResult, imageURL string, recorder *workflowRecorder) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil {
		return
	}
	options := task.Request.Options.SDS
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}
	representatives := representativeSDSVariantsByColor(options.Variants)
	if len(representatives) == 0 {
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())

	summaries := make([]SDSSyncSummary, 0, len(representatives))
	for _, variant := range representatives {
		stage := recorder.Start("sds_design_sync", "")
		stage.SetTaskID(strings.TrimSpace(variant.VariantSKU))
		syncInput := sdsusecase.SyncInput{
			VariantID:        firstNonZeroInt64(variant.VariantID, options.VariantID),
			ParentProductID:  options.ParentProductID,
			PrototypeGroupID: firstNonZeroInt64(variant.PrototypeGroupID, options.PrototypeGroupID),
			DesignType:       options.DesignType,
			LayerID:          firstNonEmptyString(variant.LayerID, options.LayerID),
			FitLevel:         options.FitLevel,
			ResizeMode:       options.ResizeMode,
			BlankDesignURL:   firstNonEmptyString(variant.BlankDesignURL, options.BlankDesignURL),
		}
		syncResult, err := func() (*sdsworkflow.SyncResult, error) {
			if key, ok := uploadedListingKitImageKeyFromURL(imageURL); ok {
				result, handled, err := s.syncSDSDesignFromUploadedImageKey(ctx, task, key, syncInput, sdsDesignSyncTimeoutForVariantCount(1))
				if handled {
					return result, err
				}
			}
			syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeoutForVariantCount(1))
			defer cancel()
			return syncService.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
				Sync: syncInput,
				Image: sdsusecase.ImageSource{
					URL:      imageURL,
					FileName: studioSDSMaterialFileName(task),
				},
			})
		}()
		if err != nil {
			finishSDSStageWithError(stage, recorder, "sds_variant_render_failed", "SDS variant render failed", err)
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

	result.SDSDesignResult = mergeSDSVariantSyncSummaries(options, summaries)
	if result.SDSDesignResult.Status == "failed" {
		appendWarning(result, result.SDSDesignResult.Error)
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), result.SDSDesignResult.Error)
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_variant_render_failed", result.SDSDesignResult.Error, "")
		ensureResultPodExecution(result, task.Request)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	ensureResultPodExecution(result, task.Request)
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
	var authFailureDetail string
	var primary *SDSSyncSummary
	for _, summary := range summaries {
		if summary.Status == "failed" || len(summary.MockupImageURLs) == 0 {
			if authFailureDetail == "" && isSDSAuthRequiredError(fmt.Errorf("%s", strings.TrimSpace(summary.Error))) {
				authFailureDetail = strings.TrimSpace(summary.Error)
			}
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
	if authFailureDetail != "" {
		merged.Status = "failed"
		merged.Error = sdsAuthRequiredMessage
		merged.MockupImageURLs = nil
		return merged
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

func uploadedListingKitImageKeyFromURL(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}
	const prefix = "/api/v1/listing-kits/uploads/files/"
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimPrefix(trimmed, prefix), true
	}
	const localhostPrefix = "http://localhost:3000/api/v1/listing-kits/uploads/files/"
	if strings.HasPrefix(trimmed, localhostPrefix) {
		return strings.TrimPrefix(trimmed, localhostPrefix), true
	}
	const localhostSecurePrefix = "https://localhost:3000/api/v1/listing-kits/uploads/files/"
	if strings.HasPrefix(trimmed, localhostSecurePrefix) {
		return strings.TrimPrefix(trimmed, localhostSecurePrefix), true
	}
	return "", false
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
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
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
	if result.SDSDesignResult == nil {
		result.SDSDesignResult = &SDSSyncSummary{VariantID: options.VariantID}
	}
	result.SDSDesignResult.MockupImageURLs = rendered
	result.SDSDesignResult.Status = "local_rendered"
	if result.SDSDesignResult.Error == "" {
		result.SDSDesignResult.Error = "SDS render unavailable; used local SDS mockup composite"
	}
	ensureResultPodExecution(result, nil)
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
