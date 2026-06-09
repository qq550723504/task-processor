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
	prepareSheinDirectSubmitProduct func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions) (*sheinproduct.Product, error)
	completeSheinDirectRemoteSubmit func(context.Context, string, *Task, *SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, sheinDirectSubmitOptions) error
	buildTaskPreview                func(context.Context, *Task, string) (*ListingKitPreview, error)
}

type taskDirectSubmissionService struct {
	normalizeSheinSubmitPackage     func(*Task, *SheinPackage, *SubmitTaskRequest, string)
	validateSheinPublishFreshness   func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	failSheinDirectSubmit           func(context.Context, string, *Task, *SheinPackage, string, error) error
	buildSheinSubmitProductAPI      func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	persistSheinDirectSubmitPhase   func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions, string) error
	prepareSheinDirectSubmitProduct func(context.Context, string, *Task, *SheinPackage, sheinDirectSubmitOptions) (*sheinproduct.Product, error)
	completeSheinDirectRemoteSubmit func(context.Context, string, *Task, *SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, sheinDirectSubmitOptions) error
	buildTaskPreview                func(context.Context, *Task, string) (*ListingKitPreview, error)
}

func newTaskDirectSubmissionService(config taskDirectSubmissionServiceConfig) *taskDirectSubmissionService {
	return &taskDirectSubmissionService{
		normalizeSheinSubmitPackage:     config.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   config.validateSheinPublishFreshness,
		failSheinDirectSubmit:           config.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      config.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   config.persistSheinDirectSubmitPhase,
		prepareSheinDirectSubmitProduct: config.prepareSheinDirectSubmitProduct,
		completeSheinDirectRemoteSubmit: config.completeSheinDirectRemoteSubmit,
		buildTaskPreview:                config.buildTaskPreview,
	}
}

func (s *taskDirectSubmissionService) submitSheinTaskDirect(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
	pkg := task.Result.Shein
	if s.normalizeSheinSubmitPackage != nil {
		s.normalizeSheinSubmitPackage(task, pkg, req, opts.action)
	}

	ensureTaskPodExecution(task)
	readiness := buildSheinSubmitReadinessWithPodForAction(pkg, task.Result.PodExecution, opts.action)
	if err := validateSheinSubmitReadinessGates(ctx, task, pkg, opts.action, readiness, s.validateSheinPublishFreshness); err != nil {
		return nil, s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}

	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	return s.executeDirectSubmitProductFlow(ctx, taskID, task, pkg, productAPI, opts)
}

func (s *taskDirectSubmissionService) executeDirectSubmitProductFlow(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePrepareProduct); err != nil {
		return nil, err
	}
	submitProduct, err := s.prepareSheinDirectSubmitProduct(ctx, taskID, task, pkg, opts)
	if err != nil {
		return nil, err
	}
	responseErr := s.completeSheinDirectRemoteSubmit(ctx, taskID, task, pkg, productAPI, submitProduct, opts)
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
