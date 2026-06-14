package listingkit

import (
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinRemoteRefreshExecutionState struct {
	completion     sheinRemoteCompletionState
	supplierCode   string
	refreshStarted time.Time
}

func newSheinRemoteRefreshExecutionState(completion sheinRemoteCompletionState, supplierCode string, refreshStarted time.Time) *sheinRemoteRefreshExecutionState {
	return &sheinRemoteRefreshExecutionState{
		completion:     completion,
		supplierCode:   supplierCode,
		refreshStarted: refreshStarted,
	}
}

func buildSheinRemoteRefreshRequest(productAPI sheinproduct.ProductAPI, state *sheinRemoteRefreshExecutionState) *sheinRemoteRefreshRequest {
	if state == nil {
		return nil
	}
	return &sheinRemoteRefreshRequest{
		task:         state.completion.task,
		taskID:       state.completion.taskID,
		pkg:          state.completion.pkg,
		productAPI:   productAPI,
		action:       state.completion.action,
		requestID:    state.completion.requestID,
		supplierCode: state.supplierCode,
		startedAt:    state.refreshStarted,
	}
}
