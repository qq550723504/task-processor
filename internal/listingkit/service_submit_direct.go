package listingkit

import (
	"context"

	listingsubmission "task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) submitSheinTaskDirect(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
	return s.taskDirectSubmissionOrDefault().submitSheinTaskDirect(ctx, taskID, task, req, opts)
}

func (s *service) taskDirectSubmissionOrDefault() *taskDirectSubmissionService {
	if s.taskDirectSubmission != nil {
		return s.taskDirectSubmission
	}
	s.taskDirectSubmission = newTaskDirectSubmissionService(buildTaskDirectSubmissionServiceConfig(s))
	return s.taskDirectSubmission
}

func (s *service) prepareSheinDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions) (*sheinproduct.Product, error) {
	submitProduct, err := s.prepareSheinSubmitProduct(ctx, task, pkg, opts.action)
	if err != nil {
		return nil, s.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "prepared", submitProduct)
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if sheinProductPendingImageUploadCount(submitProduct) > 0 {
		if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseUploadImages); err != nil {
			return nil, err
		}
		if err := s.uploadSheinSubmitImages(ctx, task, pkg, submitProduct); err != nil {
			return nil, s.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
		}
		prepareSheinProductForSubmit(submitProduct, s.resolveSheinSubmitSettings(ctx, task))
		dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "uploaded", submitProduct)
	}
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePreValidate); err != nil {
		return nil, err
	}
	if err := preValidateSheinSubmitProduct(submitProduct); err != nil {
		return nil, s.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	return submitProduct, nil
}

func (s *service) completeSheinDirectRemoteSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	supplierCode := sheinSubmitSupplierCode(submitProduct, pkg)
	setSheinSubmitSupplierCode(pkg, opts.action, opts.requestID, supplierCode)
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return err
	}
	response, responseErr := executeSheinSubmitRemote(productAPI, opts.action, submitProduct)
	if responseErr == nil {
		responseErr = listingsubmission.BuildResponseError(opts.action, response)
	}
	if retryResponse, retryErr, retried := s.retrySheinSensitiveWordSubmit(ctx, taskID, pkg, opts.action, opts.requestID, productAPI, submitProduct, response, responseErr); retried {
		response = retryResponse
		responseErr = retryErr
		setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	}
	if responseErr == nil {
		if err := s.persistSuccessfulSheinDirectResponse(ctx, taskID, task, pkg, opts, supplierCode, response); err != nil {
			return err
		}
	}
	return s.finishSheinDirectSubmitAttempt(ctx, taskID, task, pkg, opts, response, responseErr)
}
