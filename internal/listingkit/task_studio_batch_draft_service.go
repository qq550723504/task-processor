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
	if req == nil {
		return nil, fmt.Errorf("selection is required")
	}
	var session *SheinStudioSession
	var err error
	isCreate := strings.TrimSpace(req.ID) == ""
	batchReq := req
	if isCreate {
		if req.Selection == nil || req.Selection.VariantID <= 0 {
			return nil, fmt.Errorf("selection is required")
		}
		session = &SheinStudioSession{
			ID:                      uuid.NewString(),
			UserID:                  RequestUserIDFromContext(ctx),
			RenderSizeImagesWithSDS: true,
		}
	} else {
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
		if req.Selection == nil {
			existingSelection := SheinStudioSelection(session.Selection)
			if existingSelection.VariantID <= 0 {
				return nil, fmt.Errorf("selection is required")
			}
			cloned := *req
			cloned.Selection = &existingSelection
			batchReq = &cloned
		} else if req.Selection.VariantID <= 0 {
			return nil, fmt.Errorf("selection is required")
		}
	}
	fields := applyListingStudioBatchDraftFields(batchReq, isCreate)
	existingBatchName := strings.TrimSpace(session.BatchName)
	selection := *batchReq.Selection
	selection.DesignType = fields.SelectionDesignType

	session.SelectionKey = fields.SelectionKey
	session.Status = SheinStudioSessionStatus(fields.Status)
	session.ProductID = fields.Selection.ProductID
	session.ParentProductID = fields.Selection.ParentProductID
	session.VariantID = fields.Selection.VariantID
	session.PrototypeGroupID = fields.Selection.PrototypeGroupID
	session.LayerID = fields.Selection.LayerID
	session.PrintableWidth = fields.Selection.PrintableWidth
	session.PrintableHeight = fields.Selection.PrintableHeight
	session.SelectedVariantIDs = append(SheinStudioInt64List(nil), fields.Selection.SelectedVariantIDs...)
	session.Selection = SheinStudioSelectionSnapshot(selection)
	session.Prompt = fields.Prompt
	session.PromptMode = fields.PromptMode
	session.StyleCount = fields.StyleCount
	session.VariationIntensity = fields.VariationIntensity
	session.ArtworkModel = fields.ArtworkModel
	session.GroupedImageMode = fields.GroupedImageMode
	session.SelectedSDSImages = toStudioSelectedSDSImageList(fields.SelectedSDSImages)
	session.GroupedSelections = toStudioGroupedSelectionList(fields.GroupedSelections)
	session.TransparentBackgroundMode = StudioTransparencyMode(fields.TransparentBackgroundMode)
	session.TransparentBackground = fields.TransparentBackground
	session.RenderSizeImagesWithSDS = fields.RenderSizeImagesWithSDS
	session.HotStyleReferenceImageURLs = SheinStudioStringList(fields.HotStyleReferenceImageURLs)
	session.HotStyleReferenceBrief = fields.HotStyleReferenceBrief
	session.HotStyleReferencePrompt = fields.HotStyleReferencePrompt
	session.SheinStoreID = fields.SheinStoreID
	session.ApprovedDesignIDs = append(SheinStudioStringList(nil), fields.ApprovedDesignIDs...)
	session.GenerationJobs = append(SheinStudioGenerationJobList(nil), fields.GenerationJobs...)
	session.SavedAsBatch = true
	session.BatchName, err = s.resolveBatchName(ctx, studiodomain.BatchNameResolutionInput{
		RequestedName: batchReq.BatchName,
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
	if err := s.repo.ReplaceDesigns(ctx, session.ID, fields.ApprovedDesignIDs, batchReq.Designs); err != nil {
		return nil, err
	}
	studioSessionLogger.WithFields(studioSessionLogFields(ctx, logrus.Fields{
		"session_id":              session.ID,
		"batch_name":              session.BatchName,
		"is_create":               isCreate,
		"status":                  session.Status,
		"design_count":            len(batchReq.Designs),
		"approved_design_count":   len(fields.ApprovedDesignIDs),
		"generation_jobs_count":   len(fields.GenerationJobs),
		"grouped_selection_count": len(fields.GroupedSelections),
		"shein_store_id":          session.SheinStoreID,
	})).Info("studio batch upserted")
	return s.loadStudioBatchDraftDetail(ctx, session)
}

func applyListingStudioBatchDraftFields(
	req *UpsertStudioBatchRequest,
	isCreate bool,
) studiodomain.BatchDraftFields[
	SheinStudioSelectedSDSImage,
	SheinStudioGroupedSelection,
	SheinStudioGenerationJob,
] {
	selection := req.Selection
	return studiodomain.ApplyBatchDraftFields(studiodomain.BatchDraftInput[
		SheinStudioSelectedSDSImage,
		SheinStudioGroupedSelection,
		SheinStudioGenerationJob,
	]{
		Selection: studiodomain.SelectionKeyInput{
			ProductID:          selection.ProductID,
			ParentProductID:    selection.ParentProductID,
			VariantID:          selection.VariantID,
			PrototypeGroupID:   selection.PrototypeGroupID,
			LayerID:            selection.LayerID,
			PrintableWidth:     selection.PrintableWidth,
			PrintableHeight:    selection.PrintableHeight,
			SelectedVariantIDs: selection.SelectedVariantIDs,
		},
		SelectionDesignType:        selection.DesignType,
		Prompt:                     req.Prompt,
		PromptMode:                 req.PromptMode,
		StyleCount:                 req.StyleCount,
		VariationIntensity:         req.VariationIntensity,
		ArtworkModel:               req.ArtworkModel,
		GroupedImageMode:           req.GroupedImageMode,
		SelectedSDSImages:          req.SelectedSDSImages,
		GroupedSelections:          req.GroupedSelections,
		TransparentBackground:      req.TransparentBackground,
		TransparentBackgroundMode:  string(req.TransparentBackgroundMode),
		RenderSizeImagesWithSDS:    req.RenderSizeImagesWithSDS,
		HotStyleReferenceImageURLs: req.HotStyleReferenceImageURLs,
		HotStyleReferenceBrief:     req.HotStyleReferenceBrief,
		HotStyleReferencePrompt:    req.HotStyleReferencePrompt,
		SheinStoreID:               req.SheinStoreID,
		ApprovedDesignIDs:          req.ApprovedDesignIDs,
		GenerationJobs:             req.GenerationJobs,
		DesignCount:                len(req.Designs),
	}, isCreate)
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
