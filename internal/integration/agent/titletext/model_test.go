package titletext

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	resolver := openai.NewOrganizationOnlyCredentialResolver(credentialDB)
	requireNoErrorText(t, resolver.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "org", ClientName: "text", APIKey: "test-only", BaseURL: srv.URL + "/v1", Model: "gemini-2.5-flash", APIStyle: "grsai", Enabled: true, TimeoutSecond: 2}))
	manager.SetConfigResolver(resolver)
	route, err := manager.ResolveTextRoute(authidentity.WithAuthenticatedIdentity(context.Background(), identity), "text")
	requireNoErrorText(t, err)
	m, err := NewAgentTextModel(manager, ledger, map[string]AgentTextPolicy{"org": {
		ProviderID: "grsai", Endpoint: srv.URL + "/v1", APIStyle: "grsai", ClientName: "text", PolicyVersion: "title-review-v1", PricingVersion: "test-price-v1", BoundEvidence: "isolated-fixture-v1", Currency: "CNY", InputWindowTokens: agentInputWindow, OutputWindowTokens: agentOutputWindow, MaximumOutputTokens: 8192, OutputLimitField: "max_tokens", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000, AdmittedRoute: route,
		PointPricing: &aicapability.ModelPointTariff{PriceVersion: "synthetic-points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 2000000},
	}}, []commercetool.ToolRef{{ID: "product.snapshot.read", Version: "v1"}}, func(context.Context) (authidentity.AuthenticatedIdentity, error) {
		return fresh.Load().(authidentity.AuthenticatedIdentity), nil
	})
	requireNoErrorText(t, err)
	in := agent.ModelInput{Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation", ProductKey: "product", CatalogVersion: "1", PublicationID: "publication", TargetPlatform: "product"}, PolicyVersion: "title-review-v1", PromptVersion: "agent-title-v1", AgentRunID: "run", AgentID: "product-agent", AgentVersion: "v1"}
	return m, authidentity.WithAuthenticatedIdentity(context.Background(), identity), in, ledger, &calls, &fresh
}

func TestAgentTextModelBindsEachOrganizationToItsOwnProviderAndRoute(t *testing.T) {
	newProvider := func() (*httptest.Server, *atomic.Int32) {
		calls := &atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"Kind\":\"interrupt\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
		}))
		return server, calls
	}
	aServer, aCalls := newProvider()
	bServer, bCalls := newProvider()
	t.Cleanup(aServer.Close)
	t.Cleanup(bServer.Close)
	db := openTestCredentialDB(t)
	sqlDB, err := db.DB()
	requireNoErrorText(t, err)
	sqlDB.SetMaxOpenConns(1)
	writer := openai.NewGormCredentialResolver(db)
	for _, credential := range []openai.AIClientCredential{
		{TenantID: "org-a", ClientName: "text", APIKey: "a-key", BaseURL: aServer.URL + "/v1", Model: "model-a", APIStyle: "grsai", Enabled: true, TimeoutSecond: 2},
		{TenantID: "org-b", ClientName: "text", APIKey: "b-key", BaseURL: bServer.URL + "/v1", Model: "model-b", APIStyle: "openai-compatible", Enabled: true, TimeoutSecond: 2},
		{TenantID: "org-b", UserID: "actor-b", ClientName: "text", APIKey: "member-key", BaseURL: aServer.URL + "/v1", Model: "member-model", APIStyle: "grsai", Enabled: true, TimeoutSecond: 2},
	} {
		requireNoErrorText(t, writer.SaveCredential(context.Background(), credential))
	}
	manager := textTestManager(t, aServer.URL)
	manager.SetConfigResolver(openai.NewOrganizationOnlyCredentialResolver(db))
	identities := map[string]authidentity.AuthenticatedIdentity{}
	policies := map[string]AgentTextPolicy{}
	for _, item := range []struct{ org, actor, member, provider, endpoint, style string }{{"org-a", "actor-a", "member-a", "grsai", aServer.URL + "/v1", "grsai"}, {"org-b", "actor-b", "member-b", "vendor-b", bServer.URL + "/v1", "openai-compatible"}} {
		identity := authidentity.AuthenticatedIdentity{TenantID: item.org, EffectiveOrganizationID: item.org, UserID: item.actor, EffectiveMemberID: item.member, TokenExpiresAt: time.Now().Add(time.Hour)}
		identities[item.org] = identity
		route, routeErr := manager.ResolveTextRoute(authidentity.WithAuthenticatedIdentity(context.Background(), identity), "text")
		requireNoErrorText(t, routeErr)
		policies[item.org] = AgentTextPolicy{ProviderID: item.provider, Endpoint: item.endpoint, APIStyle: item.style, ClientName: "text", PolicyVersion: "title-review-v1", PricingVersion: "test-price-v1", BoundEvidence: "isolated-fixture-v1", Currency: "CNY", InputWindowTokens: 1048576, OutputWindowTokens: 65536, MaximumOutputTokens: 8192, OutputLimitField: "max_tokens", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000, PointPricing: &aicapability.ModelPointTariff{PriceVersion: "synthetic-points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 2000000}, AdmittedRoute: route}
	}
	current := &atomic.Value{}
	current.Store(identities["org-a"])
	ledger := &agentTestLedger{rows: map[string]aicapability.InvocationRecord{}}
	model, err := NewAgentTextModel(manager, ledger, policies, []commercetool.ToolRef{{ID: "product.snapshot.read", Version: "v1"}}, func(context.Context) (authidentity.AuthenticatedIdentity, error) {
		return current.Load().(authidentity.AuthenticatedIdentity), nil
	})
	requireNoErrorText(t, err)
	in := agent.ModelInput{Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation", ProductKey: "product", CatalogVersion: "1", PublicationID: "publication", TargetPlatform: "product"}, PolicyVersion: "title-review-v1", PromptVersion: "agent-title-v1", AgentRunID: "run", AgentID: "product-agent", AgentVersion: "v1"}
	aQuote, err := model.Quote(authidentity.WithAuthenticatedIdentity(context.Background(), identities["org-a"]), in)
	requireNoErrorText(t, err)
	current.Store(identities["org-b"])
	bCtx := authidentity.WithAuthenticatedIdentity(context.Background(), identities["org-b"])
	bQuote, err := model.Quote(bCtx, in)
	requireNoErrorText(t, err)
	if aQuote.Reference == bQuote.Reference {
		t.Fatal("different organizations share a title quote")
	}
	in.InvocationID, in.UpperBound = "wrong-route", aQuote
	if _, err := model.Decide(bCtx, in); !errors.Is(err, agent.ErrUnavailable) || ledger.claims != 0 {
		t.Fatalf("cross-organization quote accepted: %v, claims=%d", err, ledger.claims)
	}
	in.InvocationID, in.UpperBound = "org-b-title", bQuote
	_, err = model.Decide(bCtx, in)
	requireNoErrorText(t, err)
	record := ledger.rows[in.InvocationID]
	if record.ProviderID != "vendor-b" || record.ModelID != "model-b" || record.TenantID != "org-b" || aCalls.Load() != 0 || bCalls.Load() != 1 {
		t.Fatalf("wrong route or attribution: %+v calls=%d/%d", record, aCalls.Load(), bCalls.Load())
	}
}

func TestAgentTextModelRouteReadinessSeparatesRolloutFromCredentialRepair(t *testing.T) {
	m, ctx, _, _, _, fresh := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	if got := m.RouteReadiness(ctx); got != TextRouteAvailable {
		t.Fatalf("ready route = %s", got)
	}
	originalPolicy := m.policies["org"]
	endpointPolicy := originalPolicy
	endpointPolicy.Endpoint = "https://different.example.test/v1"
	m.policies["org"] = endpointPolicy
	if got := m.RouteReadiness(ctx); got != TextRouteNeedsConfiguration {
		t.Fatalf("admitted endpoint differs from credential: %s", got)
	}
	m.policies["org"] = originalPolicy
	originalFresh := m.freshIdentity
	m.freshIdentity = func(context.Context) (authidentity.AuthenticatedIdentity, error) {
		return authidentity.AuthenticatedIdentity{}, errors.New("no execution permission")
	}
	if got := m.RouteReadiness(ctx); got != TextRouteUnavailable {
		t.Fatalf("execution permission denial = %s", got)
	}
	if got := m.RouteReadinessForVerifiedOrganization(ctx, "org"); got != TextRouteAvailable {
		t.Fatalf("organization configuration should be independent of actor use permission: %s", got)
	}
	m.freshIdentity = originalFresh
	policy := m.policies["org"]
	policy.AdmittedRoute.ConfigurationVersion = "stale-version"
	m.policies["org"] = policy
	if got := m.RouteReadiness(ctx); got != TextRouteNeedsConfiguration {
		t.Fatalf("mismatched organization credential = %s", got)
	}
	delete(m.policies, "org")
	if got := m.RouteReadiness(ctx); got != TextRouteUnavailable {
		t.Fatalf("missing operator policy = %s", got)
	}
	m.policies["org"] = policy
	identity := fresh.Load().(authidentity.AuthenticatedIdentity)
	identity.TokenExpiresAt = time.Now().Add(-time.Minute)
	fresh.Store(identity)
	if got := m.RouteReadiness(ctx); got != TextRouteUnavailable {
		t.Fatalf("expired fresh identity = %s", got)
	}
}

func TestAgentTextModelRejectsPromptBeyondAdmittedInputWindowBeforeClaim(t *testing.T) {
	m, ctx, in, ledger, calls, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	policy := m.policies["org"]
	policy.InputWindowTokens = 4096
	m.policies["org"] = policy
	in.UserFeedback = strings.Repeat("A", 5000)
	if _, err := m.Quote(ctx, in); !errors.Is(err, openai.ErrTextInput) || ledger.claims != 0 || calls.Load() != 0 {
		t.Fatalf("oversized route prompt was admitted: %v, claims=%d calls=%d", err, ledger.claims, calls.Load())
	}
}

func TestAgentTextModelPointTariffRequiredBeforeClaimAndFrozenInFact(t *testing.T) {
	m, ctx, in, ledger, calls, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	policy := m.policies["org"]
	tariff := *policy.PointPricing
	policy.PointPricing = nil
	m.policies["org"] = policy
	_, err := m.Quote(ctx, in)
	if !errors.Is(err, agent.ErrUnavailable) || ledger.claims != 0 || calls.Load() != 0 {
		t.Fatal("missing point tariff must fail before dispatch")
	}
	policy.PointPricing = &tariff
	m.policies["org"] = policy
	first, err := m.Quote(ctx, in)
	requireNoErrorText(t, err)
	changed := tariff
	changed.PriceVersion = "synthetic-points-v2"
	changed.OutputPointsPerMillionTokens++
	policy.PointPricing = &changed
	m.policies["org"] = policy
	second, err := m.Quote(ctx, in)
	requireNoErrorText(t, err)
	if first.Reference == second.Reference {
		t.Fatal("point tariff not bound in quote")
	}
	in.InvocationID = "frozen-points"
	in.UpperBound = first
	_, err = m.Decide(ctx, in)
	if err == nil || ledger.claims != 0 || calls.Load() != 0 {
		t.Fatal("changed tariff must invalidate original quote")
	}
	in.UpperBound = second
	_, err = m.Decide(ctx, in)
	requireNoErrorText(t, err)
	fact := ledger.rows[in.InvocationID]
	if fact.PointTariff != changed || fact.MaximumPromptTokens != agentInputWindow || fact.MaximumCompletionTokens != agentOutputWindow {
		t.Fatalf("missing frozen point metadata: %+v", fact)
	}
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
				policy := m.policies["org"]
				policy.BoundEvidence = ""
				m.policies["org"] = policy
			case "price":
				policy := m.policies["org"]
				policy.PricingVersion = "changed"
				m.policies["org"] = policy
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
			if err == nil || (mode != "reserve" && result.Usage.Known) {
				t.Fatalf("failure invented known consumption: %+v %v", result, err)
			}
			if mode == "reserve" && (!result.Usage.Known || result.Usage.Tokens != 0 || result.Usage.CostMicros != 0) {
				t.Fatalf("confirmed unexecuted reservation failure must report zero usage: %+v", result)
			}
			if errors.Is(err, agent.ErrModelNotDispatched) != (mode == "reserve") {
				t.Fatalf("unconfirmed claim/terminal or provider outcome was labeled unexecuted: %s %v", mode, err)
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
	result, err := m.Decide(ctx, in)
	if err == nil || checks != 2 || calls.Load() != 0 || ledger.claims != 1 || ledger.reserves != 1 {
		t.Fatalf("dispatch accepted changed grant: checks=%d calls=%d err=%v", checks, calls.Load(), err)
	}
	if ledger.terminals != 1 || ledger.rows[in.InvocationID].Outcome != aicapability.InvocationFailed || !result.Usage.Known || result.Usage.Tokens != 0 {
		t.Fatalf("unexecuted call retained reservation: %+v result=%+v", ledger.rows[in.InvocationID], result)
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

func TestAgentPromptBoundsLargeEvidenceWithoutChangingStoredFacts(t *testing.T) {
	m, ctx, in, _, calls, _ := agentModelFixture(t, `{"Kind":"interrupt"}`, `{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}`)
	raw, err := json.Marshal(map[string]any{"product_key": "product", "catalog_version": "1", "snapshot": map[string]any{"title": "Exact saved title", "brand": "Real brand", "sources": []any{map[string]any{"detail": "source-evidence"}}, "variants": []any{map[string]any{"sku": strings.Repeat("<large-variant>", 6000)}}}})
	requireNoErrorText(t, err)
	// The maximum current run can observe eight tools; hostile escaping in
	// feedback must still leave room for bounded evidence and the SDK envelope.
	for n := 0; n < 8; n++ {
		in.History = append(in.History, agent.Observation{Step: n + 1, Tool: commercetool.ToolRef{ID: "product.canonical.inspect", Version: "v2.0.0"}, CallID: "call", Output: append(json.RawMessage(nil), raw...)})
	}
	in.UserFeedback = strings.Repeat("<", 8192)
	p, err := m.prepare(ctx, in)
	requireNoErrorText(t, err)
	if calls.Load() != 0 || string(in.History[0].Output) != string(raw) {
		t.Fatal("projection dispatched or mutated stored evidence")
	}
	for _, fact := range []string{"Exact saved title", "Real brand", "source-evidence", "omitted_fields", "snapshot.variants", agentTextHash(raw)} {
		if !strings.Contains(p.request.Prompt, fact) {
			t.Fatalf("missing fact/disclosure %s", fact)
		}
	}
	if strings.Contains(p.request.Prompt, "large-variant") {
		t.Fatal("oversized variants entered prompt")
	}
	wire, err := json.Marshal(p.request)
	requireNoErrorText(t, err)
	if len(wire) > openai.MaxTextPromptBytes-4096 {
		t.Fatal("prompt exceeded wire budget")
	}
	in.History[0].Output = json.RawMessage(strings.Replace(string(raw), "large-variant", "other-variant", 1))
	changed, err := m.prepare(ctx, in)
	requireNoErrorText(t, err)
	if changed.quote.Reference == p.quote.Reference {
		t.Fatal("omitted evidence change did not bind quote")
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
