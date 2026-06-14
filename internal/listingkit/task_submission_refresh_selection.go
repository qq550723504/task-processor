package listingkit

import (
	"context"
	"time"

	apperrors "task-processor/internal/core/errors"
	listingsubmission "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinSubmissionRefreshRemoteInputs struct {
	lookupCodes      []string
	spuName          string
	defaultConfirmed bool
	fallbackMessage  string
}

type sheinSubmissionRefreshRequest struct {
	action       string
	requestID    string
	remoteInputs sheinSubmissionRefreshRemoteInputs
}

type sheinSubmissionRefreshSelection struct {
	action       string
	record       *sheinpub.SubmissionRecord
	supplierCode string
}

func (s *taskSubmissionRefreshService) loadSubmissionRefreshInputs(ctx context.Context, taskID string, task *Task, pkg *SheinPackage) (*sheinSubmissionRefreshSelection, sheinproduct.ProductAPI, error) {
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

func resolveSubmissionRefreshAction(report *sheinpub.SubmissionReport) string {
	if report == nil {
		return ""
	}
	return listingsubmission.ResolveRefreshAction(report.LastAction, report.Publish != nil, report.SaveDraft != nil)
}

func loadSubmissionRefreshSelection(pkg *SheinPackage) (*sheinSubmissionRefreshSelection, error) {
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
	return &sheinSubmissionRefreshSelection{
		action:       selection.Action,
		record:       selection.Record,
		supplierCode: selection.SupplierCode,
	}, nil
}

func (s *taskSubmissionRefreshService) buildSheinSubmissionRefreshState(ctx context.Context, task *Task, pkg *SheinPackage, selection *sheinSubmissionRefreshSelection, productAPI sheinproduct.ProductAPI) *sheinSubmissionRefreshState {
	startedAt := time.Now()
	request := buildSubmissionRefreshRequest(pkg, selection)
	var otherAPI sheinother.OtherAPI
	if s.buildSheinSubmitOtherAPI != nil {
		otherAPI, _ = s.buildSheinSubmitOtherAPI(ctx, task)
	}
	return newSubmissionRefreshState(task, request.action, request.requestID, startedAt, productAPI, otherAPI, request.remoteInputs)
}

func buildSubmissionRefreshRequestID(record *sheinpub.SubmissionRecord) string {
	if record == nil {
		return ""
	}
	return listingsubmission.ResolveRefreshRequestID(record.RequestID)
}

func buildSubmissionRefreshRequest(pkg *SheinPackage, selection *sheinSubmissionRefreshSelection) sheinSubmissionRefreshRequest {
	action := ""
	record := (*sheinpub.SubmissionRecord)(nil)
	supplierCode := ""
	if selection != nil {
		action = selection.action
		record = selection.record
		supplierCode = selection.supplierCode
	}
	return sheinSubmissionRefreshRequest{
		action:       action,
		requestID:    buildSubmissionRefreshRequestID(record),
		remoteInputs: buildSubmissionRefreshRemoteInputs(pkg, action, supplierCode),
	}
}

func newSubmissionRefreshState(task *Task, action, requestID string, startedAt time.Time, productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, remoteInputs sheinSubmissionRefreshRemoteInputs) *sheinSubmissionRefreshState {
	return &sheinSubmissionRefreshState{
		task:             task,
		action:           action,
		requestID:        requestID,
		startedAt:        startedAt,
		lookupCodes:      remoteInputs.lookupCodes,
		defaultConfirmed: remoteInputs.defaultConfirmed,
		fallbackMessage:  remoteInputs.fallbackMessage,
		productAPI:       productAPI,
		otherAPI:         otherAPI,
		spuName:          remoteInputs.spuName,
	}
}

func buildSubmissionRefreshRemoteInputs(pkg *SheinPackage, action, supplierCode string) sheinSubmissionRefreshRemoteInputs {
	policy := listingsubmission.BuildRefreshRemotePolicy(action, sheinpub.RemotePublishAccepted(pkg, action))
	return sheinSubmissionRefreshRemoteInputs{
		lookupCodes:      sheinpub.CollectRemoteLookupCodes(pkg, supplierCode),
		spuName:          sheinpub.RemoteLookupSPUName(pkg, action),
		defaultConfirmed: policy.DefaultConfirmed,
		fallbackMessage:  policy.FallbackMessage,
	}
}
