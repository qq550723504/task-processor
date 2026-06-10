package listingkit

import (
	"context"
	"strings"
	"time"

	apperrors "task-processor/internal/core/errors"
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
	action := strings.TrimSpace(report.LastAction)
	if action != "" {
		return action
	}
	if report.Publish != nil {
		return "publish"
	}
	if report.SaveDraft != nil {
		return "save_draft"
	}
	return ""
}

func loadSubmissionRefreshReport(pkg *SheinPackage) (*sheinpub.SubmissionReport, error) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
	}
	return pkg.SubmissionState, nil
}

func resolveSubmissionRefreshRecord(report *sheinpub.SubmissionReport, action string) (*sheinpub.SubmissionRecord, error) {
	record := sheinSubmissionRecordForAction(report, action)
	if record == nil {
		return nil, apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission record is not available")
	}
	return record, nil
}

func resolveSubmissionRefreshSupplierCode(record *sheinpub.SubmissionRecord, pkg *SheinPackage) (string, error) {
	supplierCode := ""
	if record != nil {
		supplierCode = strings.TrimSpace(record.SupplierCode)
	}
	if supplierCode == "" {
		supplierCode = sheinSubmitSupplierCode(nil, pkg)
	}
	if supplierCode == "" {
		return "", apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein supplier code is not available")
	}
	return supplierCode, nil
}

func loadSubmissionRefreshSelection(pkg *SheinPackage) (*sheinSubmissionRefreshSelection, error) {
	report, err := loadSubmissionRefreshReport(pkg)
	if err != nil {
		return nil, err
	}
	action := resolveSubmissionRefreshAction(report)
	record, err := resolveSubmissionRefreshRecord(report, action)
	if err != nil {
		return nil, err
	}
	supplierCode, err := resolveSubmissionRefreshSupplierCode(record, pkg)
	if err != nil {
		return nil, err
	}
	return &sheinSubmissionRefreshSelection{
		action:       action,
		record:       record,
		supplierCode: supplierCode,
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
	return strings.TrimSpace(record.RequestID)
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
	return sheinSubmissionRefreshRemoteInputs{
		lookupCodes:      collectSheinRemoteLookupCodes(pkg, supplierCode),
		spuName:          sheinRemoteLookupSPUName(pkg, action),
		defaultConfirmed: action == "publish" && sheinRemotePublishAccepted(pkg, action),
		// Preserve current submission-service behavior; resolveSheinSubmitRemoteStatus supplies
		// the publish fallback when defaultConfirmed is true.
		fallbackMessage: "",
	}
}
