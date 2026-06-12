package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskDirectSubmissionServiceConfig struct {
	normalizeSheinSubmitPackage     func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	validateSheinPublishFreshness   func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	failSheinDirectSubmit           func(context.Context, string, *Task, *SheinPackage, string, error) error
	buildSheinSubmitProductAPI      func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	persistSheinDirectSubmitPhase   func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions, string) error
	prepareSheinSubmitProduct       func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages         func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings           func(context.Context, *Task) SheinSettings
	preValidateSheinSubmitProduct   func(*SheinPackage, *sheinproduct.Product) error
	executeSheinSubmitRemote        func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*sheinpub.SubmissionResponse, error)
	retrySheinSensitiveWordSubmit   func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	persistSuccessfulDirectResponse func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions, string, *sheinpub.SubmissionResponse) error
	finishSheinDirectSubmitAttempt  func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions, *sheinpub.SubmissionResponse, error) error
	buildTaskPreview                func(context.Context, *Task, string) (*ListingKitPreview, error)
}

type taskDirectSubmissionService struct {
	normalizeSheinSubmitPackage     func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	validateSheinPublishFreshness   func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	failSheinDirectSubmit           func(context.Context, string, *Task, *SheinPackage, string, error) error
	buildSheinSubmitProductAPI      func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	persistSheinDirectSubmitPhase   func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions, string) error
	prepareSheinSubmitProduct       func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error)
	uploadSheinSubmitImages         func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error
	resolveSubmitSettings           func(context.Context, *Task) SheinSettings
	preValidateSheinSubmitProduct   func(*SheinPackage, *sheinproduct.Product) error
	executeSheinSubmitRemote        func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*sheinpub.SubmissionResponse, error)
	retrySheinSensitiveWordSubmit   func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	persistSuccessfulDirectResponse func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions, string, *sheinpub.SubmissionResponse) error
	finishSheinDirectSubmitAttempt  func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions, *sheinpub.SubmissionResponse, error) error
	buildTaskPreview                func(context.Context, *Task, string) (*ListingKitPreview, error)
}

func newTaskDirectSubmissionService(config taskDirectSubmissionServiceConfig) *taskDirectSubmissionService {
	return &taskDirectSubmissionService{
		normalizeSheinSubmitPackage:     config.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   config.validateSheinPublishFreshness,
		failSheinDirectSubmit:           config.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      config.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   config.persistSheinDirectSubmitPhase,
		prepareSheinSubmitProduct:       config.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:         config.uploadSheinSubmitImages,
		resolveSubmitSettings:           config.resolveSubmitSettings,
		preValidateSheinSubmitProduct:   config.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:        config.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:   config.retrySheinSensitiveWordSubmit,
		persistSuccessfulDirectResponse: config.persistSuccessfulDirectResponse,
		finishSheinDirectSubmitAttempt:  config.finishSheinDirectSubmitAttempt,
		buildTaskPreview:                config.buildTaskPreview,
	}
}

func (s *taskDirectSubmissionService) submitSheinTaskDirect(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
	pkg := task.Result.Shein
	prepared := prepareSheinSubmitReadinessForAction(task, pkg, req, opts.action, s.normalizeSheinSubmitPackage)
	readiness := prepared.readiness
	if err := validateSheinSubmitReadinessGates(ctx, task, pkg, opts.action, readiness, s.validateSheinPublishFreshness); err != nil {
		return nil, s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}

	productAPI, err := s.loadDirectSubmitProductAPI(ctx, taskID, task, pkg, opts)
	if err != nil {
		return nil, err
	}
	return s.executeDirectSubmitProductFlow(ctx, taskID, task, pkg, productAPI, opts)
}

func (s *taskDirectSubmissionService) loadDirectSubmitProductAPI(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions) (sheinproduct.ProductAPI, error) {
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	return productAPI, nil
}

func (s *taskDirectSubmissionService) executeDirectSubmitProductFlow(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePrepareProduct); err != nil {
		return nil, err
	}
	submitProduct, err := s.prepareDirectSubmitProduct(ctx, taskID, task, pkg, opts)
	if err != nil {
		return nil, err
	}
	responseErr := s.completeDirectRemoteSubmit(ctx, taskID, task, pkg, productAPI, submitProduct, opts)
	if responseErr != nil {
		return nil, responseErr
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskDirectSubmissionService) failDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {
	if s.failSheinDirectSubmit == nil {
		return submitErr
	}
	return s.failSheinDirectSubmit(ctx, taskID, task, pkg, action, submitErr)
}

func (s *taskDirectSubmissionService) prepareDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions) (*sheinproduct.Product, error) {
	submitProduct, err := s.prepareSheinSubmitProduct(ctx, task, pkg, opts.action)
	if err != nil {
		return nil, s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "prepared", submitProduct)
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if err := s.uploadPendingDirectSubmitImages(ctx, taskID, task, pkg, submitProduct, opts); err != nil {
		return nil, err
	}
	if err := s.preValidateDirectSubmitProduct(ctx, taskID, task, pkg, submitProduct, opts); err != nil {
		return nil, err
	}
	return submitProduct, nil
}

func (s *taskDirectSubmissionService) uploadPendingDirectSubmitImages(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	if sheinProductPendingImageUploadCount(submitProduct) <= 0 {
		return nil
	}
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseUploadImages); err != nil {
		return err
	}
	if err := s.uploadSheinSubmitImages(ctx, task, pkg, submitProduct); err != nil {
		return s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	prepareSheinProductForSubmit(submitProduct, s.resolveSubmitSettings(ctx, task))
	dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "uploaded", submitProduct)
	return nil
}

func (s *taskDirectSubmissionService) preValidateDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePreValidate); err != nil {
		return err
	}
	if err := s.preValidateSheinSubmitProduct(pkg, submitProduct); err != nil {
		return s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	return nil
}

func (s *taskDirectSubmissionService) completeDirectRemoteSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	supplierCode := sheinSubmitSupplierCode(submitProduct, pkg)
	setSheinSubmitSupplierCode(pkg, opts.action, opts.requestID, supplierCode)
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return err
	}
	response, responseErr := s.executeDirectRemoteSubmitAttempt(ctx, taskID, pkg, productAPI, submitProduct, opts)
	if responseErr == nil {
		if err := s.persistSuccessfulDirectResponse(ctx, taskID, task, pkg, opts, supplierCode, response); err != nil {
			return err
		}
	}
	return s.finishSheinDirectSubmitAttempt(ctx, taskID, task, pkg, opts, response, responseErr)
}

func (s *taskDirectSubmissionService) executeDirectRemoteSubmitAttempt(ctx context.Context, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) (*sheinpub.SubmissionResponse, error) {
	attempt := executeSheinSubmitRemoteAttempt(
		ctx,
		taskID,
		pkg,
		opts.action,
		opts.requestID,
		productAPI,
		submitProduct,
		s.executeSheinSubmitRemote,
		s.retrySheinSensitiveWordSubmit,
	)
	return attempt.response, attempt.err
}
