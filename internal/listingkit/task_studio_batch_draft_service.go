package listingkit

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type taskStudioBatchDraftServiceConfig struct {
	repo studioBatchDraftRepository
}

type taskStudioBatchDraftService struct {
	repo studioBatchDraftRepository
}

func newTaskStudioBatchDraftService(config taskStudioBatchDraftServiceConfig) *taskStudioBatchDraftService {
	return &taskStudioBatchDraftService{repo: config.repo}
}

func (s *taskStudioBatchDraftService) ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	items, err := s.repo.ListGalleryItems(ctx, limit)
	if err != nil {
		return nil, err
	}
	return &StudioSessionGalleryResponse{
		Items: items,
		Total: len(items),
	}, nil
}

func (s *taskStudioBatchDraftService) ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	sessions, err := s.repo.ListBatchSessions(ctx, limit)
	if err != nil {
		return nil, err
	}
	sessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
	}
	designCounts, err := s.repo.CountSessionDesignsBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, err
	}
	items := make([]SheinStudioBatchListItem, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, mapStudioBatchListItem(&session, designCounts[session.ID]))
	}
	return &StudioBatchListResponse{Items: items, Total: len(items)}, nil
}

func (s *taskStudioBatchDraftService) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchDraftDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if session == nil || !session.SavedAsBatch {
		return nil, ErrStudioSessionNotFound
	}
	return s.loadStudioBatchDraftDetail(ctx, session)
}

