package listingkit

import (
	"context"
	"fmt"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinTemporalPublishExecutionState struct {
	taskID    string
	task      *Task
	pkg       *SheinPackage
	action    string
	requestID string
}

type sheinTemporalPreparedPublishState struct {
	execution                  *sheinTemporalPublishExecutionState
	request                    *SubmitTaskRequest
	finalDraftConfirmedChanged bool
}

type sheinTemporalPreparedPayloadState struct {
	execution    *sheinTemporalPublishExecutionState
	stageContext submissiondomain.PayloadStageContext[*Task, *SheinPackage]
	payload      *submissiondomain.PreparedPayload[*sheinproduct.Product, *sheinpub.SubmitSnapshot]
}

func loadSheinTemporalPublishExecutionState(
	ctx context.Context,
	taskID string,
	action string,
	requestID string,
	load func(context.Context, string) (*Task, *SheinPackage, error),
) (*sheinTemporalPublishExecutionState, error) {
	if load == nil {
		return nil, fmt.Errorf("shein publish task loader is not configured")
	}
	task, pkg, err := load(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return &sheinTemporalPublishExecutionState{
		taskID:    taskID,
		task:      task,
		pkg:       pkg,
		action:    action,
		requestID: requestID,
	}, nil
}

func loadSheinTemporalPreparedPublishState(
	ctx context.Context,
	in SheinPublishAttemptInput,
	load func(context.Context, string) (*Task, *SheinPackage, error),
	normalize func(*Task, *SheinPackage, *SubmitTaskRequest, string),
) (*sheinTemporalPreparedPublishState, error) {
	state, err := loadSheinTemporalPublishExecutionState(ctx, in.TaskID, in.Action, in.RequestID, load)
	if err != nil {
		return nil, err
	}
	req := sheinSubmitRequestFromActivity(in)
	finalWasConfirmed := state.pkg != nil && state.pkg.FinalSubmissionDraft != nil && state.pkg.FinalSubmissionDraft.Confirmed
	if normalize != nil {
		normalize(state.task, state.pkg, req, state.action)
	}
	finalNowConfirmed := state.pkg != nil && state.pkg.FinalSubmissionDraft != nil && state.pkg.FinalSubmissionDraft.Confirmed
	return &sheinTemporalPreparedPublishState{
		execution:                  state,
		request:                    req,
		finalDraftConfirmedChanged: finalNowConfirmed != finalWasConfirmed,
	}, nil
}

func loadSheinTemporalPreparedPayloadState(
	ctx context.Context,
	in *SheinPreparedSubmitPayload,
	load func(context.Context, string) (*Task, *SheinPackage, error),
) (*sheinTemporalPreparedPayloadState, error) {
	if err := requireSheinPreparedSubmitPayload(in); err != nil {
		return nil, err
	}
	execution, err := loadSheinTemporalPublishExecutionState(ctx, in.TaskID, in.Action, in.RequestID, load)
	if err != nil {
		return nil, err
	}
	stageContext := buildSheinTemporalSubmissionPayloadStageContext(execution)
	return &sheinTemporalPreparedPayloadState{
		execution:    execution,
		stageContext: stageContext,
		payload:      adaptListingKitPreparedPayload(in),
	}, nil
}

func (s *taskTemporalSubmissionFlowService) loadSheinPreparedPayloadState(ctx context.Context, in *SheinPreparedSubmitPayload) (*sheinTemporalPreparedPayloadState, error) {
	return loadSheinTemporalPreparedPayloadState(ctx, in, s.loadSheinPublishTask)
}

func buildSheinTemporalSubmissionPayloadStageContext(state *sheinTemporalPublishExecutionState) submissiondomain.PayloadStageContext[*Task, *SheinPackage] {
	if state == nil {
		return submissiondomain.PayloadStageContext[*Task, *SheinPackage]{}
	}
	return newSubmissionPayloadStageContext(state.taskID, state.task, state.pkg, state.action, state.requestID)
}

func buildSheinTemporalRemoteSubmitInput(
	state *sheinTemporalPublishExecutionState,
	productAPI sheinproduct.ProductAPI,
	product *sheinproduct.Product,
	snapshot *sheinpub.SubmitSnapshot,
) submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot] {
	if state == nil {
		return submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]{}
	}
	return submissiondomain.RemoteSubmitInput[*SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmitSnapshot]{
		TaskID:     state.taskID,
		Package:    state.pkg,
		Action:     state.action,
		RequestID:  state.requestID,
		ProductAPI: productAPI,
		Product:    product,
		Snapshot:   snapshot,
	}
}
