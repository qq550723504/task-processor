package listingkit

import (
	"context"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func newSheinDirectSubmitFlowRunner(s *taskDirectSubmissionService) *submissiondomain.DirectSubmitFlowService[*Task, *SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *ListingKitPreview] {
	if s == nil {
		return nil
	}
	return submissiondomain.NewDirectSubmitFlowService(submissiondomain.DirectSubmitFlowServiceConfig[*Task, *SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *ListingKitPreview]{
		Phases: submissiondomain.DirectSubmitFlowPhases{
			PrepareProduct: sheinpub.SubmissionPhasePrepareProduct,
			UploadImages:   sheinpub.SubmissionPhaseUploadImages,
			PreValidate:    sheinpub.SubmissionPhasePreValidate,
			SubmitRemote:   sheinpub.SubmissionPhaseSubmitRemote,
		},
		BuildProductAPI:  s.loadDirectSubmitProductAPI,
		PersistPhase:     s.persistDirectSubmitPhase,
		PrepareProduct:   s.prepareDirectSubmitProduct,
		NeedsImageUpload: sheinDirectSubmitNeedsImageUpload,
		UploadImages:     s.uploadPendingDirectSubmitImages,
		PreValidate:      s.preValidateDirectSubmitProduct,
		SubmitRemote:     s.completeDirectRemoteSubmit,
		BuildTaskPreview: s.buildDirectSubmitTaskPreview,
	})
}

func newSheinDirectSubmitPayloadStages(s *taskDirectSubmissionService) *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot] {
	if s == nil {
		return nil
	}
	return submissiondomain.NewPayloadStageService(submissiondomain.PayloadStageServiceConfig[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot]{
		Phases: submissiondomain.PayloadStagePhases{
			PrepareProduct: sheinpub.SubmissionPhasePrepareProduct,
			UploadImages:   sheinpub.SubmissionPhaseUploadImages,
			PreValidate:    sheinpub.SubmissionPhasePreValidate,
		},
		PersistPhase:           s.persistDirectSubmitPayloadPhase,
		PreparePayload:         s.prepareDirectSubmitPayload,
		PersistSnapshot:        s.persistDirectSubmitSnapshot,
		RequirePreparedPayload: requireSubmissionPreparedPayload,
		UploadImages:           s.uploadDirectSubmitPayloadImages,
		FinalizeUploaded:       s.finalizeDirectSubmitPayload,
		PreValidate:            s.preValidateDirectSubmitPayload,
	})
}

func (s *taskDirectSubmissionService) failDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {
	if s.failSheinDirectSubmit == nil {
		return submitErr
	}
	return s.failSheinDirectSubmit(ctx, taskID, task, pkg, action, submitErr)
}

func (s *taskDirectSubmissionService) loadDirectSubmitProductAPI(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts submissiondomain.DirectSubmitFlowOptions) (sheinproduct.ProductAPI, error) {
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, s.failDirectSubmit(ctx, taskID, task, pkg, opts.Action, err)
	}
	return productAPI, nil
}

func (s *taskDirectSubmissionService) persistDirectSubmitPhase(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts submissiondomain.DirectSubmitFlowOptions, phase string) error {
	return s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, sheinDirectSubmitOptions{
		action:    opts.Action,
		requestID: opts.RequestID,
		startedAt: opts.StartedAt,
	}, phase)
}

func (s *taskDirectSubmissionService) prepareDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts submissiondomain.DirectSubmitFlowOptions) (*sheinproduct.Product, error) {
	preparedPayload, err := s.payloadStages.Prepare(ctx, newSubmissionPayloadStageContext(taskID, task, pkg, opts.Action, opts.RequestID))
	if err != nil {
		return nil, err
	}
	return preparedPayload.Product, nil
}

func sheinDirectSubmitNeedsImageUpload(submitProduct *sheinproduct.Product) bool {
	return sheinProductPendingImageUploadCount(submitProduct) > 0
}

func (s *taskDirectSubmissionService) uploadPendingDirectSubmitImages(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts submissiondomain.DirectSubmitFlowOptions) error {
	_, err := s.payloadStages.UploadImages(
		ctx,
		newSubmissionPayloadStageContext(taskID, task, pkg, opts.Action, opts.RequestID),
		newSubmissionPreparedPayload(taskID, opts.Action, opts.RequestID, submitProduct),
	)
	return err
}

