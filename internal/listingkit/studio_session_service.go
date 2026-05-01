package listingkit

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

type StudioSessionService interface {
	EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error)
	GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error)
	UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error)
	ReplaceStudioSessionDesigns(ctx context.Context, sessionID string, req *ReplaceStudioSessionDesignsRequest) (*SheinStudioSessionDetail, error)
	ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error)
}

func (s *service) EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	if s.studioSessionRepo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	if req == nil || req.Selection == nil || req.Selection.VariantID <= 0 {
		return nil, fmt.Errorf("selection is required")
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
		UserID:                  strings.TrimSpace(req.UserID),
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
			ID:               designID,
			SessionID:        sessionID,
			ImageURL:         design.ImageURL,
			ProductImageURLs: append(SheinStudioStringList(nil), design.ProductImageURLs...),
			RevisedPrompt:    design.RevisedPrompt,
			Role:             design.Role,
			RoleLabel:        design.RoleLabel,
			ReviewNote:       design.ReviewNote,
			SortOrder:        index,
			Approved:         approved,
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
