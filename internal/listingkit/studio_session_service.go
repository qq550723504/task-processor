package listingkit

import (
	"context"
	"time"

	studiodomain "task-processor/internal/listing/studio"
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
	UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error)
	DeleteStudioBatch(ctx context.Context, batchID string) error
	SyncStudioDesignAsyncJob(ctx context.Context, sessionID string, jobStatus StudioAsyncJobStatus, jobID string, errMessage string) error
}

func buildStudioSelectionKey(selection *SheinStudioSelection) string {
	if selection == nil {
		return ""
	}
	return studiodomain.BuildSelectionKey(studiodomain.SelectionKeyInput{
		ProductID:          selection.ProductID,
		ParentProductID:    selection.ParentProductID,
		VariantID:          selection.VariantID,
		PrototypeGroupID:   selection.PrototypeGroupID,
		LayerID:            selection.LayerID,
		PrintableWidth:     selection.PrintableWidth,
		PrintableHeight:    selection.PrintableHeight,
		SelectedVariantIDs: selection.SelectedVariantIDs,
	})
}

func deriveBatchStatus(req *UpsertStudioBatchRequest) SheinStudioSessionStatus {
	if req == nil {
		return SheinStudioSessionStatusSelecting
	}
	return SheinStudioSessionStatus(studiodomain.ResolveDraftStatus(studiodomain.DraftStatusInput{
		GenerationJobCount: len(req.GenerationJobs),
		DesignCount:        len(req.Designs),
	}))
}

func mapStudioBatchListItem(session *SheinStudioSession, designCount int) SheinStudioBatchListItem {
	if session == nil {
		return SheinStudioBatchListItem{}
	}
	selection := SheinStudioSelection(session.Selection)
	legacyTransparentBackground := session.TransparentBackground
	transparencyMode := NormalizeStudioTransparencyMode(string(session.TransparentBackgroundMode), &legacyTransparentBackground)
	return SheinStudioBatchListItem{
		ID:                         session.ID,
		BatchName:                  session.BatchName,
		Status:                     string(session.Status),
		Prompt:                     session.Prompt,
		PromptMode:                 session.PromptMode,
		StyleCount:                 session.StyleCount,
		VariationIntensity:         session.VariationIntensity,
		ArtworkModel:               session.ArtworkModel,
		GroupedImageMode:           session.GroupedImageMode,
		TransparentBackground:      transparencyMode != StudioTransparencyModeNone,
		TransparentBackgroundMode:  transparencyMode,
		RenderSizeImagesWithSDS:    session.RenderSizeImagesWithSDS,
		HotStyleReferenceImageURLs: append([]string(nil), session.HotStyleReferenceImageURLs...),
		HotStyleReferenceBrief:     session.HotStyleReferenceBrief,
		HotStyleReferencePrompt:    session.HotStyleReferencePrompt,
		SheinStoreID:               session.SheinStoreID,
		Selection:                  &selection,
		GroupedSelections:          []SheinStudioGroupedSelection(session.GroupedSelections),
		ApprovedDesignIDs:          []string(session.ApprovedDesignIDs),
		DesignCount:                designCount,
		UpdatedAt:                  session.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
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

func toStudioGroupedSelectionList(items []SheinStudioGroupedSelection) SheinStudioGroupedSelectionList {
	return append(SheinStudioGroupedSelectionList(nil), items...)
}
