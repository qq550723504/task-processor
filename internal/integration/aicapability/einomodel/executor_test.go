package einomodel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
)

type recordingLedger struct {
	claimed     bool
	claimErr    error
	replayClaim bool
	reserved    bool
	records     []aicapability.InvocationRecord
}

type parallelAdmissionLedger struct {
	mu      sync.Mutex
	claimed map[string]bool
}

func (l *parallelAdmissionLedger) ClaimInvocation(_ context.Context, record aicapability.InvocationRecord) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.claimed[record.InvocationID] {
		return false, nil
	}
	l.claimed[record.InvocationID] = true
	return true, nil
}
func (*parallelAdmissionLedger) ReserveAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error {
	return nil
}
func (*parallelAdmissionLedger) ReleaseAIInvocationUsage(context.Context, string, string) error {
	return nil
}
func (*parallelAdmissionLedger) RecordInvocation(context.Context, aicapability.InvocationRecord) error {
	return nil
}

func TestExecutorBoundsOverlappingSendsForOneCredential(t *testing.T) {
	var sends atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sends.Add(1)
		<-release
		_, _ = fmt.Fprint(w, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"synthetic-model","choices":[{"index":0,"message":{"role":"assistant","content":"READY"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	}))
	defer server.Close()
	profile := testTextProfile(server.URL)
	ledger := &parallelAdmissionLedger{claimed: make(map[string]bool)}
	executor := Executor{Ledger: ledger, Admission: NewBoundedAdmission(),
		Resolve: func(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error) {
			return QualifiedRoute{Profile: profile, Endpoint: server.URL, APIKey: "synthetic-only"}, nil
		},
		Authorize: func(context.Context, aicapability.TextInputIdentity) error { return nil }}
	var calls sync.WaitGroup
	for i := range 12 {
		calls.Add(1)
		go func(index int) {
			defer calls.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			input := aicapability.TextInputIdentity{OrganizationID: "org-1", ActorID: "user-1", MemberID: "member-1",
				Operation: aicapability.OperationAIWorkbenchChatPlan, InvocationID: fmt.Sprintf("inv-%d", index),
				System: "system", Prompt: "prompt", Profile: profile}
			quote, err := aicapability.QuoteText(input)
			if err == nil {
				_, _ = executor.Generate(ctx, input, quote, nil)
			}
		}(i)
	}
	deadline := time.Now().Add(5 * time.Second)
	for sends.Load() < 10 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	// Give the remaining contenders time to reach the same route's admission.
	time.Sleep(100 * time.Millisecond)
	observed := sends.Load()
	close(release)
	calls.Wait()
	require.EqualValues(t, 10, observed, "a credential must not have more than ten simultaneous model sends")
}

func TestExecutorAdmissionDeadlineLeavesDurableNoSendFact(t *testing.T) {
	var sends atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { sends.Add(1) }))
	defer server.Close()
	profile := testTextProfile(server.URL)
	input := aicapability.TextInputIdentity{OrganizationID: "org-1", ActorID: "user-1", MemberID: "member-1",
		Operation: aicapability.OperationAIWorkbenchChatPlan, InvocationID: "admission-expired",
		System: "system", Prompt: "prompt", Profile: profile}
	quote, err := aicapability.QuoteText(input)
	require.NoError(t, err)
	admission := NewBoundedAdmission()
	for range 15 {
		release, acquireErr := admission.Acquire(context.Background(), input)
		require.NoError(t, acquireErr)
		release()
	}
	ledger := &recordingLedger{}
	executor := Executor{Ledger: ledger, Admission: admission,
		Resolve: func(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error) {
			return QualifiedRoute{Profile: profile, Endpoint: server.URL, APIKey: "synthetic-only"}, nil
		},
		Authorize: func(context.Context, aicapability.TextInputIdentity) error { return nil }}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err = executor.Generate(ctx, input, quote, nil)
	require.ErrorIs(t, err, ErrNotDispatched)
	require.Zero(t, sends.Load())
	require.Len(t, ledger.records, 2)
	require.Equal(t, aicapability.InvocationFailed, ledger.records[1].Outcome)
	require.True(t, ledger.records[1].UsageKnown)
	require.Zero(t, ledger.records[1].TotalTokens)
}

