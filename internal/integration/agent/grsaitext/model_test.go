package grsaitext

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
	"task-processor/internal/agent"
	"task-processor/internal/aicapability"
	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
	"task-processor/internal/integration/openai"
)

type agentTestLedger struct {
	mu                                sync.Mutex
	rows                              map[string]aicapability.InvocationRecord
	claims, reserves, terminals       int
	claimErr, reserveErr, terminalErr bool
}

func (l *agentTestLedger) ClaimInvocation(_ context.Context, r aicapability.InvocationRecord) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.claims++
	if l.claimErr {
		return false, errors.New("unknown claim")
	}
	if _, ok := l.rows[r.InvocationID]; ok {
		return false, nil
	}
	l.rows[r.InvocationID] = r
	return true, nil
}
func (l *agentTestLedger) ReserveAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reserves++
	if l.reserveErr {
		return errors.New("quota unavailable")
	}
	return nil
}
func (l *agentTestLedger) ReleaseAIInvocationUsage(context.Context, string, string) error { return nil }
func (l *agentTestLedger) RecordInvocation(_ context.Context, r aicapability.InvocationRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.terminals++
	if l.terminalErr {
		return errors.New("unknown terminal")
	}
	l.rows[r.InvocationID] = r
	return nil
}

func agentModelFixture(t *testing.T, content, usage string) (*AgentTextModel, context.Context, agent.ModelInput, *agentTestLedger, *atomic.Int32, *atomic.Value) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		encoded, _ := json.Marshal(content)
		_, _ = w.Write([]byte(`{"id":"provider-test","choices":[{"message":{"content":` + string(encoded) + `},"finish_reason":"stop"}],"usage":` + usage + `}`))
	}))
	t.Cleanup(srv.Close)
	ledger := &agentTestLedger{rows: map[string]aicapability.InvocationRecord{}}
	identity := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member", TokenExpiresAt: time.Now().Add(time.Hour)}
	var fresh atomic.Value
	fresh.Store(identity)
	manager := textTestManager(t, srv.URL)
	credentialDB := openTestCredentialDB(t)
	sqlDB, err := credentialDB.DB()
	requireNoErrorText(t, err)
	// This fixture uses SQLite :memory:, whose schema belongs to one connection.
	sqlDB.SetMaxOpenConns(1)
	resolver := openai.NewOrganizationCredentialResolver(credentialDB)
	requireNoErrorText(t, resolver.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "org", ClientName: "text", APIKey: "test-only", BaseURL: srv.URL + "/v1", Model: "gemini-2.5-flash", APIStyle: "grsai", Enabled: true, TimeoutSecond: 2}))
	manager.SetConfigResolver(resolver)
	m, err := NewAgentTextModel(manager, ledger, AgentTextPolicy{
		ClientName: "text", PolicyVersion: "title-review-v1", PricingVersion: "test-price-v1", BoundEvidence: "isolated-fixture-v1", Currency: "CNY", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000,
	}, []commercetool.ToolRef{{ID: "product.snapshot.read", Version: "v1"}}, func(context.Context) (authidentity.AuthenticatedIdentity, error) {
		return fresh.Load().(authidentity.AuthenticatedIdentity), nil
	})
	requireNoErrorText(t, err)
	in := agent.ModelInput{Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation", ProductKey: "product", CatalogVersion: "1", PublicationID: "publication", TargetPlatform: "product"}, PolicyVersion: "title-review-v1", PromptVersion: "agent-title-v1", AgentRunID: "run", AgentID: "product-agent", AgentVersion: "v1"}
	m.policy.AdmittedRoute, err = manager.ResolveTextRoute(authidentity.WithAuthenticatedIdentity(context.Background(), identity), "text")
	requireNoErrorText(t, err)
	return m, authidentity.WithAuthenticatedIdentity(context.Background(), identity), in, ledger, &calls, &fresh
}

