package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/agent"
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

func TestProductAgentAcquisitionToReviewUsesRealOwners(t *testing.T) {
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
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step := calls.Add(1)
		var content string
		switch step {
		case 1:
			content = `{"Kind":"tool","Tool":{"ID":"product.asset.inspect","Version":"v1.0.0"}}`
		case 2:
			wire, readErr := io.ReadAll(r.Body)
			require.NoError(t, readErr)
			require.Contains(t, string(wire), "approved-shein-main", "model must see the selected platform's real inventory")
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
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	deps.organizationResolver = workbenchcontext.NewResolver(agentFixtureGrants{f.grants}, "project", "v1", nil)
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	deps.authorizer = auth
	settings := ProductAgentDependencies{Enabled: true, AllowedOrganizationIDs: []string{"B"}, RunDB: f.owner, AssetDB: f.owner, ReviewDB: f.owner, Manager: m, Ledger: ledger, TextPolicy: grsaitext.AgentTextPolicy{ClientName: "default", PolicyVersion: "title-review-v1", PricingVersion: "fixture-v1", BoundEvidence: "fixture-metering-v1", Currency: "CNY", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000, AdmittedRoute: route}, Limits: agent.Limits{Steps: 12, ModelCalls: 6, Tokens: 5000000, CostMicros: 5000000, Currency: "CNY", Runtime: time.Minute}}
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
	require.Equal(t, agent.HumanReviewRequired, result.Phase, string(raw))
	require.True(t, result.CanSubmitReview)
	require.EqualValues(t, 90, result.Tokens)
	require.EqualValues(t, 3, calls.Load())
	// Replaying Start and reading after reconstructing its HTTP handler never
	// cause a fourth model call. The same candidate enters existing Review.
	code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"shein"}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.EqualValues(t, 3, calls.Load())
	code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"temu"}`)
	require.NoError(t, err)
	require.Equal(t, 409, code, string(raw))
	require.EqualValues(t, 3, calls.Load())
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
	require.EqualValues(t, 3, calls.Load())
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
	require.EqualValues(t, 1, callsCount)
	var usage struct{ Quantity int64 }
	require.NoError(t, f.owner.Table("saas_usage_events").Select("SUM(quantity) AS quantity").Where("source_type = ?", "ai_invocation").Scan(&usage).Error)
	require.EqualValues(t, 90, usage.Quantity)
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