func TestExecutorFinalGateRechecksDeadlineAfterSlowAuthorization(t *testing.T) {
	var sends atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { sends.Add(1) }))
	defer server.Close()
	profile := testTextProfile(server.URL)
	input := aicapability.TextInputIdentity{OrganizationID: "org-1", ActorID: "user-1", MemberID: "member-1",
		Operation: aicapability.OperationAIWorkbenchChatPlan, InvocationID: "slow-final-gate",
		System: "system", Prompt: "prompt", Profile: profile}
	quote, err := aicapability.QuoteText(input)
	require.NoError(t, err)
	ledger := &recordingLedger{}
	var authorizations atomic.Int32
	executor := Executor{Ledger: ledger, Admission: NewBoundedAdmission(),
		Resolve: func(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error) {
			return QualifiedRoute{Profile: profile, Endpoint: server.URL, APIKey: "synthetic-only"}, nil
		},
		Authorize: func(ctx context.Context, _ aicapability.TextInputIdentity) error {
			if authorizations.Add(1) == 2 {
				<-ctx.Done() // A stale dependency may still return nil after this wait.
			}
			return nil
		}}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err = executor.Generate(ctx, input, quote, nil)
	require.ErrorIs(t, err, ErrNotDispatched)
	require.Zero(t, sends.Load())
	require.EqualValues(t, 2, authorizations.Load())
	require.Len(t, ledger.records, 2)
	require.Equal(t, aicapability.InvocationFailed, ledger.records[1].Outcome)
}

func (l *recordingLedger) ClaimInvocation(_ context.Context, r aicapability.InvocationRecord) (bool, error) {
	l.records = append(l.records, r)
	if l.claimErr != nil {
		l.claimed = true // The durable insert may have committed before its response was lost.
		return false, l.claimErr
	}
	if l.replayClaim {
		return false, nil
	}
	if l.claimed {
		return false, nil
	}
	l.claimed = true
	return true, nil
}

func TestExecutorPreservesUnknownClaimInsteadOfInventingNoDispatch(t *testing.T) {
	for _, tc := range []struct {
		name     string
		claimErr error
		replay   bool
	}{
		{name: "lost claim response", claimErr: errors.New("claim response lost")},
		{name: "existing claim", replay: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sends := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { sends++ }))
			defer server.Close()
			profile := testTextProfile(server.URL)
			input := aicapability.TextInputIdentity{OrganizationID: "org-1", ActorID: "user-1", MemberID: "member-1", Operation: aicapability.OperationAIWorkbenchChatPlan, InvocationID: "inv-claim",
				System: "system", Prompt: "prompt", Profile: profile}
			quote, err := aicapability.QuoteText(input)
			require.NoError(t, err)
			ledger := &recordingLedger{claimErr: tc.claimErr, replayClaim: tc.replay}
			executor := Executor{Ledger: ledger, Admission: NewBoundedAdmission(),
				Resolve: func(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error) {
					return QualifiedRoute{Profile: profile, Endpoint: server.URL, APIKey: "synthetic-only"}, nil
				},
				Authorize: func(context.Context, aicapability.TextInputIdentity) error { return nil },
			}
			_, err = executor.Generate(context.Background(), input, quote, nil)
			require.ErrorIs(t, err, ErrOutcomeUnknown)
			require.NotErrorIs(t, err, ErrNotDispatched)
			require.Zero(t, sends)
			require.False(t, ledger.reserved)
			require.Len(t, ledger.records, 1, "no terminal no-dispatch fact may be invented")
		})
	}
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
		Ledger: ledger, Admission: NewBoundedAdmission(),
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

	invalidInput := input
	invalidInput.InvocationID = "inv-invalid"
	ledger.claimed = false // The fake tracks one claim at a time; this is a new invocation ID.
	invalidQuote, err := aicapability.QuoteText(invalidInput)
	require.NoError(t, err)
	invalid, err := executor.Generate(context.Background(), invalidInput, invalidQuote, func(string) error { return ErrInvalid })
	require.ErrorIs(t, err, ErrInvalid)
	require.True(t, invalid.Usage.Known, "terminal invalid output must retain observed usage")
	require.Equal(t, 15, invalid.Usage.TotalTokens)
	require.Equal(t, 2, sends)
	require.Equal(t, aicapability.InvocationUsageObservedFailed, ledger.records[len(ledger.records)-1].Outcome)
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
	executor := Executor{Ledger: ledger, Admission: NewBoundedAdmission(),
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
