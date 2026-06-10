package listingkit

import (
	"context"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) completeSheinDirectRemoteSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	supplierCode := sheinSubmitSupplierCode(submitProduct, pkg)
	setSheinSubmitSupplierCode(pkg, opts.action, opts.requestID, supplierCode)
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, sheinpub.BuildSubmitSnapshot(submitProduct))
	if err := s.taskSubmissionStateOrDefault().persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return err
	}
	response, responseErr := s.executeSheinDirectRemoteSubmitAttempt(ctx, taskID, pkg, productAPI, submitProduct, opts)
	if responseErr == nil {
		if err := s.taskSubmissionStateOrDefault().persistSuccessfulSheinDirectResponse(ctx, taskID, task, pkg, opts, supplierCode, response); err != nil {
			return err
		}
	}
	return s.taskSubmissionStateOrDefault().finishSheinDirectSubmitAttempt(ctx, taskID, task, pkg, opts, response, responseErr)
}

func (s *service) executeSheinDirectRemoteSubmitAttempt(ctx context.Context, taskID string, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) (*sheinpub.SubmissionResponse, error) {
	response, responseErr := s.taskSubmissionExecutionOrDefault().executeSheinSubmitRemote(productAPI, opts.action, submitProduct)
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
