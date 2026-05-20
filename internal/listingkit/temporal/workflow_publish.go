package temporal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

func PublishWorkflow(ctx sdkworkflow.Context, in SheinPublishWorkflowInput) error {
	in = normalizeSheinPublishWorkflowInput(ctx, in)

	ctx = sdkworkflow.WithActivityOptions(ctx, sdkworkflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2,
			MaximumInterval:        30 * time.Second,
			MaximumAttempts:        5,
			NonRetryableErrorTypes: []string{listingkit.SheinSubmitRemoteActivityErrorType},
		},
	})

	state := SheinPublishStateQueryResult{
		TaskID:          in.TaskID,
		Action:          in.Action,
		RequestID:       in.RequestID,
		StartedAt:       timePointer(in.RequestedAt),
		WorkflowRunning: true,
	}
	if err := sdkworkflow.SetQueryHandler(ctx, SheinPublishQueryCurrentState, func() (SheinPublishStateQueryResult, error) {
		return state, nil
	}); err != nil {
		return err
	}

	state.CurrentPhase = sheinpub.SubmissionPhaseValidate
	if err := sdkworkflow.ExecuteActivity(ctx, activityNameBeginPublishAttempt, in).Get(ctx, nil); err != nil {
		return persistWorkflowFailure(ctx, &state, err, nil, nil)
	}
	if err := sdkworkflow.ExecuteActivity(ctx, activityNameValidateReadiness, in).Get(ctx, nil); err != nil {
		return persistWorkflowFailure(ctx, &state, err, nil, nil)
	}

	state.CurrentPhase = sheinpub.SubmissionPhasePrepareProduct
	var prepared *listingkit.SheinPreparedSubmitPayload
	if err := sdkworkflow.ExecuteActivity(ctx, activityNamePrepareProduct, in).Get(ctx, &prepared); err != nil {
		return persistWorkflowFailure(ctx, &state, err, prepared, nil)
	}

	if prepared != nil && prepared.NeedsImageUpload {
		state.CurrentPhase = sheinpub.SubmissionPhaseUploadImages
		var uploaded *listingkit.SheinPreparedSubmitPayload
		if err := sdkworkflow.ExecuteActivity(ctx, activityNameUploadImages, prepared).Get(ctx, &uploaded); err != nil {
			return persistWorkflowFailure(ctx, &state, err, prepared, nil)
		}
		if uploaded != nil {
			prepared = uploaded
		}
	}

	state.CurrentPhase = sheinpub.SubmissionPhasePreValidate
	if err := sdkworkflow.ExecuteActivity(ctx, activityNamePreValidate, prepared).Get(ctx, nil); err != nil {
		return persistWorkflowFailure(ctx, &state, err, prepared, nil)
	}

	state.CurrentPhase = sheinpub.SubmissionPhaseSubmitRemote
	var remoteResult *listingkit.SheinRemoteSubmitResult
	if err := sdkworkflow.ExecuteActivity(ctx, activityNameSubmitRemote, prepared).Get(ctx, &remoteResult); err != nil {
		return persistWorkflowFailure(ctx, &state, err, prepared, remoteResult)
	}

	state.CurrentPhase = sheinpub.SubmissionPhasePersistResult
	if err := sdkworkflow.ExecuteActivity(ctx, activityNamePersistSuccess, listingkit.SheinPersistSubmitSuccessInput{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		SupplierCode: sheinRemoteSupplierCode(remoteResult),
		Response:     sheinRemoteResponse(remoteResult),
		Snapshot:     sheinRemoteSnapshot(remoteResult, prepared),
	}).Get(ctx, nil); err != nil {
		return persistWorkflowFailure(ctx, &state, err, prepared, remoteResult)
	}

	finishWorkflowSuccess(ctx, &state)
	return nil
}

func persistWorkflowFailure(
	ctx sdkworkflow.Context,
	state *SheinPublishStateQueryResult,
	activityErr error,
	prepared *listingkit.SheinPreparedSubmitPayload,
	remoteResult *listingkit.SheinRemoteSubmitResult,
) error {
	failure := sheinPublishFailureInput(*state, activityErr, prepared, remoteResult)
	if failure.ErrorMessage != "" {
		state.LastError = failure.ErrorMessage
	}
	if persistErr := sdkworkflow.ExecuteActivity(ctx, activityNamePersistFailure, &failure).Get(ctx, nil); persistErr != nil {
		finishWorkflowState(ctx, state, failure.ErrorMessage)
		return fmt.Errorf("persist workflow failure after %w: %v", activityErr, persistErr)
	}
	finishWorkflowState(ctx, state, failure.ErrorMessage)
	return activityErr
}

