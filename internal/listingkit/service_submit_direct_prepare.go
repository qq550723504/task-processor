package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) prepareSheinDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions) (*sheinproduct.Product, error) {
	state := s.taskSubmissionStateOrDefault()
	submitProduct, err := s.taskSubmissionExecutionOrDefault().prepareSheinSubmitProduct(ctx, task, pkg, opts.action)
	if err != nil {
		return nil, state.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
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
	state := s.taskSubmissionStateOrDefault()
	if sheinProductPendingImageUploadCount(submitProduct) <= 0 {
		return nil
	}
	if err := state.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhaseUploadImages); err != nil {
		return err
	}
	if err := s.taskSubmissionExecutionOrDefault().uploadSheinSubmitImages(ctx, task, pkg, submitProduct); err != nil {
		return state.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	prepareSheinProductForSubmit(submitProduct, s.resolveSheinSubmitSettings(ctx, task))
	dumpSheinSubmitPayloadForDebug(taskID, opts.action, opts.requestID, "uploaded", submitProduct)
	return nil
}

func (s *service) preValidateSheinDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts sheinDirectSubmitOptions) error {
	state := s.taskSubmissionStateOrDefault()
	if err := state.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePreValidate); err != nil {
		return err
	}
	if err := s.taskSubmissionExecutionOrDefault().preValidateSheinSubmitProduct(pkg, submitProduct); err != nil {
		return state.failSheinDirectSubmit(ctx, taskID, task, pkg, opts.action, err)
	}
	return nil
}
