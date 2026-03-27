package amazonlisting

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func (s *service) ProcessListing(ctx context.Context, task *Task) (*AmazonListingDraft, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	if err := s.repo.MarkProcessing(ctx, task.ID); err != nil {
		return nil, fmt.Errorf("failed to mark task as processing: %w", err)
	}

	productTask, err := s.productService.CreateGenerateTask(ctx, &productenrich.GenerateRequest{
		ImageURLs:  task.Request.ImageURLs,
		Text:       task.Request.Text,
		ProductURL: task.Request.ProductURL,
	})
	if err != nil {
		_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to create product task: %v", err))
		return nil, err
	}
	productJSON, err := s.productService.ProcessProduct(ctx, productTask)
	if err != nil {
		_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("product enrichment failed: %v", err))
		return nil, err
	}

	var (
		imageResult *productimage.ImageProcessResult
		imageTask   *productimage.Task
	)
	if task.Request.Options == nil || task.Request.Options.ProcessImages {
		if s.imageService == nil {
			_ = s.repo.MarkFailed(ctx, task.ID, "image service is not configured")
			return nil, fmt.Errorf("image service is not configured")
		}
		imageTask, err = s.imageService.CreateProcessTask(ctx, &productimage.ImageProcessRequest{
			ProductURL:  task.Request.ProductURL,
			ImageURLs:   task.Request.ImageURLs,
			Text:        task.Request.Text,
			Marketplace: task.Request.Marketplace,
			Country:     task.Request.Country,
		})
		if err != nil {
			_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to create image task: %v", err))
			return nil, err
		}
		imageResult, err = s.imageService.ProcessImages(ctx, imageTask)
		if err != nil {
			_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("image processing failed: %v", err))
			return nil, err
		}
	}

	draft := s.assembler.Assemble(task, productJSON, imageResult)
	draft.ProductTaskID = productTask.ID
	if imageTask != nil {
		draft.ProductImageTaskID = imageTask.ID
	}
	if s.autoFixer != nil {
		s.autoFixer.Fix(task.Request, draft)
	}
	if s.exportBuilder != nil {
		draft.Export = s.exportBuilder.Build(task.Request, draft)
	}

	report := s.validator.Validate(task.Request, draft)
	draft.Compliance = &AmazonComplianceReport{
		Ready:          report.Ready,
		BlockingIssues: append([]string(nil), report.BlockingIssues...),
		Warnings:       append([]string(nil), report.Warnings...),
	}
	draft.Review = &AmazonReviewReport{
		NeedsReview: report.NeedsReview,
		Reasons:     append([]string(nil), report.ReviewReasons...),
	}

	if len(report.BlockingIssues) > 0 {
		draft.Status = string(TaskStatusFailed)
		if err := s.repo.MarkFailed(ctx, task.ID, strings.Join(report.BlockingIssues, "; ")); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", strings.Join(report.BlockingIssues, "; "))
	}
	if report.NeedsReview {
		draft.Status = string(TaskStatusNeedsReview)
		if err := s.repo.MarkNeedsReview(ctx, task.ID, draft, strings.Join(report.ReviewReasons, "; ")); err != nil {
			return nil, err
		}
		return draft, nil
	}
	draft.Status = string(TaskStatusCompleted)
	if err := s.repo.MarkCompleted(ctx, task.ID, draft); err != nil {
		return nil, err
	}
	return draft, nil
}
