package einomodel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
)

const googleTestOrigin = "https://generativelanguage.googleapis.com"
const googleTestUsage = `{"total_input_tokens":3,"total_output_tokens":4,"total_thought_tokens":7,"total_tokens":14,"total_tool_use_tokens":0}`

type googleTestTransport func(*http.Request) (*http.Response, error)

func (f googleTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func googleLoopbackTransport(t *testing.T, endpoint string) http.RoundTripper {
	t.Helper()
	target, err := url.Parse(endpoint)
	require.NoError(t, err)
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DisableKeepAlives = true
	t.Cleanup(base.CloseIdleConnections)
	return googleTestTransport(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "generativelanguage.googleapis.com", r.URL.Host)
		forward := r.Clone(r.Context())
		forward.URL.Scheme, forward.URL.Host, forward.Host = target.Scheme, target.Host, target.Host
		return base.RoundTrip(forward)
	})
}

func googleTestProfile() aicapability.ModelProfile {
	p := testTextProfile(googleTestOrigin)
	p.ProviderID, p.AdapterKind, p.ModelID = "google", "google-interactions", "gemini-3.8-flash"
	p.AdapterPolicyVersion, p.UsageMappingVersion = "google-interactions-v1", "google-interactions-usage-v1"
	return p
}

func googleTestResponse(status, steps, usage string) string {
	return `{"id":"interaction-1","model":"gemini-3.8-flash","status":"` + status + `","steps":` + steps + `,"usage":` + usage + `}`
}

func TestGoogleSharedExecutorPreservesThoughtUsageAndUnknown(t *testing.T) {
	text := `[{"type":"thought","content":[]},{"type":"model_output","content":[{"type":"text","text":"hello"}]}]`
	for _, tc := range []struct {
		name, status, steps, usage string
		known                      bool
		outcome                    aicapability.InvocationOutcome
	}{
		{"completed", "completed", text, googleTestUsage, true, aicapability.InvocationSucceeded},
		{"incomplete", "incomplete", text, googleTestUsage, true, aicapability.InvocationUsageObservedFailed},
		{"non text", "completed", `[{"type":"model_output","content":[{"type":"image","uri":"https://example.test/x.png","mime_type":"image/png"}]}]`, googleTestUsage, false, ""},
		{"tool step", "completed", `[{"type":"function_call","name":"unknown","arguments":{}},{"type":"model_output","content":[{"type":"text","text":"hello"}]}]`, googleTestUsage, false, ""},
		{"missing thoughts", "completed", text, `{"total_input_tokens":3,"total_output_tokens":4,"total_tokens":7,"total_tool_use_tokens":0}`, false, ""},
		{"unknown dimension", "completed", text, strings.TrimSuffix(googleTestUsage, "}") + `,"total_other_tokens":1}`, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sends atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends.Add(1)
				require.Equal(t, "/v1beta/interactions", r.URL.Path)
				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				require.Equal(t, false, payload["store"])
				require.Equal(t, false, payload["stream"])
				require.Equal(t, false, payload["background"])
				require.NotContains(t, payload, "tools")
				require.NotContains(t, payload, "previous_interaction_id")
				generation := payload["generation_config"].(map[string]any)
				require.Equal(t, "low", generation["thinking_level"])
				require.Equal(t, float64(64), generation["max_output_tokens"])
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, googleTestResponse(tc.status, tc.steps, tc.usage))
			}))
			defer server.Close()
			profile := googleTestProfile()
			input := aicapability.TextInputIdentity{OrganizationID: "org", ActorID: "actor", MemberID: "member", Operation: aicapability.OperationProductAgentDecision,
				AgentRunID: "run", InvocationID: "inv", System: "system", Prompt: "prompt", Profile: profile}
			quote, err := aicapability.QuoteText(input)
			require.NoError(t, err)
			ledger := &recordingLedger{}
			executor := Executor{Ledger: ledger, Admission: NewBoundedAdmission(), BaseTransport: googleLoopbackTransport(t, server.URL),
				Resolve: func(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error) {
					return QualifiedRoute{Profile: profile, Endpoint: googleTestOrigin, APIKey: "synthetic-only"}, nil
				},
				Authorize: func(context.Context, aicapability.TextInputIdentity) error { return nil }}
			output, err := executor.Generate(context.Background(), input, quote, nil)
			require.EqualValues(t, 1, sends.Load())
			if tc.known {
				require.Len(t, ledger.records, 2)
				fact := ledger.records[1]
				require.Equal(t, tc.outcome, fact.Outcome)
				require.Equal(t, 3, fact.PromptTokens)
				require.Equal(t, 11, fact.CompletionTokens)
				require.Equal(t, "interaction-1", fact.ProviderRequestID)
				require.True(t, output.Usage.Known)
				if tc.outcome == aicapability.InvocationSucceeded {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, ErrObservedInvalidSettled)
				}
			} else {
				require.ErrorIs(t, err, ErrOutcomeUnknown)
				require.Len(t, ledger.records, 1, "unpriceable output must leave only the original dispatch fact")
			}
			_, err = executor.Generate(context.Background(), input, quote, nil)
			require.ErrorIs(t, err, ErrOutcomeUnknown)
			require.EqualValues(t, 1, sends.Load(), "same invocation cannot send again")
		})
	}
}

func TestGoogleSharedTransportRejectsRetryRedirectAndPlanner(t *testing.T) {
	profile := googleTestProfile()
	planning := aicapability.TextInputIdentity{OrganizationID: "org", ActorID: "actor", MemberID: "member", Operation: aicapability.OperationAIWorkbenchChatPlan,
		System: "system", Prompt: "prompt", Profile: profile}
	_, err := aicapability.QuoteText(planning)
	require.Error(t, err, "Google Planner is not admitted")
	for _, status := range []int{307, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var sends atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends.Add(1)
				w.Header().Set("Location", googleTestOrigin+"/redirected")
				w.WriteHeader(status)
			}))
			defer server.Close()
			guard, err := NewGuardedClient(GuardConfig{Adapter: AdapterKind("google-interactions"), Endpoint: googleTestOrigin, ModelID: "gemini-3.8-flash",
				MaximumOutputTokens: 64, MaximumRequestBytes: 128 << 10, MaximumResponseBytes: 256 << 10, Timeout: time.Second}, googleLoopbackTransport(t, server.URL), func(context.Context) error { return nil })
			require.NoError(t, err)
			component, err := NewComponent(context.Background(), ComponentConfig{Adapter: AdapterKind("google-interactions"), Endpoint: googleTestOrigin, APIKey: "synthetic-only", ModelID: "gemini-3.8-flash", MaximumOutputTokens: 64}, guard)
			require.NoError(t, err)
			_, err = GenerateText(context.Background(), component, "system", "prompt", guard)
			require.ErrorIs(t, err, ErrOutcomeUnknown)
			require.EqualValues(t, 1, sends.Load())
		})
	}
}
