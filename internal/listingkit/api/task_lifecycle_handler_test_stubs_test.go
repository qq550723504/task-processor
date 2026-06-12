package api

import (
	"context"
	"errors"

	"task-processor/internal/listingkit"
)

type stubTaskLifecycleHandlerService struct{}

func (stubTaskLifecycleHandlerService) CreateGenerateTask(context.Context, *listingkit.GenerateRequest) (*listingkit.Task, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) ListTasks(context.Context, *listingkit.TaskListQuery) (*listingkit.TaskListPage, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) GetSDSBaselineReadiness(context.Context, *listingkit.SDSBaselineReadinessQuery) (*listingkit.SDSBaselineReadiness, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) GetTaskResult(context.Context, string) (*listingkit.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) GetTaskPreview(context.Context, string, string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) GetTaskRevisionHistory(context.Context, string, *listingkit.RevisionHistoryQuery) (*listingkit.ListingKitRevisionHistoryPage, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) GetTaskRevisionHistoryDetail(context.Context, string, string, *listingkit.RevisionHistoryDetailQuery) (*listingkit.ListingKitRevisionHistoryDetail, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) GetTaskExport(context.Context, string, string) (*listingkit.ListingKitExport, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) ApplyTaskRevision(context.Context, string, *listingkit.ApplyRevisionRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) ValidateTaskRevision(context.Context, string, *listingkit.ApplyRevisionRequest) (*listingkit.RevisionValidationResult, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) SubmitTask(context.Context, string, *listingkit.SubmitTaskRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (stubTaskLifecycleHandlerService) RefreshSubmissionStatus(context.Context, string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}
