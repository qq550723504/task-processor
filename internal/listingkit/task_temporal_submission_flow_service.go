package listingkit

import (
	"context"

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

func (s *taskTemporalSubmissionFlowService) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	state, err := loadSheinTemporalPreparedPublishState(ctx, in, s.loadSheinPublishTask, s.normalizeSheinSubmitPackage)
	if err != nil {
		return nil, err
	}
	stageContext := buildSheinTemporalSubmissionPayloadStageContext(state.execution)
	preparedPayload, err := s.payloadStages.Prepare(ctx, stageContext)
	if err != nil {
		return nil, err
	}
	return adaptSubmissionPreparedPayload(preparedPayload, stageContext), nil
}

func (s *taskTemporalSubmissionFlowService) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	if in != nil && !in.NeedsImageUpload {
		return in, nil
	}
	state, err := s.loadSheinPreparedPayloadState(ctx, in)
	if err != nil {
		return nil, err
	}
	out, err := s.payloadStages.UploadImages(
		ctx,
		state.stageContext,
		state.payload,
	)
	if err != nil {
		return nil, err
	}
	return adaptSubmissionPreparedPayload(out, state.stageContext), nil
}

func (s *taskTemporalSubmissionFlowService) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	state, err := s.loadSheinPreparedPayloadState(ctx, in)
	if err != nil {
		return err
	}
	return s.payloadStages.PreValidate(ctx, state.stageContext, state.payload)
}

func (s *taskTemporalSubmissionFlowService) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	state, err := s.loadSheinPreparedPayloadState(ctx, in)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, state.execution.task)
	if err != nil {
		return nil, err
	}
	if err := s.persistSheinSubmitPhase(ctx, state.execution.taskID, state.execution.task.Result, state.execution.pkg, state.execution.action, state.execution.requestID, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return nil, err
	}

	result := s.remoteSubmitter.Submit(ctx, buildSheinTemporalRemoteSubmitInput(state.execution, productAPI, state.payload.Product, state.payload.Snapshot))
	if result.Err != nil {
		return nil, newSubmitRemoteActivityError(result.Err, result.SupplierCode, result.Response, result.Snapshot)
	}

	return &SheinRemoteSubmitResult{
		TaskID:       state.execution.taskID,
		Action:       state.execution.action,
		RequestID:    state.execution.requestID,
		SupplierCode: result.SupplierCode,
		Response:     result.Response,
		Snapshot:     result.Snapshot,
	}, nil
}
