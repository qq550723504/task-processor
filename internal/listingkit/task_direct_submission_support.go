package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

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
	preparedPayload, err := prepareSheinSubmitPayloadProduct(ctx, taskID, opts.action, opts.requestID, task, pkg, s.prepareSheinSubmitProduct)
	if err != nil {
		return nil, s.failDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	setSheinSubmitSnapshot(pkg, opts.action, opts.requestID, preparedPayload.Snapshot)
	submitProduct := preparedPayload.Product
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
	finalizeSheinUploadedSubmitPayload(ctx, taskID, opts.action, opts.requestID, task, &SheinPreparedSubmitPayload{
		TaskID:    taskID,
		Action:    opts.action,
		RequestID: opts.requestID,
		Product:   submitProduct,
	}, s.resolveSubmitSettings)
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
	remoteState := prepareSheinRemoteSubmitState(pkg, opts.action, opts.requestID, submitProduct, nil)
	if err := s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return err
	}
	response, responseErr := s.executeDirectRemoteSubmitAttempt(ctx, taskID, pkg, productAPI, submitProduct, opts)
	if responseErr == nil {
		if err := s.persistSuccessfulDirectResponse(ctx, taskID, task, pkg, opts, remoteState.supplierCode, response); err != nil {
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
