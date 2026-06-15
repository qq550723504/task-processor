package listingkit

import (
	"context"
	"fmt"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchTaskExecuteCandidate struct {
	session       *SheinStudioSession
	design        StudioMaterializedDesignRecord
	grouped       SheinStudioGroupedSelection
	title         string
	sessionDesign SheinStudioDesign
}

type listingStudioBatchTaskExecuteRunner = studiodomain.BatchTaskExecuteService[
	SheinStudioSession,
	listingStudioBatchTaskExecuteCandidate,
	SheinStudioCreatedTask,
	SheinStudioFailedTask,
	CreateStudioBatchTasksResult,
]

func newListingStudioBatchTaskExecuteService(s *taskStudioBatchService) *listingStudioBatchTaskExecuteRunner {
	return studiodomain.NewBatchTaskExecuteService(studiodomain.BatchTaskExecuteServiceConfig[
		SheinStudioSession,
		listingStudioBatchTaskExecuteCandidate,
		SheinStudioCreatedTask,
		SheinStudioFailedTask,
		CreateStudioBatchTasksResult,
	]{
		LoadSession: func(ctx context.Context, batchID string) (*SheinStudioSession, error) {
			if s == nil {
				return nil, ErrStudioSessionNotFound
			}
			return s.loadStudioBatchTaskSession(ctx, batchID)
		},
		LoadItems: func(ctx context.Context, batchID string, designIDs []string) ([]listingStudioBatchTaskExecuteCandidate, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			candidates, _, err := s.prepareStudioBatchTaskExecuteCandidates(ctx, batchID, designIDs)
			return candidates, err
		},
		FindExisting: func(ctx context.Context, session *SheinStudioSession, candidate listingStudioBatchTaskExecuteCandidate) (SheinStudioCreatedTask, bool) {
			if s == nil || session == nil {
				return SheinStudioCreatedTask{}, false
			}
			return s.findExistingStudioBatchTask(ctx, session.CreatedTasks, candidate.design, candidate.grouped, candidate.title)
		},
		CreateTask: func(ctx context.Context, candidate listingStudioBatchTaskExecuteCandidate) (SheinStudioCreatedTask, error) {
			if s == nil || s.createGenerateTask == nil {
				return SheinStudioCreatedTask{}, fmt.Errorf("listing task creator is not configured")
			}
			task, err := s.createGenerateTask(
				ctx,
				buildStudioBatchTaskGenerateRequest(
					candidate.session,
					candidate.grouped,
					candidate.design,
					candidate.sessionDesign,
				),
			)
			if err != nil {
				return SheinStudioCreatedTask{}, err
			}
			return SheinStudioCreatedTask{
				ID:       task.ID,
				Title:    candidate.title,
				DesignID: candidate.design.ID,
			}, nil
		},
		BuildFailed: func(candidate listingStudioBatchTaskExecuteCandidate, err error) SheinStudioFailedTask {
			return SheinStudioFailedTask{
				DesignID: candidate.design.ID,
				Title:    candidate.title,
				Message:  err.Error(),
			}
		},
		Finalize: func(ctx context.Context, batchID string, session *SheinStudioSession, created []SheinStudioCreatedTask, failed []SheinStudioFailedTask) (*CreateStudioBatchTasksResult, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			batch, err := s.repo.GetStudioBatch(ctx, batchID)
			if err != nil {
				return nil, err
			}
			return s.completeStudioBatchTaskExecution(ctx, batchID, session, batch, created, failed)
		},
	})
}
