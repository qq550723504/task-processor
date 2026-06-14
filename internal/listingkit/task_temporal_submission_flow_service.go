package listingkit

import (
	"context"
	"fmt"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskTemporalSubmissionFlowServiceConfig struct {
	loadSheinPublishTask          func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage   func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	persistSheinSubmitPhase       func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	prepareSheinSubmitProduct     func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages       func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings         func(context.Context, *Task) SheinSettings
	buildSheinSubmitProductAPI    func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	preValidateSheinSubmitProduct func(*SheinPackage, *sheinproduct.Product) error
	executeSheinSubmitRemote      func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*sheinpub.SubmissionResponse, error)
	retrySheinSensitiveWordSubmit func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	payloadStages                 *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot]
	remoteSubmitter               *submissiondomain.RemoteSubmitService[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot]
	persistence                   *taskTemporalSubmissionPersistenceService
}

type taskTemporalSubmissionFlowService struct {
	loadSheinPublishTask          func(context.Context, string) (*Task, *SheinPackage, error)
	normalizeSheinSubmitPackage   func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	persistSheinSubmitPhase       func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	prepareSheinSubmitProduct     func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages       func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings         func(context.Context, *Task) SheinSettings
	buildSheinSubmitProductAPI    func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	preValidateSheinSubmitProduct func(*SheinPackage, *sheinproduct.Product) error
	executeSheinSubmitRemote      func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*sheinpub.SubmissionResponse, error)
	retrySheinSensitiveWordSubmit func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	payloadStages                 *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot]
	remoteSubmitter               *submissiondomain.RemoteSubmitService[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot]
	persistence                   *taskTemporalSubmissionPersistenceService
}

func newTaskTemporalSubmissionFlowService(config taskTemporalSubmissionFlowServiceConfig) *taskTemporalSubmissionFlowService {
	service := &taskTemporalSubmissionFlowService{
		loadSheinPublishTask:          config.loadSheinPublishTask,
		normalizeSheinSubmitPackage:   config.normalizeSheinSubmitPackage,
		persistSheinSubmitPhase:       config.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:     config.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:       config.uploadSheinSubmitImages,
		resolveSubmitSettings:         config.resolveSubmitSettings,
		buildSheinSubmitProductAPI:    config.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct: config.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:      config.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit: config.retrySheinSensitiveWordSubmit,
		payloadStages:                 config.payloadStages,
		remoteSubmitter:               config.remoteSubmitter,
		persistence:                   config.persistence,
	}
	if service.payloadStages == nil {
		service.payloadStages = newSheinTemporalSubmitPayloadStages(service)
	}
	if service.remoteSubmitter == nil {
		service.remoteSubmitter = newSheinRemoteSubmitService(service.executeTemporalRemoteSubmitAttempt)
	}
	return service
}

func (s *taskTemporalSubmissionFlowService) loadSheinPublishTaskState(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	if s.loadSheinPublishTask == nil {
		return nil, nil, fmt.Errorf("shein publish task loader is not configured")
	}
	return s.loadSheinPublishTask(ctx, taskID)
}

func (s *taskTemporalSubmissionFlowService) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	s.normalizeSheinSubmitPackage(task, pkg, sheinSubmitRequestFromActivity(in), in.Action)
	preparedPayload, err := s.payloadStages.Prepare(ctx, newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID))
	if err != nil {
		return nil, err
	}
	return adaptSubmissionPreparedPayload(preparedPayload, newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID)), nil
}

func (s *taskTemporalSubmissionFlowService) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	if err := requireSheinPreparedSubmitPayload(in); err != nil {
		return nil, err
	}
	if !in.NeedsImageUpload {
		return in, nil
	}
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	out, err := s.payloadStages.UploadImages(
		ctx,
		newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID),
		adaptListingKitPreparedPayload(in),
	)
	if err != nil {
		return nil, err
	}
	return adaptSubmissionPreparedPayload(out, newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID)), nil
}

func (s *taskTemporalSubmissionFlowService) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	return s.payloadStages.PreValidate(
		ctx,
		newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID),
		adaptListingKitPreparedPayload(in),
	)
}

func (s *taskTemporalSubmissionFlowService) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	if err := requireSheinPreparedSubmitPayload(in); err != nil {
		return nil, err
	}
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return nil, err
	}

	result := s.remoteSubmitter.Submit(ctx, submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]{
		TaskID:     in.TaskID,
		Package:    pkg,
		Action:     in.Action,
		RequestID:  in.RequestID,
		ProductAPI: productAPI,
		Product:    in.Product,
		Snapshot:   in.Snapshot,
	})
	if result.Err != nil {
		return nil, newSubmitRemoteActivityError(result.Err, result.SupplierCode, result.Response, result.Snapshot)
	}

	return &SheinRemoteSubmitResult{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		SupplierCode: result.SupplierCode,
		Response:     result.Response,
		Snapshot:     result.Snapshot,
	}, nil
}