func (s *taskStudioBatchDraftService) UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	if req == nil || req.Selection == nil || req.Selection.VariantID <= 0 {
		return nil, fmt.Errorf("selection is required")
	}

	var session *SheinStudioSession
	var err error
	isCreate := strings.TrimSpace(req.ID) == ""
	normalizedReq := req
	if isCreate {
		normalizedReq = sanitizeStudioBatchCreateRequest(req)
	}
	if strings.TrimSpace(req.ID) != "" {
		session, err = s.repo.GetSession(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		if session == nil {
			return nil, ErrStudioSessionNotFound
		}
		if err := validateStudioSessionExpectedUpdatedAt(session.UpdatedAt, studioSessionStringPtr(req.ExpectedUpdatedAt)); err != nil {
			return nil, err
		}
	} else {
		session = &SheinStudioSession{
			ID:                      uuid.NewString(),
			UserID:                  RequestUserIDFromContext(ctx),
			SelectionKey:            buildStudioSelectionKey(req.Selection),
			RenderSizeImagesWithSDS: true,
		}
	}
	existingBatchName := strings.TrimSpace(session.BatchName)

	session.SelectionKey = buildStudioSelectionKey(normalizedReq.Selection)
	session.Status = deriveBatchStatus(normalizedReq)
	session.ProductID = normalizedReq.Selection.ProductID
	session.ParentProductID = normalizedReq.Selection.ParentProductID
	session.VariantID = normalizedReq.Selection.VariantID
	session.PrototypeGroupID = normalizedReq.Selection.PrototypeGroupID
	session.LayerID = normalizedReq.Selection.LayerID
	session.PrintableWidth = normalizedReq.Selection.PrintableWidth
	session.PrintableHeight = normalizedReq.Selection.PrintableHeight
	session.SelectedVariantIDs = append(SheinStudioInt64List(nil), normalizedReq.Selection.SelectedVariantIDs...)
	session.Selection = SheinStudioSelectionSnapshot(*normalizedReq.Selection)
	session.Prompt = normalizedReq.Prompt
	session.StyleCount = normalizedReq.StyleCount
	session.VariationIntensity = normalizedReq.VariationIntensity
	session.ProductImageCount = normalizedReq.ProductImageCount
	session.ProductImagePrompt = normalizedReq.ProductImagePrompt
	session.ProductImagePrompts = toStudioProductImagePromptList(normalizedReq.ProductImagePrompts)
	session.ArtworkModel = normalizedReq.ArtworkModel
	session.ImageStrategy = normalizedReq.ImageStrategy
	session.GroupedImageMode = normalizedReq.GroupedImageMode
	session.SelectedSDSImages = toStudioSelectedSDSImageList(normalizedReq.SelectedSDSImages)
	session.GroupedSelections = toStudioGroupedSelectionList(normalizedReq.GroupedSelections)
	session.TransparentBackground = normalizedReq.TransparentBackground
	session.RenderSizeImagesWithSDS = normalizedReq.RenderSizeImagesWithSDS
	session.SheinStoreID = normalizedReq.SheinStoreID
	session.ApprovedDesignIDs = append(SheinStudioStringList(nil), normalizedReq.ApprovedDesignIDs...)
	session.CreatedTasks = toStudioCreatedTaskList(normalizedReq.CreatedTasks)
	session.CreatedTaskIDs = buildCreatedTaskIDs(normalizedReq.CreatedTasks)
	session.GenerationJobs = append(SheinStudioGenerationJobList(nil), normalizedReq.GenerationJobs...)
	session.SavedAsBatch = true
	session.BatchName = strings.TrimSpace(normalizedReq.BatchName)
	if session.BatchName == "" {
		switch {
		case !isCreate && existingBatchName != "":
			session.BatchName = existingBatchName
		default:
			session.BatchName, err = s.nextTenantBatchName(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	if isCreate {
		if err := s.repo.CreateSession(ctx, session); err != nil {
			return nil, err
		}
	} else if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceDesigns(ctx, session.ID, normalizedReq.ApprovedDesignIDs, normalizedReq.Designs); err != nil {
		return nil, err
	}
	studioSessionLogger.WithFields(studioSessionLogFields(ctx, logrus.Fields{
		"session_id":              session.ID,
		"batch_name":              session.BatchName,
		"is_create":               isCreate,
		"status":                  session.Status,
		"design_count":            len(normalizedReq.Designs),
		"approved_design_count":   len(normalizedReq.ApprovedDesignIDs),
		"created_task_count":      len(normalizedReq.CreatedTasks),
		"generation_jobs_count":   len(normalizedReq.GenerationJobs),
		"grouped_selection_count": len(normalizedReq.GroupedSelections),
		"shein_store_id":          session.SheinStoreID,
	})).Info("studio batch upserted")
	return s.loadStudioBatchDraftDetail(ctx, session)
}

func sanitizeStudioBatchCreateRequest(req *UpsertStudioBatchRequest) *UpsertStudioBatchRequest {
	if req == nil || len(req.GenerationJobs) == 0 {
		return req
	}
	cloned := *req
	cloned.GenerationJobs = nil
	return &cloned
}

func (s *taskStudioBatchDraftService) DeleteStudioBatch(ctx context.Context, batchID string) error {
	if s.repo == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || !session.SavedAsBatch {
		return nil
	}
	return s.repo.DeleteSession(ctx, batchID)
}

func (s *taskStudioBatchDraftService) nextTenantBatchName(ctx context.Context) (string, error) {
	names, err := s.repo.ListTenantBatchNames(ctx)
	if err != nil {
		return "", err
	}
	maxBatchNumber := 0
	for _, name := range names {
		nextValue, ok := parseStudioBatchNumber(name)
		if ok && nextValue > maxBatchNumber {
			maxBatchNumber = nextValue
		}
	}
	return fmt.Sprintf("批次%d", maxBatchNumber+1), nil
}

func (s *taskStudioBatchDraftService) loadStudioBatchDraftDetail(ctx context.Context, session *SheinStudioSession) (*StudioBatchDraftDetail, error) {
	designs, err := s.repo.ListSessionDesigns(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	return &StudioBatchDraftDetail{
		Batch:   (*StudioBatchDraft)(session),
		Designs: designs,
	}, nil
}

func parseStudioBatchNumber(name string) (int, bool) {
	trimmed := strings.TrimSpace(name)
	if !strings.HasPrefix(trimmed, "批次") {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(trimmed, "批次"))
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
