package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	corelogger "task-processor/internal/core/logger"
	studiodomain "task-processor/internal/listing/studio"
)

type taskStudioSessionServiceConfig struct {
	repo                     studioSessionDraftRepository
	runner                   *listingStudioSessionRunner
	asyncJobRunner           *listingStudioSessionAsyncJobRunner
	generationMetadataRunner *listingStudioSessionGenerationMetadataRunner
	reviewTaskMetadataRunner *listingStudioSessionReviewTaskMetadataRunner
	generalMetadataRunner    *listingStudioSessionGeneralMetadataRunner
}

type taskStudioSessionService struct {
	repo                     studioSessionDraftRepository
	runner                   *listingStudioSessionRunner
	asyncJobRunner           *listingStudioSessionAsyncJobRunner
	generationMetadataRunner *listingStudioSessionGenerationMetadataRunner
	reviewTaskMetadataRunner *listingStudioSessionReviewTaskMetadataRunner
	generalMetadataRunner    *listingStudioSessionGeneralMetadataRunner
}

var studioSessionLogger = corelogger.GetGlobalLogger("listingkit.studio.session")

func newTaskStudioSessionService(config taskStudioSessionServiceConfig) *taskStudioSessionService {
	service := &taskStudioSessionService{
		repo:                     config.repo,
		runner:                   config.runner,
		asyncJobRunner:           config.asyncJobRunner,
		generationMetadataRunner: config.generationMetadataRunner,
		reviewTaskMetadataRunner: config.reviewTaskMetadataRunner,
		generalMetadataRunner:    config.generalMetadataRunner,
	}
	service.ensureRunner()
	service.ensureAsyncJobRunner()
	service.ensureGenerationMetadataRunner()
	service.ensureReviewTaskMetadataRunner()
	service.ensureGeneralMetadataRunner()
	return service
}

func (s *taskStudioSessionService) EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	result, err := s.runner.EnsureSession(ctx, &studiodomain.EnsureSessionRequest[SheinStudioSelection]{
		UserID:    req.UserID,
		Selection: req.Selection,
	})
	if err != nil {
		return nil, adaptStudioSessionError(err)
	}
	return &SheinStudioSessionDetail{Session: result.Session, Designs: result.Designs}, nil
}

