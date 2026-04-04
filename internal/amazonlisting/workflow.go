package amazonlisting

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type WorkflowArtifacts struct {
	ProductTask      *productenrich.Task
	CanonicalProduct *productenrich.CanonicalProduct
	ImageTask        *productimage.Task
	ImageResult      *productimage.ImageProcessResult
	Draft            *AmazonListingDraft
}

type WorkflowError struct {
	Artifacts *WorkflowArtifacts
	Err       error
}

func (e *WorkflowError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *WorkflowError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type ListingWorkflow interface {
	Run(ctx context.Context, task *Task) (*WorkflowArtifacts, error)
}

type listingWorkflow struct {
	productService ProductService
	imageService   ImageService
	assembler      Assembler
	autoFixer      AutoFixer
	exportBuilder  ExportBuilder
}

func NewListingWorkflow(productService ProductService, imageService ImageService, assembler Assembler, autoFixer AutoFixer, exportBuilder ExportBuilder) ListingWorkflow {
	return &listingWorkflow{
		productService: productService,
		imageService:   imageService,
		assembler:      assembler,
		autoFixer:      autoFixer,
		exportBuilder:  exportBuilder,
	}
}

func (w *listingWorkflow) Run(ctx context.Context, task *Task) (*WorkflowArtifacts, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	if w.productService == nil {
		return nil, fmt.Errorf("product service is not configured")
	}
	if w.assembler == nil {
		return nil, fmt.Errorf("assembler is not configured")
	}

	artifacts := &WorkflowArtifacts{
		Draft: initWorkflowDraft(task),
	}

	productTask, canonicalProduct, err := w.ensureProductArtifacts(ctx, task, artifacts)
	if err != nil {
		return nil, err
	}
	artifacts.ProductTask = productTask
	artifacts.CanonicalProduct = canonicalProduct

	var (
		imageTask   *productimage.Task
		imageResult *productimage.ImageProcessResult
	)
	if task.Request.Options == nil || task.Request.Options.ProcessImages {
		imageTask, imageResult, err = w.ensureImageArtifacts(ctx, task, artifacts)
		if err != nil {
			return nil, err
		}
		artifacts.ImageTask = imageTask
		artifacts.ImageResult = imageResult
	}

	draft := w.assembler.Assemble(task, canonicalProduct, imageResult)
	draft.CanonicalProduct = canonicalProduct
	draft.ChildTasks = cloneChildTasks(artifacts.Draft.ChildTasks)
	draft.ReviewItems = append(draft.ReviewItems, buildReviewItemsFromCanonical(canonicalProduct)...)
	draft.ProductTaskID = productTask.ID
	if imageTask != nil {
		draft.ProductImageTaskID = imageTask.ID
	}
	if w.autoFixer != nil {
		w.autoFixer.Fix(task.Request, draft)
	}
	if w.exportBuilder != nil {
		draft.Export = w.exportBuilder.Build(task.Request, draft)
	}

	artifacts.Draft = draft
	return artifacts, nil
}

func (w *listingWorkflow) ensureProductArtifacts(ctx context.Context, task *Task, artifacts *WorkflowArtifacts) (*productenrich.Task, *productenrich.CanonicalProduct, error) {
	if task != nil && task.Result != nil && task.Result.ProductTaskID != "" {
		taskResult, err := w.productService.GetTaskResult(ctx, task.Result.ProductTaskID)
		if err == nil && taskResult != nil && taskResult.Status == productenrich.TaskStatusCompleted && taskResult.ProductJSON != nil {
			updateChildTaskState(artifacts.Draft, "product_enrich", task.Result.ProductTaskID, string(taskResult.Status), "")
			productTask := &productenrich.Task{
				ID:      task.Result.ProductTaskID,
				Request: toProductGenerateRequest(task),
				Status:  taskResult.Status,
				Result:  taskResult.ProductJSON,
			}
			canonical := productenrich.BuildCanonicalProduct(productTask.Request, taskResult.ProductJSON)
			return productTask, canonical, nil
		}
	}

	inlineProductCtx := productenrich.WithInlineTaskExecution(ctx)
	productTask, err := w.productService.CreateGenerateTask(inlineProductCtx, toProductGenerateRequest(task))
	if err != nil {
		markChildTaskFailed(artifacts.Draft, "product_enrich", "", err)
		return nil, nil, &WorkflowError{Artifacts: artifacts, Err: fmt.Errorf("failed to create product task: %w", err)}
	}
	updateChildTaskState(artifacts.Draft, "product_enrich", productTask.ID, string(productenrich.TaskStatusPending), "")

	productJSON, err := w.productService.ProcessProduct(ctx, productTask)
	if err != nil {
		markChildTaskFailed(artifacts.Draft, "product_enrich", productTask.ID, err)
		return nil, nil, &WorkflowError{Artifacts: artifacts, Err: fmt.Errorf("product enrichment failed: %w", err)}
	}
	updateChildTaskState(artifacts.Draft, "product_enrich", productTask.ID, string(productenrich.TaskStatusCompleted), "")
	canonicalProduct := productenrich.BuildCanonicalProduct(productTask.Request, productJSON)
	return productTask, canonicalProduct, nil
}

func (w *listingWorkflow) ensureImageArtifacts(ctx context.Context, task *Task, artifacts *WorkflowArtifacts) (*productimage.Task, *productimage.ImageProcessResult, error) {
	if w.imageService == nil {
		markChildTaskFailed(artifacts.Draft, "product_image", "", fmt.Errorf("image service is not configured"))
		return nil, nil, &WorkflowError{Artifacts: artifacts, Err: fmt.Errorf("image service is not configured")}
	}

	if task != nil && task.Result != nil && task.Result.ProductImageTaskID != "" {
		taskResult, err := w.imageService.GetTaskResult(ctx, task.Result.ProductImageTaskID)
		if err == nil && taskResult != nil {
			switch taskResult.Status {
			case productimage.TaskStatusCompleted, productimage.TaskStatusNeedsReview:
				if taskResult.Result != nil {
					updateChildTaskState(artifacts.Draft, "product_image", task.Result.ProductImageTaskID, string(taskResult.Status), taskResult.Error)
					imageTask := &productimage.Task{
						ID:      task.Result.ProductImageTaskID,
						Request: toImageProcessRequest(task),
						Status:  taskResult.Status,
						Result:  taskResult.Result,
						Error:   taskResult.Error,
					}
					return imageTask, taskResult.Result, nil
				}
			}
		}
	}

	inlineImageCtx := productimage.WithInlineTaskExecution(ctx)
	imageTask, err := w.imageService.CreateProcessTask(inlineImageCtx, toImageProcessRequest(task))
	if err != nil {
		markChildTaskFailed(artifacts.Draft, "product_image", "", err)
		return nil, nil, &WorkflowError{Artifacts: artifacts, Err: fmt.Errorf("failed to create image task: %w", err)}
	}
	updateChildTaskState(artifacts.Draft, "product_image", imageTask.ID, string(productimage.TaskStatusPending), "")
	imageResult, err := w.imageService.ProcessImages(ctx, imageTask)
	if err != nil {
		markChildTaskFailed(artifacts.Draft, "product_image", imageTask.ID, err)
		return nil, nil, &WorkflowError{Artifacts: artifacts, Err: fmt.Errorf("image processing failed: %w", err)}
	}
	updateChildTaskState(artifacts.Draft, "product_image", imageTask.ID, string(productimage.TaskStatusCompleted), "")
	return imageTask, imageResult, nil
}

func initWorkflowDraft(task *Task) *AmazonListingDraft {
	if task == nil {
		return &AmazonListingDraft{}
	}
	return &AmazonListingDraft{
		TaskID:      task.ID,
		Status:      string(TaskStatusProcessing),
		Marketplace: task.Request.Marketplace,
		Country:     task.Request.Country,
		Language:    task.Request.Language,
		Source: AmazonSourceTrace{
			InputTextProvided: strings.TrimSpace(task.Request.Text) != "",
			InputImageCount:   len(task.Request.ImageURLs),
			ProductURL:        task.Request.ProductURL,
			UsedImageSources:  append([]string(nil), task.Request.ImageURLs...),
		},
	}
}

func updateChildTaskState(draft *AmazonListingDraft, kind, taskID, status, errorMsg string) {
	if draft == nil {
		return
	}
	for idx := range draft.ChildTasks {
		if draft.ChildTasks[idx].Kind == kind {
			draft.ChildTasks[idx].TaskID = taskID
			draft.ChildTasks[idx].Status = status
			draft.ChildTasks[idx].Error = errorMsg
			return
		}
	}
	draft.ChildTasks = append(draft.ChildTasks, ChildTaskState{
		Kind:   kind,
		TaskID: taskID,
		Status: status,
		Error:  errorMsg,
	})
}

func markChildTaskFailed(draft *AmazonListingDraft, kind, taskID string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	updateChildTaskState(draft, kind, taskID, string(TaskStatusFailed), msg)
}

func cloneChildTasks(tasks []ChildTaskState) []ChildTaskState {
	if len(tasks) == 0 {
		return nil
	}
	return append([]ChildTaskState(nil), tasks...)
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
		Marketplace: task.Request.Marketplace,
		Country:     task.Request.Country,
	}
}
