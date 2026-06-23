package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	studiodomain "task-processor/internal/listing/studio"

	"gorm.io/gorm"
)

type taskStudioBatchDraftServiceConfig struct {
	repo       studioBatchDraftRepository
	batchRepo  StudioBatchRepository
	loadDetail func(context.Context, string) (*StudioBatchDetail, error)
	runner     *listingStudioBatchDraftRunner
}

type taskStudioBatchDraftService struct {
	repo       studioBatchDraftRepository
	batchRepo  StudioBatchRepository
	loadDetail func(context.Context, string) (*StudioBatchDetail, error)
	runner     *listingStudioBatchDraftRunner
}

func newTaskStudioBatchDraftService(config taskStudioBatchDraftServiceConfig) *taskStudioBatchDraftService {
	service := &taskStudioBatchDraftService{
		repo:       config.repo,
		batchRepo:  config.batchRepo,
		loadDetail: config.loadDetail,
		runner:     config.runner,
	}
	service.ensureRunner()
	return service
}

func (s *taskStudioBatchDraftService) ListStudioSessionGallery(ctx context.Context, limit int) (*StudioSessionGalleryResponse, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	result, err := s.runner.ListSessionGallery(ctx, limit)
	if err != nil {
		return nil, err
	}
	return &StudioSessionGalleryResponse{
		Items: result.Items,
		Total: result.Total,
	}, nil
}

func (s *taskStudioBatchDraftService) ListStudioBatches(ctx context.Context, limit int) (*StudioBatchListResponse, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	result, err := s.runner.ListBatches(ctx, limit)
	if err != nil {
		return nil, err
	}
	s.reconcileBatchListStatuses(ctx, result.Items)
	return &StudioBatchListResponse{Items: result.Items, Total: result.Total}, nil
}

func (s *taskStudioBatchDraftService) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchDraftDetail, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	result, err := s.runner.GetBatch(ctx, batchID)
	if err != nil {
		return nil, adaptStudioBatchDraftError(err)
	}
	return &StudioBatchDraftDetail{Batch: (*StudioBatchDraft)(result.Batch), Designs: result.Designs}, nil
}

func (s *taskStudioBatchDraftService) UpsertStudioBatch(ctx context.Context, req *UpsertStudioBatchRequest) (*StudioBatchDraftDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	if req == nil || req.Selection == nil || req.Selection.VariantID <= 0 {
		return nil, fmt.Errorf("selection is required")
	}

	req.Selection.DesignType = studiodomain.NormalizeBatchDesignType(req.Selection.DesignType)

	var session *SheinStudioSession
	var err error
	isCreate := strings.TrimSpace(req.ID) == ""
	normalizedReq := req
	if isCreate {
		normalizedReq = sanitizeStudioBatchCreateRequest(req, isCreate)
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
	session.PromptMode = strings.TrimSpace(normalizedReq.PromptMode)
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
	session.BatchName, err = s.resolveBatchName(ctx, studiodomain.BatchNameResolutionInput{
		RequestedName: normalizedReq.BatchName,
		ExistingName:  existingBatchName,
		IsCreate:      isCreate,
	})
	if err != nil {
		return nil, err
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

func sanitizeStudioBatchCreateRequest(req *UpsertStudioBatchRequest, isCreate bool) *UpsertStudioBatchRequest {
	if req == nil || !studiodomain.ShouldDropCreateGenerationJobs(isCreate, len(req.GenerationJobs)) {
		return req
	}
	cloned := *req
	cloned.GenerationJobs = nil
	return &cloned
}

func (s *taskStudioBatchDraftService) DeleteStudioBatch(ctx context.Context, batchID string) error {
	s.ensureRunner()
	if s.runner == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	return adaptStudioBatchDraftError(s.runner.DeleteBatch(ctx, batchID))
}

func (s *taskStudioBatchDraftService) resolveBatchName(ctx context.Context, input studiodomain.BatchNameResolutionInput) (string, error) {
	if strings.TrimSpace(input.RequestedName) != "" || (!input.IsCreate && strings.TrimSpace(input.ExistingName) != "") {
		return studiodomain.ResolveBatchName(input), nil
	}
	names, err := s.repo.ListTenantBatchNames(ctx)
	if err != nil {
		return "", err
	}
	input.ExistingNames = names
	return studiodomain.ResolveBatchName(input), nil
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

func (s *taskStudioBatchDraftService) ensureRunner() {
	if s == nil || s.runner != nil || s.repo == nil {
		return
	}
	s.runner = newListingStudioBatchDraftService(s.repo)
}

func (s *taskStudioBatchDraftService) reconcileBatchListStatuses(ctx context.Context, items []SheinStudioBatchListItem) {
	if s == nil {
		return
	}
	for index := range items {
		batchID := strings.TrimSpace(items[index].ID)
		if batchID == "" {
			continue
		}
		if s.batchRepo == nil {
			continue
		}
		detail, err := s.batchRepo.GetStudioBatchDetail(ctx, batchID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			studioSessionLogger.WithFields(studioSessionLogFields(ctx, logrus.Fields{
				"batch_id": batchID,
				"source":   "batch_repo_error",
				"error":    err.Error(),
			})).Warn("listingkit studio batch list item reconcile failed")
			continue
		}
		if detail == nil || detail.Batch == nil {
			continue
		}
		items[index].Status = string(resolveProjectedStudioBatchStatus(detail.Batch.Status, detail.Items))
		items[index].DesignCount = countStudioBatchGraphMaterializedDesigns(detail.DesignsByItem)
	}
}

func countStudioBatchGraphMaterializedDesigns(designsByItem map[string][]StudioMaterializedDesignRecord) int {
	total := 0
	for _, designs := range designsByItem {
		total += len(designs)
	}
	return total
}