func finishWorkflowError(ctx sdkworkflow.Context, state *SheinPublishStateQueryResult, err error) error {
	finishWorkflowState(ctx, state, errorMessage(err))
	return err
}

func finishWorkflowSuccess(ctx sdkworkflow.Context, state *SheinPublishStateQueryResult) {
	finishWorkflowState(ctx, state, "")
}

func finishWorkflowState(ctx sdkworkflow.Context, state *SheinPublishStateQueryResult, lastError string) {
	now := sdkworkflow.Now(ctx)
	state.FinishedAt = &now
	state.WorkflowRunning = false
	state.LastError = strings.TrimSpace(lastError)
}

func normalizeSheinPublishWorkflowInput(ctx sdkworkflow.Context, in SheinPublishWorkflowInput) SheinPublishWorkflowInput {
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.Platform = strings.TrimSpace(in.Platform)
	in.Action = strings.TrimSpace(in.Action)
	in.RequestID = strings.TrimSpace(in.RequestID)
	if in.Action == "" {
		in.Action = "publish"
	}
	if in.RequestedAt.IsZero() {
		in.RequestedAt = sdkworkflow.Now(ctx)
	}
	return in
}

func sheinPublishFailureInput(
	state SheinPublishStateQueryResult,
	activityErr error,
	prepared *listingkit.SheinPreparedSubmitPayload,
	remoteResult *listingkit.SheinRemoteSubmitResult,
) listingkit.SheinPersistSubmitFailureInput {
	failure := listingkit.SheinPersistSubmitFailureInput{
		TaskID:       state.TaskID,
		Action:       state.Action,
		RequestID:    state.RequestID,
		Phase:        state.CurrentPhase,
		ErrorMessage: strings.TrimSpace(errorMessage(activityErr)),
		SupplierCode: sheinRemoteSupplierCode(remoteResult),
		Response:     sheinRemoteResponse(remoteResult),
		Snapshot:     sheinRemoteSnapshot(remoteResult, prepared),
	}

	if details, ok := sheinSubmitRemoteActivityErrorDetails(activityErr); ok {
		if details.ErrorMessage != "" {
			failure.ErrorMessage = strings.TrimSpace(details.ErrorMessage)
		}
		if details.SupplierCode != "" {
			failure.SupplierCode = details.SupplierCode
		}
		if details.Response != nil {
			failure.Response = details.Response
		}
		if details.Snapshot != nil {
			failure.Snapshot = details.Snapshot
		}
	}
	if failure.ErrorMessage == "" {
		failure.ErrorMessage = "shein publish workflow activity failed"
	}
	return failure
}

func sheinSubmitRemoteActivityErrorDetails(err error) (*listingkit.SheinSubmitRemoteActivityErrorDetails, bool) {
	var appErr *sdktemporal.ApplicationError
	if !errors.As(err, &appErr) || appErr.Type() != listingkit.SheinSubmitRemoteActivityErrorType {
		return nil, false
	}
	var details listingkit.SheinSubmitRemoteActivityErrorDetails
	if detailsErr := appErr.Details(&details); detailsErr != nil {
		return nil, false
	}
	return &details, true
}

func sheinRemoteSupplierCode(remoteResult *listingkit.SheinRemoteSubmitResult) string {
	if remoteResult == nil {
		return ""
	}
	return strings.TrimSpace(remoteResult.SupplierCode)
}

func sheinRemoteResponse(remoteResult *listingkit.SheinRemoteSubmitResult) *sheinpub.SubmissionResponse {
	if remoteResult == nil {
		return nil
	}
	return remoteResult.Response
}

func sheinRemoteSnapshot(remoteResult *listingkit.SheinRemoteSubmitResult, prepared *listingkit.SheinPreparedSubmitPayload) *sheinpub.SubmitSnapshot {
	if remoteResult != nil && remoteResult.Snapshot != nil {
		return remoteResult.Snapshot
	}
	if prepared != nil {
		return prepared.Snapshot
	}
	return nil
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
