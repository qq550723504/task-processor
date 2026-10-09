package submission

import (
	"context"
	"task-processor/internal/authidentity"
)

// ReadIntent supports checking the original durable operation after an
// uncertain Acquire. Reading never mints or reconstructs a SendPermit.
type ExecutionIntentReader interface {
	ReadIntent(context.Context, ExecutionScope, string) (ExecutionAttempt, error)
}

func (k *ExecutionKernel) ReadIntent(ctx context.Context, scope ExecutionScope, key string) (ExecutionAttempt, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	defer cancel()
	if k == nil || !validExecutionScope(scope) || !authidentity.IsBoundedIdentifier(key) {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	reader, ok := k.repository.(ExecutionIntentReader)
	if !ok {
		return ExecutionAttempt{}, ErrExecutionUnavailable
	}
	attempt, err := reader.ReadIntent(ctx, scope, key)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	if ValidatePersistedExecutionAttempt(attempt) != nil || attempt.OrganizationID != scope.OrganizationID || attempt.IntentKey != key {
		return ExecutionAttempt{}, ErrExecutionUnavailable
	}
	return attempt, nil
}
