package aicapability

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidInvocationUsage = errors.New("invalid successful invocation usage")

// InvocationUsageSettler is the narrow adapter boundary from observed AI
// facts to commercial usage accounting. It deliberately accepts no task or
// generation counters.
type InvocationUsageSettler interface {
	SettleAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error
}

// SettleSuccessfulInvocation forwards only a successful invocation with
// provider-observed, internally consistent token usage. The adapter is
// idempotent at the commercial owner using invocation identity and window.
func SettleSuccessfulInvocation(ctx context.Context, record InvocationRecord, settler InvocationUsageSettler) error {
	if record.Outcome != InvocationSucceeded || !record.UsageKnown {
		return nil
	}
	if record.TenantID == "" || record.UserID == "" || record.MemberID == "" || record.InvocationID == "" || record.TotalTokens <= 0 || record.PromptTokens < 0 || record.CompletionTokens < 0 || record.PromptTokens+record.CompletionTokens != record.TotalTokens || record.FinishedAt.IsZero() {
		return ErrInvalidInvocationUsage
	}
	if settler == nil {
		return ErrInvalidInvocationUsage
	}
	return settler.SettleAIInvocationUsage(ctx, record.TenantID, record.MemberID, record.InvocationID, int64(record.TotalTokens), record.FinishedAt.UTC())
}
