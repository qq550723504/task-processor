package aicapability

import (
	"context"
	"testing"
	"time"
)

type recordingUsageSettler struct {
	tenant, member, invocation string
	total                      int64
	at                         time.Time
}

func (r *recordingUsageSettler) SettleAIInvocationUsage(_ context.Context, tenant, member, invocation string, total int64, at time.Time) error {
	r.tenant, r.member, r.invocation, r.total, r.at = tenant, member, invocation, total, at
	return nil
}

func TestSettleSuccessfulInvocationUsesObservedTokensOnly(t *testing.T) {
	settler := &recordingUsageSettler{}
	at := time.Date(2026, 9, 20, 1, 2, 3, 0, time.UTC)
	record := InvocationRecord{InvocationID: "inv-1", TenantID: "org-1", UserID: "member-1", PromptTokens: 7, CompletionTokens: 5, TotalTokens: 12, UsageKnown: true, Outcome: InvocationSucceeded, FinishedAt: at}
	if err := SettleSuccessfulInvocation(context.Background(), record, settler); err != nil {
		t.Fatal(err)
	}
	if settler.tenant != "org-1" || settler.member != "member-1" || settler.invocation != "inv-1" || settler.total != 12 || !settler.at.Equal(at) {
		t.Fatalf("settlement=%+v", settler)
	}
}

func TestSettleSuccessfulInvocationIgnoresFailedOrUnknownUsage(t *testing.T) {
	settler := &recordingUsageSettler{}
	record := InvocationRecord{InvocationID: "inv-1", TenantID: "org-1", UserID: "member-1", TotalTokens: 12, UsageKnown: true, Outcome: InvocationFailed, FinishedAt: time.Now()}
	if err := SettleSuccessfulInvocation(context.Background(), record, settler); err != nil {
		t.Fatal(err)
	}
	if settler.invocation != "" {
		t.Fatal("failed invocation was settled")
	}
}
