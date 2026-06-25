package listingkit

import (
	"context"
	"time"

	apperrors "task-processor/internal/core/errors"
	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *taskSubmissionRefreshService) loadSubmissionRefreshInputs(ctx context.Context, taskID string, task *Task, pkg *SheinPackage) (*sheinpub.SubmissionRefreshSelection, sheinproduct.ProductAPI, error) {
	selection, err := loadSubmissionRefreshSelection(pkg)
	if err != nil {
		return nil, nil, err
	}
	productAPI, err := s.buildSubmissionRefreshProductAPI(ctx, task, taskID)
	if err != nil {
		return nil, nil, err
	}
	return selection, productAPI, nil
}

func (s *taskSubmissionRefreshService) buildSubmissionRefreshProductAPI(ctx context.Context, task *Task, taskID string) (sheinproduct.ProductAPI, error) {
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, apperrors.Wrapf(err, apperrors.ErrCodePlatformError, "failed to build shein product API for task %s", taskID)
	}
	return productAPI, nil
}

func loadSubmissionRefreshSelection(pkg *SheinPackage) (*sheinpub.SubmissionRefreshSelection, error) {
	var ok bool
	pkg, ok = sheinpub.SubmissionStatePackage(pkg)
	if !ok {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	selection := sheinpub.ResolveSubmissionRefreshSelection(pkg)
	if selection.Record == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission record is not available")
	}
	if selection.SupplierCode == "" {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein supplier code is not available")
	}
	return &selection, nil
}

func (s *taskSubmissionRefreshService) buildSheinSubmissionRefreshState(ctx context.Context, task *Task, pkg *SheinPackage, selection *sheinpub.SubmissionRefreshSelection, productAPI sheinproduct.ProductAPI) *sheinSubmissionRefreshState {
	startedAt := time.Now()
	request := buildSubmissionRefreshRequest(pkg, selection)
	var otherAPI sheinother.OtherAPI
	if s.buildSheinSubmitOtherAPI != nil {
		otherAPI, _ = s.buildSheinSubmitOtherAPI(ctx, task)
	}
	return newSubmissionRefreshState(task, request.Action, request.RequestID, startedAt, productAPI, otherAPI, request.RemoteInputs)
}

func buildSubmissionRefreshRequest(pkg *SheinPackage, selection *sheinpub.SubmissionRefreshSelection) sheinpub.SubmissionRefreshRequest {
	if selection == nil {
		return sheinpub.SubmissionRefreshRequest{}
	}
	requestID := ""
	if selection.Record != nil {
		requestID = submissiondomain.ResolveRefreshRequestID(selection.Record.RequestID)
	}
	return sheinpub.SubmissionRefreshRequest{
		Action:       selection.Action,
		RequestID:    requestID,
		RemoteInputs: sheinpub.BuildSubmissionRefreshRemoteLookupInputs(pkg, selection.Action, selection.SupplierCode),
	}
}

func newSubmissionRefreshState(task *Task, action, requestID string, startedAt time.Time, productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, remoteInputs sheinpub.SubmissionRemoteLookupInputs) *sheinSubmissionRefreshState {
	taskID := ""
	if task != nil {
		taskID = task.ID
	}
	return &sheinSubmissionRefreshState{
		task:          task,
		remoteRequest: newSheinRemoteStatusRequest(taskID, action, requestID, startedAt, productAPI, otherAPI, remoteInputs),
	}
}
