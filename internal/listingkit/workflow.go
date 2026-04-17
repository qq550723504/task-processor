package listingkit

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func (s *service) runWorkflow(ctx context.Context, task *Task) (*ListingKitResult, error) {
	result := initResult(task)

	productTask, err := s.productSvc.CreateGenerateTask(productenrich.WithInlineTaskExecution(ctx), toProductGenerateRequest(task))
	if err != nil {
		markChildTask(result, "product_enrich", "", string(TaskStatusFailed), err.Error())
		return result, fmt.Errorf("failed to create product task: %w", err)
	}
	markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusPending), "")

	productJSON, err := s.productSvc.ProcessProduct(ctx, productTask)
	if err != nil {
		markChildTask(result, "product_enrich", productTask.ID, string(TaskStatusFailed), err.Error())
		return result, fmt.Errorf("product enrichment failed: %w", err)
	}
	markChildTask(result, "product_enrich", productTask.ID, string(productenrich.TaskStatusCompleted), "")

	canonical := productenrich.BuildCanonicalProduct(productTask.Request, productJSON)
	result.CanonicalProduct = canonical
	result.CatalogProduct = catalog.BuildProduct(canonical)
	result.AssetBundle = asset.BuildBundle(canonical, nil)

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
			}
		}
	}

	final := s.assembler.Assemble(task, canonical, imageResult)
	final.CatalogProduct = result.CatalogProduct
	final.AssetBundle = result.AssetBundle
	final.ChildTasks = append([]ChildTaskState(nil), result.ChildTasks...)
	if final.Summary == nil {
		final.Summary = &GenerationSummary{}
	}
	final.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, result.Summary.Warnings...))
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
	return &productimage.ImageProcessRequest{
		ProductURL:  task.Request.ProductURL,
		ImageURLs:   append([]string(nil), task.Request.ImageURLs...),
		Text:        task.Request.Text,
		Marketplace: "amazon",
		Country:     task.Request.Country,
	}
}

func shouldProcessImages(req *GenerateRequest) bool {
	return req != nil && req.Options != nil && req.Options.ProcessImages && len(req.ImageURLs) > 0
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
