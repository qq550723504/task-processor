package einomodel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
)

type recordingLedger struct {
	claimed  bool
	reserved bool
	records  []aicapability.InvocationRecord
}

func (l *recordingLedger) ClaimInvocation(_ context.Context, r aicapability.InvocationRecord) (bool, error) {
	l.records = append(l.records, r)
	if l.claimed {
		return false, nil
	}
	l.claimed = true
	return true, nil
}
func (l *recordingLedger) ReserveAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error {
	l.reserved = true
	return nil
}
func (l *recordingLedger) ReleaseAIInvocationUsage(context.Context, string, string) error { return nil }
func (l *recordingLedger) RecordInvocation(_ context.Context, r aicapability.InvocationRecord) error {
	l.records = append(l.records, r)
	return nil
}

func testTextProfile(endpoint string) aicapability.ModelProfile {
	return aicapability.ModelProfile{
		ClientName: "org-planning", ProviderID: "synthetic", AdapterKind: string(AdapterOpenAICompatible),
		ModelID: "synthetic-model", EndpointIdentityDigest: endpointDigest(endpoint), CredentialVersion: "v1",
		RoutingPolicyVersion: "routing-v1", AdapterPolicyVersion: "adapter-v1", PromptVersion: "prompt-v1",
		OutputSchemaVersion: "schema-v1", UsageMappingVersion: "usage-v1", CostPricingVersion: "price-v1",
		PointTariff: aicapability.ModelPointTariff{PriceVersion: "points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 1000000},
		Currency:    "USD", InputMicrosPerMillion: 1000000, OutputMicrosPerMillion: 1000000,
		MaximumPromptTokens: 8192, MaximumCompletionTokens: 64, MaximumInputBytes: 16 << 10,
		MaximumOutputBytes: 16 << 10, DeadlineBound: 5 * time.Second,
	}
}

func TestExecutorSettlesOneObservedPlannerInvocation(t *testing.T) {
	sends := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sends++
		_, _ = fmt.Fprint(w, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"synthetic-model","choices":[{"index":0,"message":{"role":"assistant","content":"READY"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	}))
	defer server.Close()
	profile := testTextProfile(server.URL)
	input := aicapability.TextInputIdentity{OrganizationID: "org-1", ActorID: "user-1", MemberID: "member-1", Operation: aicapability.OperationAIWorkbenchChatPlan, InvocationID: "inv-1", System: "system", Prompt: "prompt", Profile: profile}
	quote, err := aicapability.QuoteText(input)
	require.NoError(t, err)
	ledger := &recordingLedger{}
	authCalls := 0
	executor := Executor{
		Ledger: ledger,
		Resolve: func(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error) {
			return QualifiedRoute{Profile: profile, Endpoint: server.URL, APIKey: "synthetic-only"}, nil
		},
		Authorize: func(context.Context, aicapability.TextInputIdentity) error { authCalls++; return nil },
	}
	result, err := executor.Generate(context.Background(), input, quote, func(s string) error {
		if s != "READY" {
			return ErrInvalid
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, "READY", result.Content)
	require.Equal(t, 1, sends)
	require.GreaterOrEqual(t, authCalls, 2)
	require.True(t, ledger.reserved)
	require.Len(t, ledger.records, 2)
	require.Equal(t, aicapability.InvocationDispatched, ledger.records[0].Outcome)
	require.Equal(t, aicapability.InvocationSucceeded, ledger.records[1].Outcome)
	require.Equal(t, 15, ledger.records[1].TotalTokens)
	require.Equal(t, quote.InputHash, ledger.records[1].InputHash)

	_, err = executor.Generate(context.Background(), input, quote, nil)
	require.Error(t, err)
	require.Equal(t, 1, sends)
}

func TestExecutorFinalGateDenialHasNoNetworkSend(t *testing.T) {
	sends := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { sends++ }))
	defer server.Close()
	profile := testTextProfile(server.URL)
	input := aicapability.TextInputIdentity{OrganizationID: "org-1", ActorID: "user-1", MemberID: "member-1", Operation: aicapability.OperationAIWorkbenchChatPlan, InvocationID: "inv-2", System: "system", Prompt: "prompt", Profile: profile}
	quote, err := aicapability.QuoteText(input)
	require.NoError(t, err)
	ledger := &recordingLedger{}
	checks := 0
	executor := Executor{Ledger: ledger,
		Resolve: func(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error) {
			return QualifiedRoute{Profile: profile, Endpoint: server.URL, APIKey: "synthetic-only"}, nil
		},
		Authorize: func(context.Context, aicapability.TextInputIdentity) error {
			checks++
			if checks > 1 {
				return errors.New("permission revoked")
			}
			return nil
		},
	}
	_, err = executor.Generate(context.Background(), input, quote, nil)
	require.ErrorIs(t, err, ErrNotDispatched)
	require.Zero(t, sends)
	require.Len(t, ledger.records, 2)
	require.Equal(t, aicapability.InvocationFailed, ledger.records[1].Outcome)
}
