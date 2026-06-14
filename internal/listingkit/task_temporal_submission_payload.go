package listingkit

import (
	"context"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *taskTemporalSubmissionAdapter) PrepareSheinPublishPayload(ctx context.Context, in SheinPublishAttemptInput) (*SheinPreparedSubmitPayload, error) {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	s.normalizeSheinSubmitPackage(task, pkg, sheinSubmitRequestFromActivity(in), in.Action)
	preparedPayload, err := s.payloadStages.Prepare(ctx, newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID))
	if err != nil {
		return nil, err
	}
	return adaptSubmissionPreparedPayload(preparedPayload, newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID)), nil
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
	out, err := s.payloadStages.UploadImages(
		ctx,
		newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID),
		adaptListingKitPreparedPayload(in),
	)
	if err != nil {
		return nil, err
	}
	return adaptSubmissionPreparedPayload(out, newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID)), nil
}

func (s *taskTemporalSubmissionAdapter) PreValidateSheinPublish(ctx context.Context, in *SheinPreparedSubmitPayload) error {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	return s.payloadStages.PreValidate(
		ctx,
		newSubmissionPayloadStageContext(in.TaskID, task, pkg, in.Action, in.RequestID),
		adaptListingKitPreparedPayload(in),
	)
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
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseSubmitRemote); err != nil {
		return nil, err
	}

	result := s.remoteSubmitter.Submit(ctx, submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]{
		TaskID:     in.TaskID,
		Package:    pkg,
		Action:     in.Action,
		RequestID:  in.RequestID,
		ProductAPI: productAPI,
		Product:    in.Product,
		Snapshot:   in.Snapshot,
	})
	if result.Err != nil {
		return nil, newSubmitRemoteActivityError(result.Err, result.SupplierCode, result.Response, result.Snapshot)
	}

	return &SheinRemoteSubmitResult{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		SupplierCode: result.SupplierCode,
		Response:     result.Response,
		Snapshot:     result.Snapshot,
	}, nil
}
