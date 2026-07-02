package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type StudioSessionService interface {
	ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error)
	ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error)
	GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchDraftDetail, error)
	GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error)
	PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error)
	RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error)
	ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error)
	CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error)
	UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error)
	DeleteStudioBatch(ctx context.Context, batchID string) error
	SyncStudioDesignAsyncJob(ctx context.Context, sessionID string, jobStatus StudioAsyncJobStatus, jobID string, errMessage string) error
}

func buildStudioSelectionKey(selection *SheinStudioSelection) string {
	if selection == nil {
		return ""
	}
	variantIDs := make([]string, 0, len(selection.SelectedVariantIDs))
	for _, id := range selection.SelectedVariantIDs {
		variantIDs = append(variantIDs, fmt.Sprintf("%d", id))
	}
	return fmt.Sprintf(
		"%d:%d:%d:%d:%s:%d:%d:%s",
		selection.ProductID,
		selection.ParentProductID,
		selection.VariantID,
		selection.PrototypeGroupID,
		strings.TrimSpace(selection.LayerID),
		selection.PrintableWidth,
		selection.PrintableHeight,
		strings.Join(variantIDs, ","),
	)
}

func deriveBatchStatus(req *UpsertStudioBatchRequest) SheinStudioSessionStatus {
	switch {
	case len(req.GenerationJobs) > 0:
		return SheinStudioSessionStatusGenerating
	case len(req.CreatedTasks) > 0:
		return SheinStudioSessionStatusTasksCreated
	case len(req.Designs) > 0:
		return SheinStudioSessionStatusReviewing
	default:
		return SheinStudioSessionStatusSelecting
	}
}

func mapStudioBatchListItem(session *SheinStudioSession, designCount int) SheinStudioBatchListItem {
	if session == nil {
		return SheinStudioBatchListItem{}
	}
	selection := SheinStudioSelection(session.Selection)
	return SheinStudioBatchListItem{
		ID:                         session.ID,
		BatchName:                  session.BatchName,
		Status:                     string(session.Status),
		Prompt:                     session.Prompt,
		PromptMode:                 session.PromptMode,
		StyleCount:                 session.StyleCount,
		VariationIntensity:         session.VariationIntensity,
		ProductImageCount:          session.ProductImageCount,
		ProductImagePrompt:         session.ProductImagePrompt,
		ProductImagePrompts:        []SheinStudioProductImagePrompt(session.ProductImagePrompts),
		ArtworkModel:               session.ArtworkModel,
		ImageStrategy:              session.ImageStrategy,
		GroupedImageMode:           session.GroupedImageMode,
		TransparentBackground:      session.TransparentBackground,
		RenderSizeImagesWithSDS:    session.RenderSizeImagesWithSDS,
		HotStyleReferenceImageURLs: append([]string(nil), session.HotStyleReferenceImageURLs...),
		HotStyleReferenceBrief:     session.HotStyleReferenceBrief,
		HotStyleReferencePrompt:    session.HotStyleReferencePrompt,
		SheinStoreID:               session.SheinStoreID,
		Selection:                  &selection,
		GroupedSelections:          []SheinStudioGroupedSelection(session.GroupedSelections),
		ApprovedDesignIDs:          []string(session.ApprovedDesignIDs),
		CreatedTasks:               []SheinStudioCreatedTask(session.CreatedTasks),
		DesignCount:                designCount,
		UpdatedAt:                  session.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toStudioProductImagePromptList(items []SheinStudioProductImagePrompt) SheinStudioProductImagePromptList {
	return append(SheinStudioProductImagePromptList(nil), items...)
}

func toStudioSelectedSDSImageList(items []SheinStudioSelectedSDSImage) SheinStudioSelectedSDSImageList {
	if len(items) == 0 {
		return nil
	}
	result := make(SheinStudioSelectedSDSImageList, 0, len(items))
	for _, item := range items {
		result = append(result, SheinStudioSelectedSDSImageRecord{
			ImageURL:   item.ImageURL,
			VariantSKU: item.VariantSKU,
			Color:      item.Color,
		})
	}
	return result
}

func toStudioCreatedTaskList(items []SheinStudioCreatedTask) SheinStudioCreatedTaskList {
	return append(SheinStudioCreatedTaskList(nil), items...)
}

func toStudioGroupedSelectionList(items []SheinStudioGroupedSelection) SheinStudioGroupedSelectionList {
	return append(SheinStudioGroupedSelectionList(nil), items...)
}

func buildCreatedTaskIDs(items []SheinStudioCreatedTask) SheinStudioStringList {
	if len(items) == 0 {
		return nil
	}
	result := make(SheinStudioStringList, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) != "" {
			result = append(result, item.ID)
		}
	}
	return result
}
