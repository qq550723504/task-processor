package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/agent"
	"task-processor/internal/aicapability"
	aistore "task-processor/internal/aicapability/store"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/integration/agent/grsaitext"
	"task-processor/internal/integration/openai"
	allocationstore "task-processor/internal/integration/persistence/accountallocation"
	agentstore "task-processor/internal/integration/persistence/agent"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/listingsubscription"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/workbenchcontext"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type agentFixtureGrants struct{ base *titleGrants }

func (g agentFixtureGrants) Load(ctx context.Context, source workbenchcontext.GrantSource, r workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	result, err := g.base.Load(ctx, source, r)
	for n := range result.Grants {
		result.Grants[n].AuthorizationID = "member-" + result.Grants[n].OrganizationID + "-" + r.Subject
	}
	return result, err
}
func (g agentFixtureGrants) Invalidate(actor, project string) { g.base.Invalidate(actor, project) }

type agentReserveHook struct {
	grsaitext.AgentInvocationLedger
	afterReserve func()
}

func (l agentReserveHook) ReserveAIInvocationUsage(ctx context.Context, org, member, invocation string, tokens int64, at time.Time) error {
	err := l.AgentInvocationLedger.ReserveAIInvocationUsage(ctx, org, member, invocation, tokens, at)
	if err == nil {
		l.afterReserve()
	}
	return err
}

type agentReleaseFailure struct {
	listingsubscription.AIInvocationUsageAdapter
}

func (agentReleaseFailure) ReleaseAIInvocationUsage(context.Context, string, string) error {
	return errors.New("isolated unconfirmed release")
}

func TestProductAgentAcquisitionToReviewUsesRealOwners(t *testing.T) {
	for _, mode := range []string{"observed canonical", "guessed without tool", "asset only", "no quota", "revoked before dispatch", "release failed"} {
		t.Run(mode, func(t *testing.T) { testProductAgentOwners(t, mode) })
	}
}

