package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
)

type sheinRemoteStatusRequest struct {
	productAPI       sheinproduct.ProductAPI
	otherAPI         sheinother.OtherAPI
	action           string
	requestID        string
	lookupCodes      []string
	spuName          string
	defaultConfirmed bool
	fallbackMessage  string
	startedAt        time.Time
	taskID           string
}

type sheinRemoteRefreshRequest struct {
	task         *Task
	taskID       string
	pkg          *SheinPackage
	productAPI   sheinproduct.ProductAPI
	action       string
	requestID    string
	supplierCode string
	startedAt    time.Time
}

func (s *taskSubmissionRecoveryService) refreshSheinSubmitRemoteStatus(ctx context.Context, request *sheinRemoteRefreshRequest) (*sheinpub.SubmissionEvent, error) {
	if request == nil {
		return nil, errors.New("submit remote refresh request is not configured")
	}
	statusRequest, event := s.buildSheinRemoteStatusRequest(ctx, request)
	if event != nil {
		return event, nil
	}
	confirmation, err := s.resolveRemoteStatus(statusRequest)
	if request.pkg != nil && confirmation != nil {
		sheinpub.ApplySubmissionConfirmRemoteState(request.pkg, request.action, request.requestID, *confirmation)
	}
	return confirmation.Event, err
}

func newSheinRemoteStatusRequest(
	taskID string,
	action string,
	requestID string,
	startedAt time.Time,
	productAPI sheinproduct.ProductAPI,
	otherAPI sheinother.OtherAPI,
	remoteInputs sheinpub.SubmissionRemoteLookupInputs,
) *sheinRemoteStatusRequest {
	return &sheinRemoteStatusRequest{
		productAPI:       productAPI,
		otherAPI:         otherAPI,
		action:           action,
		requestID:        requestID,
		lookupCodes:      remoteInputs.LookupCodes,
		spuName:          remoteInputs.SPUName,
		defaultConfirmed: remoteInputs.DefaultConfirmed,
		fallbackMessage:  remoteInputs.FallbackMessage,
		startedAt:        startedAt,
		taskID:           taskID,
	}
}

func (s *taskSubmissionRecoveryService) buildSheinRemoteStatusRequest(ctx context.Context, request *sheinRemoteRefreshRequest) (*sheinRemoteStatusRequest, *sheinpub.SubmissionEvent) {
	if request == nil {
		return nil, nil
	}
	remoteInputs := sheinpub.BuildSubmissionRecoveryRemoteLookupInputs(request.pkg, request.action, request.supplierCode)
	if len(remoteInputs.LookupCodes) == 0 {
		return nil, sheinpub.ApplySubmissionMissingSupplierCodeRemoteUpdate(request.pkg, request.taskID, request.action, request.requestID, request.startedAt, remoteInputs.DefaultConfirmed)
	}
	var otherAPI sheinother.OtherAPI
	if s.buildSheinSubmitOtherAPI != nil && request.task != nil {
		otherAPI, _ = s.buildSheinSubmitOtherAPI(ctx, request.task)
	}
	return newSheinRemoteStatusRequest(
		request.taskID,
		request.action,
		request.requestID,
		request.startedAt,
		request.productAPI,
		otherAPI,
		remoteInputs,
	), nil
}

func (s *taskSubmissionRecoveryService) resolveRemoteStatus(request *sheinRemoteStatusRequest) (*sheinpub.SubmissionConfirmRemoteUpdate, error) {
	if s.resolveRemoteStatusCallback == nil {
		return nil, errors.New("submit remote status resolution is not configured")
	}
	return s.resolveRemoteStatusCallback(request)
}

func resolveSheinSubmitRemoteStatus(request *sheinRemoteStatusRequest) (*sheinpub.SubmissionConfirmRemoteUpdate, error) {
	if request == nil {
		return nil, errors.New("submit remote status request is not configured")
	}
	resolution := sheinpub.ProbeSubmissionRemoteResolution(
		request.productAPI,
		request.otherAPI,
		request.action,
		request.lookupCodes,
		request.spuName,
		request.defaultConfirmed,
		request.fallbackMessage,
		logSheinBatchCheckOnWayResponse,
	)
	update, err := sheinpub.BuildSubmissionConfirmRemoteUpdateFromResolution(request.taskID, request.action, request.requestID, request.startedAt, resolution)
	return &update, err
}

func logSheinBatchCheckOnWayResponse(expectedSPUName string, resp *sheinother.BatchCheckOnWayResponse, err error) {
	fields := logrus.Fields{
		"expected_spu_name": strings.TrimSpace(expectedSPUName),
	}
	if resp != nil {
		fields["response_code"] = strings.TrimSpace(resp.Code)
		fields["response_msg"] = strings.TrimSpace(resp.Msg)
		if encoded, marshalErr := json.Marshal(resp); marshalErr == nil {
			fields["response_json"] = string(encoded)
		} else {
			fields["response_json_error"] = marshalErr.Error()
		}
	}
	if err != nil {
		fields["error"] = err.Error()
		logrus.WithFields(fields).Warn("listingkit shein batch_check_on_way response error")
		return
	}
	logrus.WithFields(fields).Info("listingkit shein batch_check_on_way response")
}
