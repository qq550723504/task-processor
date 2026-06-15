package submission

import "time"

type SubmitAttemptPlan struct {
	Platform    string
	Action      string
	RequestID   string
	StartedAt   time.Time
	UseWorkflow bool
}

func BuildSubmitAttemptPlan(
	taskID, platform, action, idempotencyKey, requestID string,
	startedAt time.Time,
	shouldStartWorkflow func(string, string) bool,
) SubmitAttemptPlan {
	resolvedRequestID := ResolveSubmitRequestID(idempotencyKey, requestID)
	useWorkflow := shouldStartWorkflow != nil && shouldStartWorkflow(platform, action)
	if useWorkflow && resolvedRequestID == "" {
		resolvedRequestID = DeriveWorkflowRequestID(taskID, action, startedAt)
	}
	return SubmitAttemptPlan{
		Platform:    platform,
		Action:      action,
		RequestID:   resolvedRequestID,
		StartedAt:   startedAt,
		UseWorkflow: useWorkflow,
	}
}