func (s *taskDirectSubmissionService) preValidateDirectSubmitProduct(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, submitProduct *sheinproduct.Product, opts submissiondomain.DirectSubmitFlowOptions) error {
	return s.payloadStages.PreValidate(
		ctx,
		newSubmissionPayloadStageContext(taskID, task, pkg, opts.Action, opts.RequestID),
		newSubmissionPreparedPayload(taskID, opts.Action, opts.RequestID, submitProduct),
	)
}

func (s *taskDirectSubmissionService) completeDirectRemoteSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, opts submissiondomain.DirectSubmitFlowOptions) error {
	result := s.remoteSubmitter.Submit(ctx, submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]{
		TaskID:     taskID,
		Package:    pkg,
		Action:     opts.Action,
		RequestID:  opts.RequestID,
		ProductAPI: productAPI,
		Product:    submitProduct,
	})
	if result.Err == nil {
		if err := s.persistSuccessfulDirectResponse(ctx, taskID, task, pkg, sheinDirectSubmitOptions{
			action:    opts.Action,
			requestID: opts.RequestID,
			startedAt: opts.StartedAt,
		}, result.SupplierCode, result.Response); err != nil {
			return err
		}
	}
	return s.finishSheinDirectSubmitAttempt(ctx, taskID, task, pkg, sheinDirectSubmitOptions{
		action:    opts.Action,
		requestID: opts.RequestID,
		startedAt: opts.StartedAt,
	}, result.Response, result.Err)
}

func (s *taskDirectSubmissionService) executeDirectRemoteSubmitAttempt(ctx context.Context, in submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]) submissiondomain.RemoteSubmitResult[*sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot] {
	attempt := executeSheinSubmitRemoteAttempt(
		ctx,
		in.TaskID,
		in.Package,
		in.Action,
		in.RequestID,
		in.ProductAPI,
		in.Product,
		s.executeSheinSubmitRemote,
		s.retrySheinSensitiveWordSubmit,
	)
	return submissiondomain.RemoteSubmitResult[*sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot]{
		Response: attempt.response,
		Snapshot: attempt.snapshot,
		Err:      attempt.err,
	}
}

func (s *taskDirectSubmissionService) buildDirectSubmitTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	if s.buildTaskPreview == nil {
		return nil, ErrTaskResultUnavailable
	}
	return s.buildTaskPreview(ctx, task, platform)
}

func (s *taskDirectSubmissionService) persistDirectSubmitPayloadPhase(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], phase string) error {
	return s.persistSheinDirectSubmitPhase(ctx, in.TaskID, in.Task, in.Package, sheinDirectSubmitOptions{
		action:    in.Action,
		requestID: in.RequestID,
	}, phase)
}

func (s *taskDirectSubmissionService) prepareDirectSubmitPayload(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage]) (*submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot], error) {
	preparedPayload, err := prepareSheinSubmitPayloadProduct(ctx, in.TaskID, in.Action, in.RequestID, in.Task, in.Package, s.prepareSheinSubmitProduct)
	if err != nil {
		return nil, s.failDirectSubmit(ctx, in.TaskID, in.Task, in.Package, in.Action, err)
	}
	return adaptListingKitPreparedPayload(preparedPayload), nil
}

func (s *taskDirectSubmissionService) persistDirectSubmitSnapshot(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {
	if snapshot != nil {
		setSheinSubmitSnapshot(in.Package, in.Action, in.RequestID, snapshot)
	}
	return nil
}

func (s *taskDirectSubmissionService) uploadDirectSubmitPayloadImages(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {
	if err := s.uploadSheinSubmitImages(ctx, in.Task, in.Package, product); err != nil {
		return s.failDirectSubmit(ctx, in.TaskID, in.Task, in.Package, in.Action, err)
	}
	return nil
}

func (s *taskDirectSubmissionService) finalizeDirectSubmitPayload(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], payload *submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot]) (*submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot], error) {
	out := finalizeSheinUploadedSubmitPayload(ctx, in.TaskID, in.Action, in.RequestID, in.Task, adaptSubmissionPreparedPayload(payload, in), s.resolveSubmitSettings)
	return adaptListingKitPreparedPayload(out), nil
}

func (s *taskDirectSubmissionService) preValidateDirectSubmitPayload(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {
	if err := s.preValidateSheinSubmitProduct(in.Package, product); err != nil {
		return s.failDirectSubmit(ctx, in.TaskID, in.Task, in.Package, in.Action, err)
	}
	return nil
}
