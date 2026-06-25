package listingkit

import (
	"context"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func newSheinTemporalSubmitPayloadStages(s *taskTemporalSubmissionFlowService) *submissiondomain.PayloadStageService[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot] {
	if s == nil {
		return nil
	}
	return submissiondomain.NewPayloadStageService(submissiondomain.PayloadStageServiceConfig[*Task, *SheinPackage, *sheinproduct.Product, *sheinpub.SubmitSnapshot]{
		Phases: submissiondomain.PayloadStagePhases{
			PrepareProduct: sheinpub.SubmissionPhasePrepareProduct,
			UploadImages:   sheinpub.SubmissionPhaseUploadImages,
			PreValidate:    sheinpub.SubmissionPhasePreValidate,
		},
		PersistPhase: func(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], phase string) error {
			return s.persistSheinSubmitPhase(ctx, in.TaskID, in.Task.Result, in.Package, in.Action, in.RequestID, phase)
		},
		PreparePayload: s.prepareTemporalSubmitPayload,
		PersistSnapshot: func(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {
			if s.persistence == nil {
				return nil
			}
			return s.persistence.persistSheinSubmitSnapshot(ctx, in.TaskID, in.Task.Result, in.Package, in.Action, in.RequestID, snapshot)
		},
		RequirePreparedPayload: requireSubmissionPreparedPayload,
		UploadImages: func(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {
			return s.uploadSheinSubmitImages(ctx, in.Task, in.Package, product)
		},
		FinalizeUploaded: s.finalizeTemporalSubmitPayload,
		PreValidate: func(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], product *sheinproduct.Product) error {
			return sheinpub.PreValidateSubmitProductWithOptions(product, !sheinpub.SecondarySaleAttributeRequired(in.Package))
		},
	})
}

func (s *taskTemporalSubmissionFlowService) prepareTemporalSubmitPayload(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage]) (*submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot], error) {
	preparedPayload, err := prepareSheinSubmitPayloadProduct(ctx, in.TaskID, in.Action, in.RequestID, in.Task, in.Package, s.prepareSheinSubmitProduct)
	if err != nil {
		return nil, err
	}
	return adaptListingKitPreparedPayload(preparedPayload), nil
}

func (s *taskTemporalSubmissionFlowService) finalizeTemporalSubmitPayload(ctx context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], payload *submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot]) (*submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot], error) {
	out := finalizeSheinUploadedSubmitPayload(ctx, in.TaskID, in.Action, in.RequestID, in.Task, adaptSubmissionPreparedPayload(payload, in), s.resolveSubmitSettings)
	return adaptListingKitPreparedPayload(out), nil
}

func (s *taskTemporalSubmissionFlowService) executeTemporalRemoteSubmitAttempt(ctx context.Context, in submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]) submissiondomain.RemoteSubmitResult[*sheinpub.SubmissionResponse, *sheinpub.SubmitSnapshot] {
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
