package submission

import (
	"fmt"
	"strings"
	"time"
)

func ResolveSubmitRequestID(idempotencyKey, requestID string) string {
	if value := strings.TrimSpace(idempotencyKey); value != "" {
		return value
	}
	return strings.TrimSpace(requestID)
}

func DeriveWorkflowRequestID(taskID, action string, requestedAt time.Time) string {
	taskID = strings.TrimSpace(taskID)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "publish"
	}
	if taskID == "" {
		taskID = "unknown-task"
	}
	timestamp := requestedAt.UTC().Format("20060102T150405.000000000Z")
	return fmt.Sprintf("temporal:%s:%s:%s", taskID, action, timestamp)
}
