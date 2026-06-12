package listingkit

import (
	"context"
	"fmt"

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

	submitProduct, err := s.prepareSheinSubmitProduct(ctx, task, pkg, in.Action)
	if err != nil {
		return nil, err
	}
	dumpSheinSubmitPayloadForDebug(in.TaskID, in.Action, in.RequestID, "prepared", submitProduct)
	snapshot := sheinpub.BuildSubmitSnapshot(submitProduct)
	if err := s.persistSheinSubmitSnapshot(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, snapshot); err != nil {
		return nil, err
	}

	return &SheinPreparedSubmitPayload{
		TaskID:           in.TaskID,
		Action:           in.Action,
		RequestID:        in.RequestID,
		Product:          submitProduct,
		NeedsImageUpload: sheinProductPendingImageUploadCount(submitProduct) > 0,
		Snapshot:         snapshot,
	}, nil
}

func (s *taskTemporalSubmissionAdapter) UploadSheinPublishImages(ctx context.Context, in *SheinPreparedSubmitPayload) (*SheinPreparedSubmitPayload, error) {
	if in == nil || in.Product == nil {
		return nil, fmt.Errorf("shein publish payload is required")
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
	prepareSheinProductForSubmit(in.Product, s.resolveSubmitSettings(ctx, task))
	dumpSheinSubmitPayloadForDebug(in.TaskID, in.Action, in.RequestID, "uploaded", in.Product)

	snapshot := sheinpub.BuildSubmitSnapshot(in.Product)
	if err := s.persistSheinSubmitSnapshot(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, snapshot); err != nil {
		return nil, err
	}

	out := *in
	out.NeedsImageUpload = false
	out.Snapshot = snapshot
	return &out, nil
}

func (s *taskTemporalSubmissionAdapter) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	if in == nil || in.Product == nil {
		return fmt.Errorf("shein publish payload is required")
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
	if in == nil || in.Product == nil {
		return nil, fmt.Errorf("shein publish payload is required")
	}
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}

	supplierCode := sheinSubmitSupplierCode(in.Product, pkg)
	snapshot := in.Snapshot
	if snapshot == nil {
		snapshot = sheinpub.BuildSubmitSnapshot(in.Product)
	}
	setSheinSubmitSupplierCode(pkg, in.Action, in.RequestID, supplierCode)
	setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, snapshot)
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
		return nil, newSubmitRemoteActivityError(attempt.err, supplierCode, attempt.response, snapshot)
	}

	return &SheinRemoteSubmitResult{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		SupplierCode: supplierCode,
		Response:     attempt.response,
		Snapshot:     snapshot,
	}, nil
}
