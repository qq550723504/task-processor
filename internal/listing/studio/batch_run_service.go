package studio

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrBatchSessionNotFound = errors.New("studio batch session not found")

type CreateBatchRunRequest struct {
	BatchIDs []string
	Mode     string
}

type BatchRunRecord struct {
	ID              string
	UserID          string
	Mode            string
	FailurePolicy   string
	Status          string
	TotalBatches    int
	CancelRequested bool
}

type BatchRunItemRecord struct {
	ID       string
	RunID    string
	BatchID  string
	Position int
	Status   string
}

type BatchSeedSession struct {
	SavedAsBatch bool
}

type BatchRunRepository interface {
	CreateBatchRun(ctx context.Context, run *BatchRunRecord, items []BatchRunItemRecord) error
	GetBatchRun(ctx context.Context, runID string) (*BatchRunRecord, error)
	ListBatchRunItems(ctx context.Context, runID string) ([]BatchRunItemRecord, error)
	UpdateBatchRun(ctx context.Context, run *BatchRunRecord) error
}

type BatchSeedSessionRepository interface {
	GetSession(ctx context.Context, batchID string) (*BatchSeedSession, error)
}

type BatchRunService struct {
	repo          BatchRunRepository
	sessionRepo   BatchSeedSessionRepository
	startRun      func(context.Context, string) error
	newRunID      func() string
	requestUserID func(context.Context) string
}

type BatchRunServiceConfig struct {
	Repo          BatchRunRepository
	SessionRepo   BatchSeedSessionRepository
	StartRun      func(context.Context, string) error
	NewRunID      func() string
	RequestUserID func(context.Context) string
}

func NewBatchRunService(config BatchRunServiceConfig) *BatchRunService {
	return &BatchRunService{
		repo:          config.Repo,
		sessionRepo:   config.SessionRepo,
		startRun:      config.StartRun,
		newRunID:      config.NewRunID,
		requestUserID: config.RequestUserID,
	}
}

func (s *BatchRunService) CreateBatchRun(ctx context.Context, req *CreateBatchRunRequest) (*BatchRunRecord, []BatchRunItemRecord, error) {
	if s.repo == nil {
		return nil, nil, fmt.Errorf("studio batch run repository is not configured")
	}
	if s.sessionRepo == nil {
		return nil, nil, fmt.Errorf("studio session repository is not configured")
	}
	if s.startRun == nil {
		return nil, nil, fmt.Errorf("studio batch run starter is not configured")
	}
	if s.newRunID == nil {
		return nil, nil, fmt.Errorf("studio batch run id generator is not configured")
	}
	if req == nil || len(req.BatchIDs) == 0 {
		return nil, nil, fmt.Errorf("batch_ids is required")
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "generate"
	}
	if mode != "generate" && mode != "create_tasks" {
		return nil, nil, fmt.Errorf("mode must be generate or create_tasks")
	}

	batchIDs := make([]string, 0, len(req.BatchIDs))
	seenBatchIDs := make(map[string]struct{}, len(req.BatchIDs))
	for _, batchID := range req.BatchIDs {
		normalized := strings.TrimSpace(batchID)
		if normalized == "" {
			return nil, nil, fmt.Errorf("batch_ids is required")
		}
		if _, exists := seenBatchIDs[normalized]; exists {
			return nil, nil, fmt.Errorf("duplicate batch_id: %s", normalized)
		}
		seenBatchIDs[normalized] = struct{}{}
		session, err := s.sessionRepo.GetSession(ctx, normalized)
		if err != nil {
			return nil, nil, err
		}
		if session == nil || !session.SavedAsBatch {
			return nil, nil, ErrBatchSessionNotFound
		}
		batchIDs = append(batchIDs, normalized)
	}

	runID := s.newRunID()
	run := &BatchRunRecord{
		ID:            runID,
		UserID:        requestUserID(ctx, s.requestUserID),
		Mode:          mode,
		FailurePolicy: "continue_on_error",
		Status:        "pending",
		TotalBatches:  len(batchIDs),
	}
	items := make([]BatchRunItemRecord, 0, len(batchIDs))
	for i, batchID := range batchIDs {
		items = append(items, BatchRunItemRecord{
			ID:       fmt.Sprintf("%s:%d", runID, i+1),
			RunID:    runID,
			BatchID:  batchID,
			Position: i + 1,
			Status:   "pending",
		})
	}
	if err := s.repo.CreateBatchRun(ctx, run, items); err != nil {
		return nil, nil, err
	}
	if err := s.startRun(ctx, run.ID); err != nil {
		return nil, nil, err
	}
	return run, items, nil
}

func (s *BatchRunService) GetBatchRun(ctx context.Context, runID string) (*BatchRunRecord, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch run repository is not configured")
	}
	return s.repo.GetBatchRun(ctx, strings.TrimSpace(runID))
}

func (s *BatchRunService) ListBatchRunItems(ctx context.Context, runID string) ([]BatchRunItemRecord, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch run repository is not configured")
	}
	return s.repo.ListBatchRunItems(ctx, strings.TrimSpace(runID))
}

func (s *BatchRunService) CancelBatchRun(ctx context.Context, runID string) error {
	if s.repo == nil {
		return fmt.Errorf("studio batch run repository is not configured")
	}
	run, err := s.repo.GetBatchRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	run.CancelRequested = true
	return s.repo.UpdateBatchRun(ctx, run)
}

func requestUserID(ctx context.Context, resolve func(context.Context) string) string {
	if resolve == nil {
		return ""
	}
	return strings.TrimSpace(resolve(ctx))
}
