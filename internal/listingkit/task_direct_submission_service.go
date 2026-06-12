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
