package einomodel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/aicapability"
	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
	governed "task-processor/internal/integration/aicapability/einomodel"
)

type testSnapshotReader struct{ snapshot agentconfig.Snapshot }

func (r testSnapshotReader) LoadSnapshot(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (agentconfig.Snapshot, error) {
	return r.snapshot, nil
}

type testLedger struct {
	claimed bool
	records []aicapability.InvocationRecord
}

func (l *testLedger) ClaimInvocation(_ context.Context, r aicapability.InvocationRecord) (bool, error) {
	l.records = append(l.records, r)
	if l.claimed {
		return false, nil
	}
	l.claimed = true
	return true, nil
}
func (l *testLedger) ReserveAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error {
	return nil
}
func (l *testLedger) ReleaseAIInvocationUsage(context.Context, string, string) error { return nil }
func (l *testLedger) RecordInvocation(_ context.Context, r aicapability.InvocationRecord) error {
	l.records = append(l.records, r)
	return nil
}

func TestTitleAdapterUsesFrozenProfileAndSharedEinoExecutor(t *testing.T) {
	sends := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sends++
		_, _ = fmt.Fprint(w, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"synthetic-title","choices":[{"index":0,"message":{"role":"assistant","content":"{\"Kind\":\"interrupt\",\"Tool\":{\"ID\":\"\",\"Version\":\"\"},\"Candidate\":{\"Changes\":null},\"Unresolved\":[\"need evidence\"],\"Confidence\":[]}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	}))
	defer server.Close()
	endpointHash := sha256.Sum256([]byte(server.URL))
	profile := aicapability.ModelProfile{ClientName: "title", ProviderID: "synthetic", AdapterKind: "openai-compatible",
		ModelID: "synthetic-title", EndpointIdentityDigest: hex.EncodeToString(endpointHash[:]), CredentialVersion: "v1",
		RoutingPolicyVersion: "route-v1", AdapterPolicyVersion: "adapter-v1", PromptVersion: "prompt-v1",
		OutputSchemaVersion: "schema-v1", UsageMappingVersion: "usage-v1", CostPricingVersion: "cost-v1",
		PointTariff: aicapability.ModelPointTariff{PriceVersion: "points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 1000000},
		Currency:    "USD", InputMicrosPerMillion: 1000000, OutputMicrosPerMillion: 1000000,
		MaximumPromptTokens: 8192, MaximumCompletionTokens: 64, MaximumInputBytes: 16 << 10,
		MaximumOutputBytes: 16 << 10, DeadlineBound: 5 * time.Second}
	identity := authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1",
		UserID: "actor-1", EffectiveMemberID: "member-1", TokenExpiresAt: time.Now().Add(time.Hour)}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	ref := agent.ConfigurationSnapshotRef{Kind: agentconfig.SnapshotKind, ID: uuid.NewString(), Digest: fmt.Sprintf("%064x", 1)}
	binding := agent.Binding{ContextKind: "acquisition", ContextID: "op-1", ProductKey: "product-1", CatalogVersion: "1", PublicationID: "publication-1", TargetPlatform: "shein"}
	snapshot := agentconfig.Snapshot{AgentID: "product.title.agent", AgentVersion: "v1.0.0", ExecutionModelProfile: profile,
		Request: agent.Request{Binding: binding, PolicyVersion: "title-review-v1", PromptVersion: "product-title-agent-v1",
			Limits: agent.Limits{Steps: 12, ModelCalls: 6, Tokens: 10000, CostMicros: 10000, Currency: "USD", Runtime: time.Minute}}}
	ledger := &testLedger{}
	executor := &governed.Executor{Ledger: ledger,
		Authorize: func(context.Context, aicapability.TextInputIdentity) error { return nil },
		Resolve: func(context.Context, aicapability.TextInputIdentity) (governed.QualifiedRoute, error) {
			return governed.QualifiedRoute{Profile: profile, Endpoint: server.URL, APIKey: "synthetic-only"}, nil
		}}
	model, err := NewAgentTextModel(executor, testSnapshotReader{snapshot},
		func(context.Context, string) (aicapability.ModelProfile, error) { return profile, nil },
		[]commercetool.ToolRef{{ID: "product.snapshot.read", Version: "v1"}},
		func(context.Context) (authidentity.AuthenticatedIdentity, error) { return identity, nil })
	require.NoError(t, err)
	input := agent.ModelInput{ConfigurationSnapshotRef: ref, Binding: binding, PolicyVersion: "title-review-v1",
		PromptVersion: "product-title-agent-v1", AgentRunID: uuid.NewString(), AgentID: "product.title.agent", AgentVersion: "v1.0.0"}
	quote, err := model.Quote(ctx, input)
	require.NoError(t, err)
	require.True(t, quote.Known)
	input.UpperBound, input.InvocationID = quote, uuid.NewString()
	result, err := model.Decide(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "interrupt", result.Action.Kind)
	require.Equal(t, int64(15), result.Usage.Tokens)
	require.Equal(t, 1, sends)
	require.Len(t, ledger.records, 2)
	require.Equal(t, aicapability.InvocationSucceeded, ledger.records[1].Outcome)
	for _, record := range ledger.records {
		require.Empty(t, record.BusinessTaskID, "acquisition operation ID is not a BusinessTask ID")
		require.Equal(t, input.AgentRunID, record.AgentRunID)
	}
}
