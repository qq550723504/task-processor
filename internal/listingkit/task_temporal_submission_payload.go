package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskTemporalSubmissionAdapter) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	s.normalizeSheinSubmitPackage(task, pkg, sheinSubmitRequestFromActivity(in), in.Action)
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhasePrepareProduct); err != nil {
		return nil, err
	}

	preparedPayload, err := prepareSheinSubmitPayloadProduct(ctx, in.TaskID, in.Action, in.RequestID, task, pkg, s.prepareSheinSubmitProduct)
	if err != nil {
		return nil, err
	}
	if err := s.persistSheinSubmitSnapshot(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, preparedPayload.Snapshot); err != nil {
		return nil, err
	}
	return preparedPayload, nil
}

func (s *taskTemporalSubmissionAdapter) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	if err := requireSheinPreparedSubmitPayload(in); err != nil {
		return nil, err
	}
	if !in.NeedsImageUpload {
		return in, nil
	}

	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseUploadImages); err != nil {
		return nil, err
	}
	if err := s.uploadSheinSubmitImages(ctx, task, pkg, in.Product); err != nil {
		return nil, err
	}
	out := finalizeSheinUploadedSubmitPayload(ctx, in.TaskID, in.Action, in.RequestID, task, in, s.resolveSubmitSettings)
	if err := s.persistSheinSubmitSnapshot(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, out.Snapshot); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *taskTemporalSubmissionAdapter) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	if err := requireSheinPreparedSubmitPayload(in); err != nil {
		return err
	}
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhasePreValidate); err != nil {
		return err
	}
	return s.preValidateSheinSubmitProduct(pkg, in.Product)
}

func (s *taskTemporalSubmissionAdapter) SubmitSheinPublishRemote(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinRemoteSubmitResult, error) {
	if err := requireSheinPreparedSubmitPayload(in); err != nil {
		return nil, err
	}
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}

	remoteState := prepareSheinRemoteSubmitState(pkg, in.Action, in.RequestID, in.Product, in.Snapshot)
	snapshot := remoteState.snapshot
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return nil, err
	}

	attempt := executeSheinSubmitRemoteAttempt(
		ctx,
		in.TaskID,
		pkg,
		in.Action,
		in.RequestID,
		productAPI,
		in.Product,
		s.executeSheinSubmitRemote,
		s.retrySheinSensitiveWordSubmit,
	)
	if attempt.snapshot != nil {
		snapshot = attempt.snapshot
	}
	if attempt.err != nil {
		return nil, newSubmitRemoteActivityError(attempt.err, remoteState.supplierCode, attempt.response, snapshot)
	}

	return &SheinRemoteSubmitResult{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		SupplierCode: remoteState.supplierCode,
		Response:     attempt.response,
		Snapshot:     snapshot,
	}, nil
}
