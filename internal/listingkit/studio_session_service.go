package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type StudioSessionService interface {
	EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error)
	GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error)
	UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error)
	ReplaceStudioSessionDesigns(ctx context.Context, sessionID string, req *ReplaceStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error)
	AppendStudioSessionDesigns(ctx context.Context, sessionID string, req *AppendStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error)
	ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error)
	ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error)
	GetStudioBatch(ctx context.Context, batchID string) (*SheinStudioSessionDetail, error)
	UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*SheinStudioSessionDetail, error)
	DeleteStudioBatch(ctx context.Context, batchID string) error
}

func (s *service) EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	return s.taskStudioSessionOrDefault().EnsureStudioSession(ctx, req)
}

func (s *service) GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error) {
	return s.taskStudioSessionOrDefault().GetStudioSession(ctx, sessionID)
}

func (s *service) UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	return s.taskStudioSessionOrDefault().UpdateStudioSession(ctx, sessionID, req)
}

func (s *service) ReplaceStudioSessionDesigns(ctx context.Context, sessionID string, req *ReplaceStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error) {
	return s.taskStudioSessionOrDefault().ReplaceStudioSessionDesigns(ctx, sessionID, req)
}

func (s *service) AppendStudioSessionDesigns(ctx context.Context, sessionID string, req *AppendStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error) {
	return s.taskStudioSessionOrDefault().AppendStudioSessionDesigns(ctx, sessionID, req)
}

func (s *service) ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error) {
	return s.taskStudioSessionOrDefault().ListStudioSessionGallery(ctx, limit)
}

func (s *service) ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error) {
	return s.taskStudioSessionOrDefault().ListStudioBatches(ctx, limit)
}

func (s *service) GetStudioBatch(ctx context.Context, batchID string) (*SheinStudioSessionDetail, error) {
	return s.taskStudioSessionOrDefault().GetStudioBatch(ctx, batchID)
}

func (s *service) UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*SheinStudioSessionDetail, error) {
	return s.taskStudioSessionOrDefault().UpsertStudioBatch(ctx, req)
}

func (s *service) DeleteStudioBatch(ctx context.Context, batchID string) error {
	return s.taskStudioSessionOrDefault().DeleteStudioBatch(ctx, batchID)
}

func (s *service) taskStudioSessionOrDefault() *taskStudioSessionService {
	if s.taskStudioSession != nil {
		return s.taskStudioSession
	}
	s.taskStudioSession = newTaskStudioSessionService(taskStudioSessionServiceConfig{
		repo: s.studioSessionRepo,
	})
	return s.taskStudioSession
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
		ID:                      session.ID,
		BatchName:               session.BatchName,
		Prompt:                  session.Prompt,
		StyleCount:              session.StyleCount,
		VariationIntensity:      session.VariationIntensity,
		ProductImageCount:       session.ProductImageCount,
		ProductImagePrompt:      session.ProductImagePrompt,
		ProductImagePrompts:     []SheinStudioProductImagePrompt(session.ProductImagePrompts),
		ArtworkModel:            session.ArtworkModel,
		ImageStrategy:           session.ImageStrategy,
		TransparentBackground:   session.TransparentBackground,
		RenderSizeImagesWithSDS: session.RenderSizeImagesWithSDS,
		SheinStoreID:            session.SheinStoreID,
		Selection:               &selection,
		GroupedSelections:       []SheinStudioGroupedSelection(session.GroupedSelections),
		ApprovedDesignIDs:       []string(session.ApprovedDesignIDs),
		CreatedTasks:            []SheinStudioCreatedTask(session.CreatedTasks),
		DesignCount:             designCount,
		UpdatedAt:               session.UpdatedAt.UTC().Format(time.RFC3339),
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
