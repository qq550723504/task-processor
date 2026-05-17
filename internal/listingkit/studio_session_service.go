package listingkit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type StudioSessionService interface {
	EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error)
	GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error)
	UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error)
	ReplaceStudioSessionDesigns(ctx context.Context, sessionID string, req *ReplaceStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error)
	ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error)
	ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error)
	GetStudioBatch(ctx context.Context, batchID string) (*SheinStudioSessionDetail, error)
	UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*SheinStudioSessionDetail, error)
	DeleteStudioBatch(ctx context.Context, batchID string) error
}

func (s *service) EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	if req == nil || req.Selection == nil || req.Selection.VariantID <= 0 {
		return nil, fmt.Errorf("selection is required")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = RequestUserIDFromContext(ctx)
	}

	selectionKey := buildStudioSelectionKey(req.Selection)
	existing, err := s.studioSessionRepo.FindLatestSessionBySelectionKey(ctx, selectionKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.loadStudioSessionDetail(ctx, existing)
	}

	session := &SheinStudioSession{
		ID:                      uuid.NewString(),
		UserID:                  userID,
		SelectionKey:            selectionKey,
		Status:                  SheinStudioSessionStatusSelecting,
		ProductID:               req.Selection.ProductID,
		ParentProductID:         req.Selection.ParentProductID,
		VariantID:               req.Selection.VariantID,
		PrototypeGroupID:        req.Selection.PrototypeGroupID,
		LayerID:                 req.Selection.LayerID,
		PrintableWidth:          req.Selection.PrintableWidth,
		PrintableHeight:         req.Selection.PrintableHeight,
		SelectedVariantIDs:      append(SheinStudioInt64List(nil), req.Selection.SelectedVariantIDs...),
		Selection:               SheinStudioSelectionSnapshot(*req.Selection),
		RenderSizeImagesWithSDS: true,
	}
	if err := s.studioSessionRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return &SheinStudioSessionDetail{
		Session: session,
		Designs: []SheinStudioDesign{},
	}, nil
}

func (s *service) GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.studioSessionRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}
	return s.loadStudioSessionDetail(ctx, session)
}

func (s *service) UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.studioSessionRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}

	if req != nil {
		if req.Status != nil {
			session.Status = *req.Status
		}
		if req.Prompt != nil {
			session.Prompt = *req.Prompt
		}
		if req.StyleCount != nil {
			session.StyleCount = *req.StyleCount
		}
		if req.VariationIntensity != nil {
			session.VariationIntensity = *req.VariationIntensity
		}
		if req.ProductImageCount != nil {
			session.ProductImageCount = *req.ProductImageCount
		}
		if req.ProductImagePrompt != nil {
			session.ProductImagePrompt = *req.ProductImagePrompt
		}
		if req.ProductImagePrompts != nil {
			session.ProductImagePrompts = append(SheinStudioProductImagePromptList(nil), req.ProductImagePrompts...)
		}
		if req.ArtworkModel != nil {
			session.ArtworkModel = *req.ArtworkModel
		}
		if req.ImageStrategy != nil {
			session.ImageStrategy = *req.ImageStrategy
		}
		if req.SelectedSDSImages != nil {
			selected := make(SheinStudioSelectedSDSImageList, 0, len(req.SelectedSDSImages))
			for _, item := range req.SelectedSDSImages {
				selected = append(selected, SheinStudioSelectedSDSImageRecord(item))
			}
			session.SelectedSDSImages = selected
		}
		if req.TransparentBackground != nil {
			session.TransparentBackground = *req.TransparentBackground
		}
		if req.RenderSizeImagesWithSDS != nil {
			session.RenderSizeImagesWithSDS = *req.RenderSizeImagesWithSDS
		}
		if req.SheinStoreID != nil {
			session.SheinStoreID = *req.SheinStoreID
		}
		if req.GenerationJobID != nil {
			session.GenerationJobID = *req.GenerationJobID
		}
		if req.GenerationError != nil {
			session.GenerationError = *req.GenerationError
		}
		if req.ApprovedDesignIDs != nil {
			session.ApprovedDesignIDs = slices.Clone(req.ApprovedDesignIDs)
		}
		if req.CreatedTasks != nil {
			session.CreatedTasks = append(SheinStudioCreatedTaskList(nil), req.CreatedTasks...)
			taskIDs := make([]string, 0, len(req.CreatedTasks))
			for _, task := range req.CreatedTasks {
				if id := strings.TrimSpace(task.ID); id != "" {
					taskIDs = append(taskIDs, id)
				}
			}
			session.CreatedTaskIDs = taskIDs
		}
	}

	if err := s.studioSessionRepo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	return s.loadStudioSessionDetail(ctx, session)
}

func (s *service) ReplaceStudioSessionDesigns(ctx context.Context, sessionID string, req *ReplaceStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.studioSessionRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}

	approvedSet := make(map[string]struct{}, len(req.ApprovedDesignIDs))
	for _, id := range req.ApprovedDesignIDs {
		if normalized := strings.TrimSpace(id); normalized != "" {
			approvedSet[normalized] = struct{}{}
		}
	}

	designs := make([]SheinStudioDesign, 0, len(req.Designs))
	for index, design := range req.Designs {
		designID := strings.TrimSpace(design.ID)
		if designID == "" {
			designID = uuid.NewString()
		}
		_, approved := approvedSet[designID]
		designs = append(designs, SheinStudioDesign{
			ID:                    designID,
			SessionID:             sessionID,
			ImageURL:              design.ImageURL,
			ProductImageURLs:      append(SheinStudioStringList(nil), design.ProductImageURLs...),
			Prompt:                design.Prompt,
			RevisedPrompt:         design.RevisedPrompt,
			ImageModel:            design.ImageModel,
			TransparentBackground: design.TransparentBackground,
			VariationIntensity:    design.VariationIntensity,
			Role:                  design.Role,
			RoleLabel:             design.RoleLabel,
			ReviewNote:            design.ReviewNote,
			SortOrder:             index,
			Approved:              approved,
		})
	}

	if req.Status != nil {
		session.Status = *req.Status
	}
	session.ApprovedDesignIDs = slices.Clone(req.ApprovedDesignIDs)
	if err := s.studioSessionRepo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	if err := s.studioSessionRepo.ReplaceDesigns(ctx, sessionID, req.ApprovedDesignIDs, designs); err != nil {
		return nil, err
	}
	return s.loadStudioSessionDetail(ctx, session)
}