func (s *taskStudioSessionService) GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	result, err := s.runner.GetSession(ctx, sessionID)
	if err != nil {
		return nil, adaptStudioSessionError(err)
	}
	return &SheinStudioSessionDetail{Session: result.Session, Designs: result.Designs}, nil
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
	if isStudioSessionGenerationMetadataOnlyUpdate(req) {
		s.ensureGenerationMetadataRunner()
		if s.generationMetadataRunner == nil {
			return nil, fmt.Errorf("studio session repository is not configured")
		}
		session, err = s.generationMetadataRunner.Patch(ctx, studiodomain.SessionGenerationMetadataPatchRequest[
			SheinStudioSessionStatus,
			SheinStudioGenerationJob,
		]{
			SessionID:       sessionID,
			Status:          req.Status,
			GenerationJobID: req.GenerationJobID,
			GenerationJobs:  req.GenerationJobs,
			GenerationError: req.GenerationError,
		})
		if err != nil {
			return nil, adaptStudioSessionError(err)
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
	if isStudioSessionReviewTaskMetadataOnlyUpdate(req) {
		s.ensureReviewTaskMetadataRunner()
		if s.reviewTaskMetadataRunner == nil {
			return nil, fmt.Errorf("studio session repository is not configured")
		}
		session, err = s.reviewTaskMetadataRunner.Patch(ctx, studiodomain.SessionReviewTaskMetadataPatchRequest[SheinStudioCreatedTask]{
			SessionID:         sessionID,
			ApprovedDesignIDs: req.ApprovedDesignIDs,
			CreatedTasks:      req.CreatedTasks,
		})
		if err != nil {
			return nil, adaptStudioSessionError(err)
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

	s.ensureGeneralMetadataRunner()
	if s.generalMetadataRunner == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err = s.generalMetadataRunner.Patch(ctx, studiodomain.SessionGeneralMetadataPatchRequest[UpdateStudioSessionRequest]{
		SessionID: sessionID,
		Patch:     req,
	})
	if err != nil {
		return nil, adaptStudioSessionError(err)
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
	s.ensureAsyncJobRunner()
	if s.asyncJobRunner == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	return adaptStudioSessionError(s.asyncJobRunner.SyncAsyncJob(ctx, studiodomain.SessionAsyncJobSyncRequest{
		SessionID:    sessionID,
		JobStatus:    string(jobStatus),
		JobID:        jobID,
		ErrorMessage: errMessage,
	}))
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

func isStudioSessionGenerationMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {
	if req == nil {
		return false
	}
	hasGenerationMetadata := req.Status != nil || req.GenerationJobID != nil || req.GenerationJobs != nil || req.GenerationError != nil
	if !hasGenerationMetadata {
		return false
	}
	return req.Prompt == nil &&
		req.StyleCount == nil &&
		req.VariationIntensity == nil &&
		req.ProductImageCount == nil &&
		req.ProductImagePrompt == nil &&
		req.ProductImagePrompts == nil &&
		req.ArtworkModel == nil &&
		req.ImageStrategy == nil &&
		req.GroupedImageMode == nil &&
		req.SelectedSDSImages == nil &&
		req.GroupedSelections == nil &&
		req.TransparentBackground == nil &&
		req.RenderSizeImagesWithSDS == nil &&
		req.SheinStoreID == nil &&
		req.ApprovedDesignIDs == nil &&
		req.CreatedTasks == nil
}

func isStudioSessionReviewTaskMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {
	if req == nil {
		return false
	}
	hasReviewTaskMetadata := req.ApprovedDesignIDs != nil || req.CreatedTasks != nil
	if !hasReviewTaskMetadata {
		return false
	}
	return req.Status == nil &&
		req.Prompt == nil &&
		req.StyleCount == nil &&
		req.VariationIntensity == nil &&
		req.ProductImageCount == nil &&
		req.ProductImagePrompt == nil &&
		req.ProductImagePrompts == nil &&
		req.ArtworkModel == nil &&
		req.ImageStrategy == nil &&
		req.GroupedImageMode == nil &&
		req.SelectedSDSImages == nil &&
		req.GroupedSelections == nil &&
		req.TransparentBackground == nil &&
		req.RenderSizeImagesWithSDS == nil &&
		req.SheinStoreID == nil &&
		req.GenerationJobID == nil &&
		req.GenerationJobs == nil &&
		req.GenerationError == nil
}

func (s *taskStudioSessionService) ensureRunner() {
	if s == nil || s.runner != nil || s.repo == nil {
		return
	}
	s.runner = newListingStudioSessionService(s.repo)
}

func (s *taskStudioSessionService) ensureAsyncJobRunner() {
	if s == nil || s.asyncJobRunner != nil || s.repo == nil {
		return
	}
	s.asyncJobRunner = newListingStudioSessionAsyncJobService(s.repo)
}

func (s *taskStudioSessionService) ensureGenerationMetadataRunner() {
	if s == nil || s.generationMetadataRunner != nil || s.repo == nil {
		return
	}
	s.generationMetadataRunner = newListingStudioSessionGenerationMetadataService(s.repo)
}

func (s *taskStudioSessionService) ensureReviewTaskMetadataRunner() {
	if s == nil || s.reviewTaskMetadataRunner != nil || s.repo == nil {
		return
	}
	s.reviewTaskMetadataRunner = newListingStudioSessionReviewTaskMetadataService(s.repo)
}

func (s *taskStudioSessionService) ensureGeneralMetadataRunner() {
	if s == nil || s.generalMetadataRunner != nil || s.repo == nil {
		return
	}
	s.generalMetadataRunner = newListingStudioSessionGeneralMetadataService(s.repo)
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
