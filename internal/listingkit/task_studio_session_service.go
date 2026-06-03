package listingkit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	corelogger "task-processor/internal/core/logger"
)

type taskStudioSessionServiceConfig struct {
	repo studioSessionDraftRepository
}

type taskStudioSessionService struct {
	repo studioSessionDraftRepository
}

var studioSessionLogger = corelogger.GetGlobalLogger("listingkit.studio.session")

func newTaskStudioSessionService(config taskStudioSessionServiceConfig) *taskStudioSessionService {
	return &taskStudioSessionService{repo: config.repo}
}

func (s *taskStudioSessionService) EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	if s.repo == nil {
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
	existing, err := s.repo.FindLatestSessionBySelectionKey(ctx, selectionKey)
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
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return &SheinStudioSessionDetail{
		Session: session,
		Designs: []SheinStudioDesign{},
	}, nil
}

func (s *taskStudioSessionService) GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}
	return s.loadStudioSessionDetail(ctx, session)
}

func (s *taskStudioSessionService) UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}
	if err := validateStudioSessionExpectedUpdatedAt(session.UpdatedAt, req.ExpectedUpdatedAt); err != nil {
		return nil, err
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

	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	studioSessionLogger.WithFields(studioSessionLogFields(ctx, logrus.Fields{
		"session_id":            session.ID,
		"status":                session.Status,
		"generation_job_id":     strings.TrimSpace(session.GenerationJobID),
		"generation_jobs_count": len(session.GenerationJobs),
		"approved_design_count": len(session.ApprovedDesignIDs),
		"created_task_count":    len(session.CreatedTasks),
	})).Info("studio session updated")
	return studioSessionDetailWithoutDesigns(session), nil
}

func validateStudioSessionExpectedUpdatedAt(current time.Time, expected *string) error {
	if expected == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*expected)
	if trimmed == "" || current.IsZero() {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return fmt.Errorf("invalid expected_updated_at: %w", err)
	}
	if !current.UTC().Equal(parsed.UTC()) {
		return ErrStudioSessionConflict
	}
	return nil
}

func studioSessionStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func (s *taskStudioSessionService) SyncStudioDesignAsyncJob(
	ctx context.Context,
	sessionID string,
	jobStatus StudioAsyncJobStatus,
	jobID string,
	errMessage string,
) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}

	sessionStatus := SheinStudioSessionStatusGenerating
	switch jobStatus {
	case StudioAsyncJobStatusSucceeded:
		sessionStatus = SheinStudioSessionStatusGenerated
	case StudioAsyncJobStatusFailed:
		sessionStatus = SheinStudioSessionStatusFailed
	}

	trimmedJobID := strings.TrimSpace(jobID)
	trimmedErr := strings.TrimSpace(errMessage)
	_, err := s.UpdateStudioSession(ctx, sessionID, &UpdateStudioSessionRequest{
		Status:          &sessionStatus,
		GenerationJobID: &trimmedJobID,
		GenerationError: &trimmedErr,
	})
	return err
}

func (s *taskStudioSessionService) loadStudioSessionDetail(ctx context.Context, session *SheinStudioSession) (*SheinStudioSessionDetail, error) {
	designs, err := s.repo.ListSessionDesigns(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	return &SheinStudioSessionDetail{
		Session: session,
		Designs: designs,
	}, nil
}

func studioSessionDetailWithoutDesigns(session *SheinStudioSession) *SheinStudioSessionDetail {
	return &SheinStudioSessionDetail{
		Session: session,
		Designs: []SheinStudioDesign{},
	}
}

func studioSessionLogFields(ctx context.Context, fields logrus.Fields) logrus.Fields {
	if fields == nil {
		fields = logrus.Fields{}
	}
	for key, value := range RequestTraceFromContext(ctx).LogFields() {
		if value == "" || value == 0 {
			continue
		}
		fields[key] = value
	}
	return fields
}