func testProductAgentOwners(t *testing.T, mode string) {
	f := newAcquisitionHTTPFixture(t)
	acquisitionServer := f.server(t)
	op := acquisitionHTTPCall(t, acquisitionServer, "POST", productAcquisitionBase, "operator", "B", uuid.NewString(), `{"source":"https://detail.1688.com/offer/981645030344.html"}`, 200)
	require.NoError(t, reviewstore.InstallSchema(f.owner))
	require.NoError(t, agentstore.InstallSchema(f.owner))
	require.NoError(t, assetstore.AutoMigrate(f.owner))
	assets, err := assetstore.NewRepository(f.owner)
	require.NoError(t, err)
	version, err := strconv.ParseUint(op.CatalogVersion, 10, 64)
	require.NoError(t, err)
	_, err = assets.CommitApproval(context.Background(), productasset.ApprovalCommit{TenantID: "B", ProductKey: op.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: version, ActionID: "approved-for-shein", Assets: []productasset.ApprovedAsset{{ID: "approved-shein-main", RunID: "image-run", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: "https://cdn.example.test/shein.png"}}})
	require.NoError(t, err)
	require.NoError(t, aistore.AutoMigrateInvocationLedger(f.owner))
	require.NoError(t, f.owner.AutoMigrate(&openai.AIClientCredential{}))
	require.NoError(t, allocationstore.AutoMigrate(f.owner))
	now := time.Now().UTC()
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	require.NoError(t, f.owner.Exec(`INSERT INTO saas_tenant_entitlements (tenant_id,module_code,status,starts_at,expires_at,limits) VALUES (?,?,?,?,?,?)`, "B", listingsubscription.ModuleListingKit, listingsubscription.StatusActive, start, end, `{"ai_tokens":10000000}`).Error)
	require.NoError(t, f.owner.Exec(`INSERT INTO account_member_token_allocations (organization_id,member_id,metric,allocated,version,active,window_start,window_end,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`, "B", "member-B-operator", "token", 10000000, 1, true, start, end, now).Error)
	if mode == "no quota" {
		require.NoError(t, f.owner.Exec(`UPDATE account_member_token_allocations SET allocated = 1 WHERE organization_id = 'B'`).Error)
	}
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step := calls.Add(1)
		var content string
		switch {
		case mode == "guessed without tool":
			content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Guessed title","EvidenceIDs":["981645030344"]}]}}`
		case step == 1:
			content = `{"Kind":"tool","Tool":{"ID":"product.asset.inspect","Version":"v1.0.0"}}`
		case step == 2 && mode == "observed canonical":
			wire, readErr := io.ReadAll(r.Body)
			require.NoError(t, readErr)
			require.Contains(t, string(wire), "approved-shein-main", "model must see the selected platform's real inventory")
			content = `{"Kind":"tool","Tool":{"ID":"product.canonical.inspect","Version":"v2.0.0"}}`
		case step == 3 && mode == "observed canonical":
			content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Reviewed bottle title","EvidenceIDs":["invented"]}]}}`
		default:
			content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Reviewed bottle title","EvidenceIDs":["981645030344"]}]},"Confidence":[{"Field":"title","Value":0.8,"Known":true}]}`
		}
		encoded, _ := json.Marshal(content)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"isolated-grsai","choices":[{"message":{"content":` + string(encoded) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}`))
	}))
	defer provider.Close()
	credentials := openai.NewOrganizationCredentialResolver(f.owner)
	require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "B", ClientName: "default", APIKey: "isolated-fixture-key", BaseURL: provider.URL + "/v1", Model: "gemini-2.5-flash", APIStyle: "grsai", Enabled: true, TimeoutSecond: 3}))
	m, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"default": openai.NewClientConfig("unused-placeholder", "gemini-2.5-flash", provider.URL+"/v1", 3)}, ConfigResolver: credentials})
	require.NoError(t, err)
	defer m.Close()
	i := authidentity.AuthenticatedIdentity{TenantID: "B", EffectiveOrganizationID: "B", UserID: "operator", EffectiveMemberID: "member-B-operator", TokenExpiresAt: now.Add(time.Hour)}
	route, err := m.ResolveTextRoute(authidentity.WithAuthenticatedIdentity(context.Background(), i), "default")
	require.NoError(t, err)
	ledger := aistore.NewGormInvocationRecorder(f.owner)
	ledger.SetUsageSettler(listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(f.owner)})
	if mode == "release failed" {
		ledger.SetUsageSettler(agentReleaseFailure{listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(f.owner)}})
	}
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	deps.organizationResolver = workbenchcontext.NewResolver(agentFixtureGrants{f.grants}, "project", "v1", nil)
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	deps.authorizer = auth
	settings := ProductAgentDependencies{Enabled: true, AllowedOrganizationIDs: []string{"B"}, RunDB: f.owner, AssetDB: f.owner, ReviewDB: f.owner, Manager: m, Ledger: ledger, TextPolicy: grsaitext.AgentTextPolicy{ClientName: "default", PolicyVersion: "title-review-v1", PricingVersion: "fixture-v1", BoundEvidence: "fixture-metering-v1", Currency: "CNY", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000, AdmittedRoute: route}, Limits: agent.Limits{Steps: 12, ModelCalls: 6, Tokens: 5000000, CostMicros: 5000000, Currency: "CNY", Runtime: time.Minute}}
	if mode == "revoked before dispatch" || mode == "release failed" {
		settings.Ledger = agentReserveHook{AgentInvocationLedger: ledger, afterReserve: func() { f.grants.revoked.Store(true) }}
	}
	module, err := buildProductAgentModule(context.Background(), f.db, deps, auth, settings)
	require.NoError(t, err)
	app := buildIsolatedApplicationHTTPServer(module.(productAgentModule).routes, deps, 2*time.Minute)
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	key := uuid.NewString()
	path := productAcquisitionBase + "/" + op.OperationID + "/product-agent/runs"
	for _, body := range []string{`{}`, `{"targetPlatform":"product"}`, `{"targetPlatform":""}`} {
		code, raw, err := acquisitionHTTPRequest(server, "POST", path, "operator", "B", uuid.NewString(), body)
		require.NoError(t, err)
		require.Equal(t, 400, code, string(raw))
		require.Zero(t, calls.Load())
	}
	code, raw, err := acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"shein"}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var result productAgentResultDTO
	require.NoError(t, json.Unmarshal(raw, &result))
	if mode == "release failed" {
		require.Equal(t, agent.StopModelUnknown, result.StopReason)
		require.Positive(t, result.Tokens)
		require.Equal(t, "unknown_reserved", result.UsageStatus)
		require.Zero(t, calls.Load())
		var remaining int64
		require.NoError(t, f.owner.Table("saas_usage_events").Where("status = ?", "reserved").Count(&remaining).Error)
		require.EqualValues(t, 1, remaining, "failed release must not be acknowledged as zero usage")
		var terminal struct{ Outcome string }
		require.NoError(t, f.owner.Table("ai_invocations").Select("outcome").Take(&terminal).Error)
		require.Equal(t, string(aicapability.InvocationFailed), terminal.Outcome, "terminal fact alone does not confirm release")
		f.grants.revoked.Store(false)
		code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"shein"}`)
		require.NoError(t, err)
		require.Equal(t, 200, code, string(raw))
		require.Zero(t, calls.Load())
		return
	}
	if mode == "no quota" || mode == "revoked before dispatch" {
		require.Equal(t, agent.Stopped, result.Phase, string(raw))
		require.Equal(t, agent.StopDependency, result.StopReason)
		require.Zero(t, result.Tokens)
		require.Zero(t, result.EstimatedCostMicros)
		require.Equal(t, "observed", result.UsageStatus)
		require.Zero(t, calls.Load())
		var remaining int64
		require.NoError(t, f.owner.Table("saas_usage_events").Where("status = ?", "reserved").Count(&remaining).Error)
		require.Zero(t, remaining, "no provider call must not occupy quota")
		var terminal struct{ Outcome string }
		require.NoError(t, f.owner.Table("ai_invocations").Select("outcome").Take(&terminal).Error)
		require.Equal(t, string(aicapability.InvocationFailed), terminal.Outcome)
		if mode == "revoked before dispatch" {
			require.NoError(t, f.owner.Table("saas_usage_events").Where("status = ?", "released").Count(&remaining).Error)
			require.EqualValues(t, 1, remaining)
		}
		return
	}
	if mode == "guessed without tool" || mode == "asset only" {
		require.False(t, result.CanSubmitReview, string(raw))
		require.Equal(t, agent.StopRepairLimit, result.StopReason)
		require.Equal(t, agent.HumanReviewRequired, result.Phase)
		before := calls.Load()
		code, raw, err = acquisitionHTTPRequest(server, "POST", path+"/"+key+"/review", "operator", "B", "", `{}`)
		require.NoError(t, err)
		require.Equal(t, 409, code, string(raw))
		require.Equal(t, before, calls.Load())
		return
	}
	require.Equal(t, agent.HumanReviewRequired, result.Phase, string(raw))
	require.True(t, result.CanSubmitReview)
	require.EqualValues(t, 120, result.Tokens)
	require.EqualValues(t, 4, calls.Load())
	// Replaying Start and reading after reconstructing its HTTP handler never
	// cause an extra model call. The same candidate enters existing Review.
	code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"shein"}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.EqualValues(t, 4, calls.Load())
	code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"temu"}`)
	require.NoError(t, err)
	require.Equal(t, 409, code, string(raw))
	require.EqualValues(t, 4, calls.Load())
	code, raw, err = acquisitionHTTPRequest(server, "GET", path+"/"+key, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"targetPlatform":"shein"`)
	code, raw, err = acquisitionHTTPRequest(server, "POST", path+"/"+key+"/resume", "operator", "B", "", `{"revision":"2","feedback":"continue","targetPlatform":"temu"}`)
	require.NoError(t, err)
	require.Equal(t, 400, code, string(raw))
	code, raw, err = acquisitionHTTPRequest(server, "GET", path+"/"+key, "operator", "A", "", "")
	require.NoError(t, err)
	require.NotEqual(t, 200, code, string(raw))
	require.EqualValues(t, 4, calls.Load())
	code, raw, err = acquisitionHTTPRequest(server, "POST", path+"/"+key+"/review", "operator", "B", "", `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var link struct {
		ProposalID string `json:"proposalId"`
	}
	require.NoError(t, json.Unmarshal(raw, &link))
	view := titleCall(t, server, "GET", titleBasePath+"/"+link.ProposalID, "operator", "B", "", "", 200)
	require.Equal(t, "pending", view.State)
	require.Equal(t, "Reviewed bottle title", view.Title)
	var callsCount int64
	require.NoError(t, f.owner.Table("product_agent_tool_calls").Count(&callsCount).Error)
	require.EqualValues(t, 2, callsCount)
	var usage struct{ Quantity int64 }
	require.NoError(t, f.owner.Table("saas_usage_events").Select("SUM(quantity) AS quantity").Where("source_type = ?", "ai_invocation").Scan(&usage).Error)
	require.EqualValues(t, 120, usage.Quantity)
	// The human, using the original Review endpoint, decides and applies.
	accepted := titleCall(t, server, "POST", titleBasePath+"/"+view.ID+"/decisions", "admin", "B", uuid.NewString(), `{"action":"accept","expected_revision":1}`, 200)
	_ = accepted
	code, raw, err = acquisitionHTTPRequest(server, "POST", titleBasePath+"/"+view.ID+"/apply", "admin", "B", uuid.NewString(), `{"expected_revision":2}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.Equal(t, "applied", decodeTitleView(t, raw).State)
	f.grants.revoked.Store(true)
	code, raw, err = acquisitionHTTPRequest(server, "GET", path+"/"+key, "operator", "B", "", "")
	require.NoError(t, err)
	require.NotEqual(t, 200, code, string(raw))
}
