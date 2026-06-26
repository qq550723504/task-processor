package listingkit

import (
	submissiondomain "task-processor/internal/listing/submission"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinRemoteRefreshExecutionState = submissiondomain.RemoteRefreshExecutionState[sheinRemoteCompletionState]

func buildSheinRemoteRefreshRequest(productAPI sheinproduct.ProductAPI, state *sheinRemoteRefreshExecutionState) *sheinRemoteRefreshRequest {
	if state == nil {
		return nil
	}
	return &sheinRemoteRefreshRequest{
		task:         state.Completion.Task,
		taskID:       state.Completion.TaskID,
		pkg:          state.Completion.Package,
		productAPI:   productAPI,
		action:       state.Completion.Action,
		requestID:    state.Completion.RequestID,
		supplierCode: state.SupplierCode,
		startedAt:    state.StartedAt,
	}
}
