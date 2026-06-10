package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) prepareSheinDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions) (*sheinproduct.Product, error) {
	submitProduct, err := s.taskSubmissionExecutionOrDefault().prepareSheinSubmitProduct(ctx, task, pkg, opts.action)
	if err != nil {
		return nil, s.taskSubmissionStateOrDefault().failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
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
	if err := s.taskSubmissionStateOrDefault().persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseUploadImages); err != nil {
		return err
	}
	if err := s.taskSubmissionExecutionOrDefault().uploadSheinSubmitImages(ctx, task, pkg, submitProduct); err != nil {
		return s.taskSubmissionStateOrDefault().failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	prepareSheinProductForSubmit(submitProduct, s.resolveSheinSubmitSettings(ctx, task))
	dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "uploaded", submitProduct)
	return nil
}

func (s *service) preValidateSheinDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	if err := s.taskSubmissionStateOrDefault().persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePreValidate); err != nil {
		return err
	}
	if err := s.taskSubmissionExecutionOrDefault().preValidateSheinSubmitProduct(pkg, submitProduct); err != nil {
		return s.taskSubmissionStateOrDefault().failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	return nil
}
