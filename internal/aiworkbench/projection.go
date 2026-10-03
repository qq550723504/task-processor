package aiworkbench

import (
	"encoding/json"
	"time"

	"task-processor/internal/agent"
)

type TaskState string

const (
	TaskRunning             TaskState = "RUNNING"
	TaskWaitingConfirmation TaskState = "WAITING_CONFIRMATION"
	TaskCompleted           TaskState = "COMPLETED"
	TaskError               TaskState = "ERROR"
	TaskPaused              TaskState = "PAUSED"
)

type TaskProjection struct {
	State     TaskState
	Reason    string
	CanStart  bool
	CanResume bool
	CanReview bool
}

// ProjectTask derives status from the immutable Task and exact current Agent
// and Product Review owner facts. It never mutates a RUNNING row or invents a
// success state when an owner is unavailable.
func ProjectTask(task BusinessTask, run *agent.Record, reviewState string, now time.Time) (TaskProjection, error) {
	if task.Scope.OrganizationID == "" || task.Scope.ActorID == "" || task.ExecutionRequestKey == "" ||
		task.OperationID == "" || task.ProductKey == "" || task.TargetPlatform == "" || now.IsZero() {
		return TaskProjection{}, ErrInvalid
	}
	if run == nil {
		if reviewState != "" {
			return TaskProjection{}, ErrUnavailable
		}
		return TaskProjection{State: TaskError, Reason: "START_NOT_CLAIMED", CanStart: true}, nil
	}
	s := run.State
	if s.Scope.OrganizationID != task.Scope.OrganizationID || s.Scope.ActorID != task.Scope.ActorID ||
		s.Request.Key != task.ExecutionRequestKey || s.Request.Binding.ContextID != task.OperationID ||
		s.Request.Binding.ProductKey != task.ProductKey || s.Request.Binding.TargetPlatform != task.TargetPlatform {
		return TaskProjection{}, ErrUnavailable
	}
	if len(task.ExecutionRequest) != 0 {
		var exact agent.Request
		if json.Unmarshal(task.ExecutionRequest, &exact) != nil || exact != s.Request {
			return TaskProjection{}, ErrUnavailable
		}
	}
	switch reviewState {
	case "applied", "rejected":
		return TaskProjection{State: TaskCompleted, Reason: reviewState}, nil
	case "pending", "accepted":
		return TaskProjection{State: TaskWaitingConfirmation, Reason: reviewState, CanReview: reviewState == "pending"}, nil
	case "":
	default:
		return TaskProjection{}, ErrUnavailable
	}
	switch s.Phase {
	case agent.HumanReviewRequired:
		return TaskProjection{State: TaskWaitingConfirmation, Reason: "HUMAN_REVIEW_REQUIRED", CanReview: true}, nil
	case agent.Interrupted:
		return TaskProjection{State: TaskPaused, Reason: "AGENT_INTERRUPTED", CanResume: true}, nil
	case agent.Running:
		if now.After(s.Deadline.Add(30 * time.Second)) {
			return TaskProjection{State: TaskError, Reason: "EXECUTION_OUTCOME_UNKNOWN", CanStart: true}, nil
		}
		return TaskProjection{State: TaskRunning}, nil
	case agent.Stopped:
		reason := string(s.StopReason)
		if s.StopReason == agent.StopExecutionOutcomeUnknown || s.StopReason == agent.StopUsageUnknown || s.StopReason == agent.StopModelUnknown {
			reason = "EXECUTION_OUTCOME_UNKNOWN"
		}
		if reason == "" {
			return TaskProjection{}, ErrUnavailable
		}
		return TaskProjection{State: TaskError, Reason: reason}, nil
	default:
		return TaskProjection{}, ErrUnavailable
	}
}
