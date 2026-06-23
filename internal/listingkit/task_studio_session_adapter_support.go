package listingkit

import (
	"context"
	"errors"
	"slices"

	studiodomain "task-processor/internal/listing/studio"
)

type studioSessionRepositoryAdapter struct {
	repo studioSessionDraftRepository
}

type studioSessionMutationRepositoryAdapter struct {
	repo studioSessionDraftRepository
}

func (a studioSessionRepositoryAdapter) FindLatestSessionBySelectionKey(ctx context.Context, selectionKey string) (*SheinStudioSession, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.FindLatestSessionBySelectionKey(ctx, selectionKey)
}

func (a studioSessionRepositoryAdapter) CreateSession(ctx context.Context, session *SheinStudioSession) error {
	if a.repo == nil {
		return nil
	}
	return a.repo.CreateSession(ctx, session)
}

func (a studioSessionRepositoryAdapter) GetSession(ctx context.Context, sessionID string) (*SheinStudioSession, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.GetSession(ctx, sessionID)
}

func (a studioSessionRepositoryAdapter) ListSessionDesigns(ctx context.Context, sessionID string) ([]SheinStudioDesign, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.ListSessionDesigns(ctx, sessionID)
}

func (a studioSessionMutationRepositoryAdapter) GetSession(ctx context.Context, sessionID string) (*SheinStudioSession, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.GetSession(ctx, sessionID)
}

func (a studioSessionMutationRepositoryAdapter) UpdateSession(ctx context.Context, session *SheinStudioSession) error {
	if a.repo == nil {
		return nil
	}
	return a.repo.UpdateSession(ctx, session)
}

func validateStudioSessionSelection(selection *SheinStudioSelection) error {
	if selection == nil || selection.VariantID <= 0 {
		return errors.New("selection is required")
	}
	return nil
}

func newListingStudioSessionRecord(id string, userID string, selectionKey string, selection *SheinStudioSelection) *SheinStudioSession {
	if selection == nil {
		return nil
	}
	return &SheinStudioSession{
		ID:                      id,
		UserID:                  userID,
		SelectionKey:            selectionKey,
		Status:                  SheinStudioSessionStatusSelecting,
		ProductID:               selection.ProductID,
		ParentProductID:         selection.ParentProductID,
		VariantID:               selection.VariantID,
		PrototypeGroupID:        selection.PrototypeGroupID,
		LayerID:                 selection.LayerID,
		PrintableWidth:          selection.PrintableWidth,
		PrintableHeight:         selection.PrintableHeight,
		SelectedVariantIDs:      append(SheinStudioInt64List(nil), selection.SelectedVariantIDs...),
		Selection:               SheinStudioSelectionSnapshot(*selection),
		RenderSizeImagesWithSDS: true,
	}
}

func studioSessionStatusForAsyncJob(jobStatus string) SheinStudioSessionStatus {
	switch StudioAsyncJobStatus(jobStatus) {
	case StudioAsyncJobStatusSucceeded:
		return SheinStudioSessionStatusGenerated
	case StudioAsyncJobStatusFailed:
		return SheinStudioSessionStatusFailed
	default:
		return SheinStudioSessionStatusGenerating
	}
}

func setListingStudioSessionStatus(session *SheinStudioSession, status SheinStudioSessionStatus) {
	if session != nil {
		session.Status = status
	}
}

func setListingStudioSessionGenerationJobID(session *SheinStudioSession, jobID string) {
	if session != nil {
		session.GenerationJobID = jobID
	}
}

func setListingStudioSessionGenerationError(session *SheinStudioSession, errMessage string) {
	if session != nil {
		session.GenerationError = errMessage
	}
}

func setListingStudioSessionGenerationJobs(session *SheinStudioSession, jobs []SheinStudioGenerationJob) {
	if session != nil {
		session.GenerationJobs = append(SheinStudioGenerationJobList(nil), jobs...)
	}
}

func setListingStudioSessionApprovedDesignIDs(session *SheinStudioSession, ids []string) {
	if session != nil {
		session.ApprovedDesignIDs = slices.Clone(ids)
	}
}

func setListingStudioSessionCreatedTasks(session *SheinStudioSession, tasks []SheinStudioCreatedTask) {
	if session == nil {
		return
	}
	session.CreatedTasks = append(SheinStudioCreatedTaskList(nil), tasks...)
	session.CreatedTaskIDs = buildCreatedTaskIDs(tasks)
}

func applyListingStudioSessionGeneralMetadataPatch(session *SheinStudioSession, req *UpdateStudioSessionRequest) {
	if session == nil || req == nil {
		return
	}
	if req.Status != nil {
		session.Status = *req.Status
	}
	if req.Prompt != nil {
		session.Prompt = *req.Prompt
	}
	if req.PromptMode != nil {
		session.PromptMode = *req.PromptMode
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
	if req.GroupedImageMode != nil {
		session.GroupedImageMode = *req.GroupedImageMode
	}
	if req.SelectedSDSImages != nil {
		selected := make(SheinStudioSelectedSDSImageList, 0, len(req.SelectedSDSImages))
		for _, item := range req.SelectedSDSImages {
			selected = append(selected, SheinStudioSelectedSDSImageRecord(item))
		}
		session.SelectedSDSImages = selected
	}
	if req.GroupedSelections != nil {
		session.GroupedSelections = append(SheinStudioGroupedSelectionList(nil), req.GroupedSelections...)
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
	if req.GenerationJobs != nil {
		session.GenerationJobs = append(SheinStudioGenerationJobList(nil), req.GenerationJobs...)
	}
	if req.GenerationError != nil {
		session.GenerationError = *req.GenerationError
	}
	if req.ApprovedDesignIDs != nil {
		session.ApprovedDesignIDs = slices.Clone(req.ApprovedDesignIDs)
	}
	if req.CreatedTasks != nil {
		setListingStudioSessionCreatedTasks(session, req.CreatedTasks)
	}
}

func adaptStudioSessionError(err error) error {
	if errors.Is(err, studiodomain.ErrSessionNotFound) {
		return ErrStudioSessionNotFound
	}
	return err
}
