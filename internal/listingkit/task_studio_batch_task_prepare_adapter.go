package listingkit

import (
	"context"
	"time"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchTaskPrepareState = studiodomain.BatchTaskPrepareState[
	SheinStudioSession,
	StudioBatchRecord,
]

type listingStudioBatchTaskPrepareRunner = studiodomain.BatchTaskPrepareService[
	SheinStudioSession,
	StudioBatchRecord,
	CreateStudioBatchTasksResult,
]

func newListingStudioBatchTaskPrepareService(
	updateSession func(context.Context, *SheinStudioSession) error,
	updateBatch func(context.Context, *StudioBatchRecord) error,
	loadResult func(context.Context, string) (*CreateStudioBatchTasksResult, error),
	currentTime func() time.Time,
) *listingStudioBatchTaskPrepareRunner {
	return studiodomain.NewBatchTaskPrepareService(studiodomain.BatchTaskPrepareServiceConfig[
		SheinStudioSession,
		StudioBatchRecord,
		CreateStudioBatchTasksResult,
	]{
		UpdateSession: updateSession,
		SetPendingDesignIDs: func(session *SheinStudioSession, designIDs []string) {
			session.PendingTaskDesignIDs = append(SheinStudioStringList(nil), designIDs...)
		},
		ClearFailedTasks: func(session *SheinStudioSession) {
			session.FailedTasks = nil
		},
		SetSessionCreating: func(session *SheinStudioSession) {
			session.Status = SheinStudioSessionStatusTasksCreating
		},
		SetSessionUpdatedAt: func(session *SheinStudioSession, updatedAt time.Time) {
			session.UpdatedAt = updatedAt
		},
		UpdateBatch: updateBatch,
		SetBatchCreating: func(batch *StudioBatchRecord) {
			batch.Status = StudioBatchStatusTasksCreating
		},
		SetBatchUpdatedAt: func(batch *StudioBatchRecord, updatedAt time.Time) {
			batch.UpdatedAt = updatedAt
		},
		LoadResult:  loadResult,
		CurrentTime: currentTime,
	})
}
