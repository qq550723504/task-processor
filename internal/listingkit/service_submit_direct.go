package listingkit

import (
	"context"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) submitSheinTaskDirect(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
	return s.taskDirectSubmissionOrDefault().submitSheinTaskDirect(ctx, taskID, task, req, opts)
}

func (s *service) taskDirectSubmissionOrDefault() *taskDirectSubmissionService {
	if s.submission.taskDirectSubmission != nil {
		return s.submission.taskDirectSubmission
	}
	s.submission.taskDirectSubmission = newTaskDirectSubmissionService(buildTaskDirectSubmissionServiceConfig(s))
	return s.submission.taskDirectSubmission
}

func (s *service) prepareSheinDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions) (*sheinproduct.Product, error) {
	submitProduct, err := s.prepareSheinSubmitProduct(ctx, task, pkg, opts.action)
	if err != nil {
		return nil, s.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "prepared", submitProduct)
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if err := s.uploadPendingSheinDirectSubmitImages(ctx, taskID, task, pkg, submitProduct, opts); err != nil {
		return nil, err
	}
	if err := s.preValidateSheinDirectSubmitProduct(ctx, taskID, task, pkg, submitProduct, opts); err != nil {
		return nil, err
	}
	return submitProduct, nil
}

func (s *service) uploadPendingSheinDirectSubmitImages(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	if sheinProductPendingImageUploadCount(submitProduct) <= 0 {
		return nil
	}
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseUploadImages); err != nil {
		return err
	}
	if err := s.uploadSheinSubmitImages(ctx, task, pkg, submitProduct); err != nil {
		return s.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	prepareSheinProductForSubmit(submitProduct, s.resolveSheinSubmitSettings(ctx, task))
	dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "uploaded", submitProduct)
	return nil
}

func (s *service) preValidateSheinDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePreValidate); err != nil {
		return err
	}
	if err := s.preValidateSheinSubmitProduct(pkg, submitProduct); err != nil {
		return s.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	return nil
}

func (s *service) completeSheinDirectRemoteSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	supplierCode := sheinSubmitSupplierCode(submitProduct, pkg)
	setSheinSubmitSupplierCode(pkg, opts.action, opts.requestID, supplierCode)
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return err
	}
	response, responseErr := s.executeSheinDirectRemoteSubmitAttempt(ctx, taskID, pkg, productAPI, submitProduct, opts)
	if responseErr == nil {
		if err := s.persistSuccessfulSheinDirectResponse(ctx, taskID, task, pkg, opts, supplierCode, response); err != nil {
			return err
		}
	}
	return s.finishSheinDirectSubmitAttempt(ctx, taskID, task, pkg, opts, response, responseErr)
}

func (s *service) executeSheinDirectRemoteSubmitAttempt(ctx context.Context, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) (*sheinpub.SubmissionResponse, error) {
	response, responseErr := s.executeSheinSubmitRemote(productAPI, opts.action, submitProduct)
	if responseErr == nil {
		responseErr = submission.BuildResponseError(opts.action, response)
	}
	if retryResponse, retryErr, retried := s.retrySheinSensitiveWordSubmit(ctx, taskID, pkg, opts.action, opts.requestID, productAPI, submitProduct, response, responseErr); retried {
		response = retryResponse
		responseErr = retryErr
		setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	}
	return response, responseErr
}
