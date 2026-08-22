package productenrich

import (
	"context"
	"strings"

	"task-processor/internal/shared/aiidentity"
)

// WithTaskIdentity restores the persisted request identity for asynchronous
// ProductEnrich processing while preserving any trace context carried by the
// worker runtime.
func WithTaskIdentity(ctx context.Context, task *Task) context.Context {
	if task == nil {
		return ctx
	}
	if envelope, err := task.ExecutionEnvelope(); err == nil && envelope.Version != 0 {
		if restored, restoreErr := aiidentity.RestoreExecutionEnvelope(ctx, envelope, task.ID); restoreErr == nil {
			return restored
		}
	}
	identity := aiidentity.FromContext(ctx)
	identity.TenantID = strings.TrimSpace(task.TenantID)
	identity.UserID = strings.TrimSpace(task.UserID)
	identity.BusinessTaskID = strings.TrimSpace(task.ID)
	if identity.TenantID == "" && identity.UserID == "" && identity.BusinessTaskID == "" {
		return ctx
	}
	return aiidentity.WithIdentity(ctx, identity)
}