func (s *service) ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	items, err := s.studioSessionRepo.ListGalleryItems(ctx, limit)
	if err != nil {
		return nil, err
	}
	return &StudioSessionGalleryResponse{
		Items: items,
		Total: len(items),
	}, nil
}

func (s *service) ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	sessions, err := s.studioSessionRepo.ListBatchSessions(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]SheinStudioBatchListItem, 0, len(sessions))
	for _, session := range sessions {
		designs, err := s.studioSessionRepo.ListSessionDesigns(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, mapStudioBatchListItem(&session, len(designs)))
	}
	return &StudioBatchListResponse{Items: items, Total: len(items)}, nil
}

func (s *service) GetStudioBatch(ctx context.Context, batchID string) (*SheinStudioSessionDetail, error) {
	detail, err := s.GetStudioSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if detail.Session == nil || !detail.Session.SavedAsBatch {
		return nil, ErrStudioSessionNotFound
	}
	return detail, nil
}

func (s *service) UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*SheinStudioSessionDetail, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	if req == nil || req.Selection == nil || req.Selection.VariantID <= 0 {
		return nil, fmt.Errorf("selection is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	var session *SheinStudioSession
	var err error
	if strings.TrimSpace(req.ID) != "" {
		session, err = s.studioSessionRepo.GetSession(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		if session == nil {
			return nil, ErrStudioSessionNotFound
		}
	} else {
		session = &SheinStudioSession{
			ID:                      uuid.NewString(),
			UserID:                  RequestUserIDFromContext(ctx),
			SelectionKey:            buildStudioSelectionKey(req.Selection),
			RenderSizeImagesWithSDS: true,
		}
	}

	session.SelectionKey = buildStudioSelectionKey(req.Selection)
	session.Status = deriveBatchStatus(req)
	session.ProductID = req.Selection.ProductID
	session.ParentProductID = req.Selection.ParentProductID
	session.VariantID = req.Selection.VariantID
	session.PrototypeGroupID = req.Selection.PrototypeGroupID
	session.LayerID = req.Selection.LayerID
	session.PrintableWidth = req.Selection.PrintableWidth
	session.PrintableHeight = req.Selection.PrintableHeight
	session.SelectedVariantIDs = append(SheinStudioInt64List(nil), req.Selection.SelectedVariantIDs...)
	session.Selection = SheinStudioSelectionSnapshot(*req.Selection)
	session.Prompt = req.Prompt
	session.StyleCount = req.StyleCount
	session.VariationIntensity = req.VariationIntensity
	session.ProductImageCount = req.ProductImageCount
	session.ProductImagePrompt = req.ProductImagePrompt
	session.ProductImagePrompts = toStudioProductImagePromptList(req.ProductImagePrompts)
	session.ArtworkModel = req.ArtworkModel
	session.ImageStrategy = req.ImageStrategy
	session.SelectedSDSImages = toStudioSelectedSDSImageList(req.SelectedSDSImages)
	session.TransparentBackground = req.TransparentBackground
	session.RenderSizeImagesWithSDS = req.RenderSizeImagesWithSDS
	session.SheinStoreID = req.SheinStoreID
	session.ApprovedDesignIDs = append(SheinStudioStringList(nil), req.ApprovedDesignIDs...)
	session.CreatedTasks = toStudioCreatedTaskList(req.CreatedTasks)
	session.CreatedTaskIDs = buildCreatedTaskIDs(req.CreatedTasks)
	session.SavedAsBatch = true
	session.BatchName = strings.TrimSpace(req.BatchName)
	if session.BatchName == "" {
		session.BatchName = deriveStudioBatchName(req.Prompt)
	}

	if strings.TrimSpace(req.ID) == "" {
		if err := s.studioSessionRepo.CreateSession(ctx, session); err != nil {
			return nil, err
		}
	} else if err := s.studioSessionRepo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	if err := s.studioSessionRepo.ReplaceDesigns(ctx, session.ID, req.ApprovedDesignIDs, req.Designs); err != nil {
		return nil, err
	}
	return s.loadStudioSessionDetail(ctx, session)
}

func (s *service) DeleteStudioBatch(ctx context.Context, batchID string) error {
	if s.studioSessionRepo == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || !session.SavedAsBatch {
		return ErrStudioSessionNotFound
	}
	return s.studioSessionRepo.DeleteSession(ctx, batchID)
}

func (s *service) loadStudioSessionDetail(ctx context.Context, session *SheinStudioSession) (*SheinStudioSessionDetail, error) {
	designs, err := s.studioSessionRepo.ListSessionDesigns(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	return &SheinStudioSessionDetail{
		Session: session,
		Designs: designs,
	}, nil
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

func deriveStudioBatchName(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "未命名批次"
	}
	if len([]rune(trimmed)) > 36 {
		return string([]rune(trimmed)[:36]) + "..."
	}
	return trimmed
}

func deriveBatchStatus(req *UpsertStudioBatchRequest) SheinStudioSessionStatus {
	switch {
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
