package listingkit

import (
	"context"
	"time"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchTaskResumeState = studiodomain.BatchTaskResumeFinalizeState[
	SheinStudioSession,
	StudioBatchRecord,
	SheinStudioCreatedTask,
	SheinStudioFailedTask,
]

type listingStudioBatchTaskResumeRunner = studiodomain.BatchTaskResumeFinalizeService[
	SheinStudioSession,
	StudioBatchRecord,
	CreateStudioBatchTasksResult,
	SheinStudioCreatedTask,
	SheinStudioFailedTask,
]

func newListingStudioBatchTaskResumeService(
	updateSession func(context.Context, *SheinStudioSession) error,
	updateBatch func(context.Context, *StudioBatchRecord) error,
	loadResult func(context.Context, string) (*CreateStudioBatchTasksResult, error),
	currentTime func() time.Time,
) *listingStudioBatchTaskResumeRunner {
	return studiodomain.NewBatchTaskResumeFinalizeService(studiodomain.BatchTaskResumeFinalizeServiceConfig[
		SheinStudioSession,
		StudioBatchRecord,
		CreateStudioBatchTasksResult,
		SheinStudioCreatedTask,
		SheinStudioFailedTask,
	]{
		UpdateSession: updateSession,
		ClearPendingTasks: func(session *SheinStudioSession) {
			session.PendingTaskDesignIDs = nil
		},
		SetCreatedTasks: func(session *SheinStudioSession, created []SheinStudioCreatedTask) {
			session.CreatedTasks = append(SheinStudioCreatedTaskList(nil), created...)
			session.CreatedTaskIDs = buildCreatedTaskIDs(created)
		},
		SetFailedTasks: func(session *SheinStudioSession, failed []SheinStudioFailedTask) {
			session.FailedTasks = append(SheinStudioFailedTaskList(nil), failed...)
		},
		SetSessionDone: func(session *SheinStudioSession) {
			session.Status = SheinStudioSessionStatusTasksCreated
		},
		SetSessionUpdated: func(session *SheinStudioSession, updatedAt time.Time) {
			session.UpdatedAt = updatedAt
		},
		UpdateBatch: updateBatch,
		SetBatchDone: func(batch *StudioBatchRecord) {
			batch.Status = StudioBatchStatusTasksCreated
		},
		SetBatchUpdated: func(batch *StudioBatchRecord, updatedAt time.Time) {
			batch.UpdatedAt = updatedAt
		},
		LoadResult:  loadResult,
		CurrentTime: currentTime,
	})
}