func TestAgentTextModelClaimAndObservedInvalidOutput(t *testing.T) {
	for _, content := range []string{`{"Kind":"interrupt","Unresolved":["more evidence needed"]}`, `not JSON`, `{"Kind":"propose","unexpected":"field"}`} {
		t.Run(content, func(t *testing.T) {
			m, ctx, in, ledger, calls, _ := agentModelFixture(t, content, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
			q, err := m.Quote(ctx, in)
			requireNoErrorText(t, err)
			if !q.Known || q.Reference == "" {
				t.Fatalf("unbound quote: %+v", q)
			}
			in.UpperBound = q
			in.InvocationID = "run:step:1"
			result, err := m.Decide(ctx, in)
			requireNoErrorText(t, err)
			if !result.Usage.Known || result.Usage.Tokens != 5 || result.InvocationID != in.InvocationID {
				t.Fatalf("result %+v", result)
			}
			want := aicapability.InvocationUsageObservedFailed
			if content[0] == '{' && result.Action.Kind == "interrupt" {
				want = aicapability.InvocationSucceeded
			}
			if ledger.rows[in.InvocationID].Outcome != want {
				t.Fatalf("ledger %+v", ledger.rows)
			}
			_, err = m.Decide(ctx, in)
			if err == nil || calls.Load() != 1 || ledger.reserves != 1 || ledger.terminals != 1 {
				t.Fatalf("replay dispatched: calls=%d reserve=%d terminal=%d err=%v", calls.Load(), ledger.reserves, ledger.terminals, err)
			}
		})
	}
}

func TestAgentTextModelBindsFreshMembershipAndAdmission(t *testing.T) {
	for _, change := range []string{"member", "evidence", "price", "input", "reference"} {
		t.Run(change, func(t *testing.T) {
			m, ctx, in, ledger, calls, fresh := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
			q, err := m.Quote(ctx, in)
			requireNoErrorText(t, err)
			in.UpperBound = q
			in.InvocationID = "run:step:1"
			switch change {
			case "member":
				id := fresh.Load().(authidentity.AuthenticatedIdentity)
				id.EffectiveMemberID = "new-grant"
				fresh.Store(id)
			case "evidence":
				m.policy.BoundEvidence = ""
			case "price":
				m.policy.PricingVersion = "changed"
			case "input":
				in.UserFeedback = "changed"
			case "reference":
				in.UpperBound.Reference = "changed"
			}
			_, err = m.Decide(ctx, in)
			if err == nil || calls.Load() != 0 || ledger.claims != 0 {
				t.Fatalf("changed %s dispatched: %v", change, err)
			}
		})
	}
}

func TestAgentTextModelFailuresNeverRedispatchOrInventUsage(t *testing.T) {
	for _, mode := range []string{"claim", "reserve", "terminal", "missing usage", "over bound"} {
		t.Run(mode, func(t *testing.T) {
			usage := `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`
			if mode == "missing usage" {
				usage = `{"prompt_tokens":2,"total_tokens":5}`
			}
			if mode == "over bound" {
				usage = `{"prompt_tokens":1048577,"completion_tokens":3,"total_tokens":1048580}`
			}
			m, ctx, in, ledger, calls, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, usage)
			ledger.claimErr = mode == "claim"
			ledger.reserveErr = mode == "reserve"
			ledger.terminalErr = mode == "terminal"
			q, err := m.Quote(ctx, in)
			requireNoErrorText(t, err)
			in.UpperBound = q
			in.InvocationID = "run:step:1"
			result, err := m.Decide(ctx, in)
			if err == nil || result.Usage.Known {
				t.Fatalf("failure invented known consumption: %+v %v", result, err)
			}
			_, _ = m.Decide(ctx, in)
			if calls.Load() > 1 || ((mode == "claim" || mode == "reserve") && calls.Load() != 0) {
				t.Fatalf("calls %d", calls.Load())
			}
			if (mode == "missing usage" || mode == "over bound") && ledger.rows[in.InvocationID].Outcome != aicapability.InvocationDispatched {
				t.Fatal("unknown consumption released")
			}
		})
	}
}

func TestAgentTextModelRechecksMemberAtDispatch(t *testing.T) {
	m, ctx, in, ledger, calls, fresh := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	q, err := m.Quote(ctx, in)
	requireNoErrorText(t, err)
	in.UpperBound, in.InvocationID = q, "run:step:1"
	var checks int
	m.freshIdentity = func(context.Context) (authidentity.AuthenticatedIdentity, error) {
		checks++
		identity := fresh.Load().(authidentity.AuthenticatedIdentity)
		if checks > 1 {
			identity.EffectiveMemberID = "replacement-grant"
		}
		return identity, nil
	}
	_, err = m.Decide(ctx, in)
	if err == nil || checks != 2 || calls.Load() != 0 || ledger.claims != 1 || ledger.reserves != 1 {
		t.Fatalf("dispatch accepted changed grant: checks=%d calls=%d err=%v", checks, calls.Load(), err)
	}
}

func TestAgentTextModelConcurrentSameInvocationSendsOnce(t *testing.T) {
	m, ctx, in, ledger, calls, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	q, err := m.Quote(ctx, in)
	requireNoErrorText(t, err)
	in.UpperBound, in.InvocationID = q, "run:step:1"
	var done sync.WaitGroup
	done.Add(8)
	for i := 0; i < 8; i++ {
		go func() { defer done.Done(); _, _ = m.Decide(ctx, in) }()
	}
	done.Wait()
	if calls.Load() != 1 || ledger.reserves != 1 || ledger.terminals != 1 {
		t.Fatalf("duplicate dispatch/settlement: calls=%d reserves=%d terminals=%d", calls.Load(), ledger.reserves, ledger.terminals)
	}
}

func requireNoErrorText(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func textTestManager(t *testing.T, base string) *openai.Manager {
	t.Helper()
	m, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"text": openai.NewClientConfig("test-only", "gemini-2.5-flash", base+"/v1", 2)}})
	requireNoErrorText(t, err)
	t.Cleanup(func() { _ = m.Close() })
	return m
}
func openTestCredentialDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	requireNoErrorText(t, err)
	requireNoErrorText(t, db.AutoMigrate(&openai.AIClientCredential{}))
	raw, err := db.DB()
	requireNoErrorText(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	return db
}
