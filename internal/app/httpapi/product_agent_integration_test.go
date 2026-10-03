package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/aicapability"
	aistore "task-processor/internal/aicapability/store"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/integration/agent/titletext"
	"task-processor/internal/integration/openai"
	resourceadapter "task-processor/internal/integration/orgresource"
	agentstore "task-processor/internal/integration/persistence/agent"
	configstore "task-processor/internal/integration/persistence/agentconfig"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/knowledge"
	"task-processor/internal/ledger/orgresource"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/workbenchcontext"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
	ProductAgentInvocationLedger
	afterReserve func()
}

func (l agentReserveHook) ReserveAIInvocationUsage(ctx context.Context, org, member, invocation string, tokens int64, at time.Time) error {
	err := l.ProductAgentInvocationLedger.ReserveAIInvocationUsage(ctx, org, member, invocation, tokens, at)
	if err == nil {
		l.afterReserve()
	}
	return err
}

type agentReleaseFailure struct {
	aicapability.InvocationUsageSettler
	aicapability.InvocationUsageReservation
}

type agentReleaseHook struct{ ProductAgentInvocationLedger }

func (l agentReleaseHook) SetUsageSettler(s aicapability.InvocationUsageSettler) {
	l.ProductAgentInvocationLedger.SetUsageSettler(agentReleaseFailure{InvocationUsageSettler: s, InvocationUsageReservation: s.(aicapability.InvocationUsageReservation)})
}

func (agentReleaseFailure) ReleaseAIInvocationUsage(context.Context, string, string) error {
	return errors.New("isolated unconfirmed release")
}

func TestProductAgentAcquisitionToReviewUsesRealOwners(t *testing.T) {
	for _, mode := range []string{"observed canonical", "configuration", "guessed without tool", "asset only", "no quota", "revoked before dispatch", "release failed"} {
		t.Run(mode, func(t *testing.T) { testProductAgentOwners(t, mode) })
	}
}

func testProductAgentOwners(t *testing.T, mode string) {
	f := newAcquisitionHTTPFixture(t)
	canonicalMode := mode == "observed canonical" || mode == "configuration" || mode == "knowledge"
	var kf *consumerKnowledgeFixture
	if mode == "knowledge" {
		kf = newConsumerKnowledgeFixture(t, f.owner)
	}
	acquisitionServer := f.server(t)
	op := acquisitionHTTPCall(t, acquisitionServer, "POST", productAcquisitionBase, "operator", "B", uuid.NewString(), `{"source":"https://detail.1688.com/offer/981645030344.html"}`, 200)
	require.NoError(t, reviewstore.InstallSchema(f.owner))
	require.NoError(t, agentstore.InstallSchema(f.owner))
	require.NoError(t, configstore.InstallSchema(f.owner))
	configuration, configErr := configstore.New(f.owner)
	require.NoError(t, configErr)
	_, configErr = configuration.Execute(context.Background(), agentconfig.Command{Scope: agent.Scope{OrganizationID: "B", ActorID: "admin"}, Key: uuid.NewString(), AgentID: "product.title.agent", Operation: "enable", Absent: true})
	require.NoError(t, configErr)
	require.NoError(t, assetstore.AutoMigrate(f.owner))
	assets, err := assetstore.NewRepository(f.owner)
	require.NoError(t, err)
	version, err := strconv.ParseUint(op.CatalogVersion, 10, 64)
	require.NoError(t, err)
	_, err = assets.CommitApproval(context.Background(), productasset.ApprovalCommit{TenantID: "B", ProductKey: op.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: version, ActionID: "approved-for-shein", Assets: []productasset.ApprovedAsset{{ID: "approved-shein-main", RunID: "image-run", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: "https://cdn.example.test/shein.png"}}})
	require.NoError(t, err)
	require.NoError(t, aistore.AutoMigrateInvocationLedger(f.owner))
	require.NoError(t, f.owner.AutoMigrate(&openai.AIClientCredential{}))
	require.NoError(t, resourceadapter.AutoMigrate(f.owner))
	now := time.Now().UTC()
	limit, err := resourceadapter.NewGormMemberLimitRepository(f.owner, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	_, err = limit.SetMonthlyLimit(context.Background(), orgresource.SetMemberLimitExecution{OrganizationID: "B", MemberID: "member-B-operator", ActorID: "admin", OperationID: "point-limit", Target: 10000000})
	require.NoError(t, err)
	require.NoError(t, f.owner.Exec("INSERT INTO saas_organization_resource_buckets (organization_id,resource_type,available,allocated,reserved,consumed,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)", "B", "ai_point", 10000000, 0, 0, 0, now, now).Error)
	if mode == "no quota" {
		require.NoError(t, f.owner.Exec("UPDATE saas_member_ai_point_limits SET monthly_limit = 1 WHERE organization_id = 'B'").Error)
	}
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step := calls.Add(1)
		var citationID string
		if kf != nil {
			var request struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			for _, message := range request.Messages {
				var prompt struct{ Knowledge *knowledge.ContextBundle }
				if json.Unmarshal([]byte(message.Content), &prompt) == nil && prompt.Knowledge != nil {
					require.Len(t, prompt.Knowledge.Entries, 1)
					require.Equal(t, "Frozen brand text", prompt.Knowledge.Entries[0].Text)
					citationID = prompt.Knowledge.Entries[0].Citation.ID
				}
			}
			require.NotEmpty(t, citationID)
		}
		var content string
		switch {
		case mode == "guessed without tool":
			content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Guessed title","EvidenceIDs":["981645030344"]}]}}`
		case step == 1:
			content = `{"Kind":"tool","Tool":{"ID":"product.asset.inspect","Version":"v1.0.0"}}`
		case step == 2 && canonicalMode:
			if kf == nil {
				wire, readErr := io.ReadAll(r.Body)
				require.NoError(t, readErr)
				require.Contains(t, string(wire), "approved-shein-main", "model must see the selected platform's real inventory")
			}
			content = `{"Kind":"tool","Tool":{"ID":"product.canonical.inspect","Version":"v2.0.0"}}`
		case step == 3 && canonicalMode:
			content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Reviewed bottle title","EvidenceIDs":["invented"]}]}}`
		default:
			content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Reviewed bottle title","EvidenceIDs":["981645030344"]}]},"Confidence":[{"Field":"title","Value":0.8,"Known":true}]}`
		}
		if kf != nil && step >= 3 {
			content = strings.TrimSuffix(content, "}") + `,"ContextCitationIDs":["` + citationID + `"]}`
		}
		encoded, _ := json.Marshal(content)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"isolated-grsai","choices":[{"message":{"content":` + string(encoded) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}`))
	}))
	defer provider.Close()
	credentials := openai.NewOrganizationOnlyCredentialResolver(f.owner)
	require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "B", ClientName: "default", APIKey: "isolated-fixture-key", BaseURL: provider.URL + "/v1", Model: "gemini-2.5-flash", APIStyle: "grsai", Enabled: true, TimeoutSecond: 3}))
	m, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"default": openai.NewClientConfig("unused-placeholder", "gemini-2.5-flash", provider.URL+"/v1", 3)}, ConfigResolver: credentials})
	require.NoError(t, err)
	defer m.Close()
	i := authidentity.AuthenticatedIdentity{TenantID: "B", EffectiveOrganizationID: "B", UserID: "operator", EffectiveMemberID: "member-B-operator", TokenExpiresAt: now.Add(time.Hour)}
	route, err := m.ResolveTextRoute(authidentity.WithAuthenticatedIdentity(context.Background(), i), "default")
	require.NoError(t, err)
	ledger := aistore.NewGormInvocationRecorder(f.owner)
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	deps.organizationResolver = workbenchcontext.NewResolver(agentFixtureGrants{f.grants}, "project", "v1", nil)
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	deps.authorizer = auth
	settings := ProductAgentDependencies{Enabled: true, AllowedOrganizationIDs: []string{"B"}, RunDB: f.owner, PointAccountingDB: f.owner, AssetDB: f.owner, ReviewDB: f.owner, Manager: m, Ledger: ledger, TextPolicies: map[string]titletext.AgentTextPolicy{"B": {ProviderID: "grsai", Endpoint: provider.URL + "/v1", APIStyle: "grsai", ClientName: "default", PolicyVersion: "title-review-v1", PricingVersion: "fixture-v1", BoundEvidence: "fixture-metering-v1", Currency: "CNY", InputWindowTokens: 1048576, OutputWindowTokens: 65536, MaximumOutputTokens: 8192, OutputLimitField: "max_tokens", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000, AdmittedRoute: route, PointPricing: &aicapability.ModelPointTariff{PriceVersion: "synthetic-points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 2000000}}}, Limits: agent.Limits{Steps: 12, ModelCalls: 6, Tokens: 5000000, CostMicros: 5000000, Currency: "CNY", Runtime: time.Minute}}
	if kf != nil {
		settings.Knowledge = kf.service
	}
	if mode == "revoked before dispatch" || mode == "release failed" {
		if mode == "release failed" {
			settings.Ledger = agentReleaseHook{ProductAgentInvocationLedger: ledger}
		}
		settings.Ledger = agentReserveHook{ProductAgentInvocationLedger: settings.Ledger, afterReserve: func() { f.grants.revoked.Store(true) }}
	}
	module, err := buildProductAgentModule(context.Background(), f.db, deps, auth, settings, nil)
	require.NoError(t, err)
	app := buildIsolatedApplicationHTTPServer(module.(productAgentModule).routes, deps, 2*time.Minute)
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	key := uuid.NewString()
	path := productAcquisitionBase + "/" + op.OperationID + "/product-agent/runs"
	startBody := `{"targetPlatform":"shein"}`
	var selectedTemplate agentconfig.Receipt
	if mode == "configuration" {
		selectedTemplate, err = configuration.Execute(context.Background(), agentconfig.Command{Scope: agent.Scope{OrganizationID: "B", ActorID: "admin"}, Key: uuid.NewString(), AgentID: "product.title.agent", Operation: "create-template", Input: agentconfig.TemplateInput{Name: "Exact v1", TargetPlatform: "amazon", DefaultKnowledgeBaseID: uuid.NewString()}})
		require.NoError(t, err)
		startBody = `{"targetPlatform":"shein","templateSelection":{"templateId":"` + selectedTemplate.TemplateID + `","revision":"1"}}`
	}
	var originalBundle struct{ ID, Digest string }
	if kf != nil {
		startBody = `{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"` + kf.base.ID + `"}}`
		for _, body := range []string{`{"targetPlatform":"shein","knowledgeSelection":null}`, `{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"` + kf.base.ID + `","revisionId":"` + kf.source.CurrentReadableRevision.ID + `"}}`} {
			code, raw, e := acquisitionHTTPRequest(server, "POST", path, "operator", "B", uuid.NewString(), body)
			require.NoError(t, e)
			require.Equal(t, 400, code, string(raw))
		}
		oversizedKey := uuid.NewString()
		code, raw, e := acquisitionHTTPRequest(server, "POST", path, "operator", "B", oversizedKey, `{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"`+kf.oversizedBase()+`"}}`)
		require.NoError(t, e)
		require.Equal(t, 409, code, string(raw))
		require.Contains(t, string(raw), knowledge.ErrContextTooLarge.Error())
		code, raw, e = acquisitionHTTPRequest(server, "POST", path, "operator", "B", uuid.NewString(), `{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"`+kf.escapedBase()+`"}}`)
		require.NoError(t, e)
		require.Equal(t, 409, code, string(raw))
		require.Contains(t, string(raw), knowledge.ErrContextTooLarge.Error(), "JSON re-escaping must reject before Agent Claim")
		var runs int64
		require.NoError(t, f.owner.Table("product_agent_runs").Count(&runs).Error)
		require.Zero(t, runs)
		for _, table := range []string{"ai_invocations", "saas_organization_resource_reservations"} {
			var count int64
			require.NoError(t, f.owner.Table(table).Count(&count).Error)
			require.Zero(t, count, "oversize rejection must not create model accounting facts")
		}
		require.Zero(t, calls.Load())
		var failClaim atomic.Bool
		failClaim.Store(true)
		require.NoError(t, f.owner.Callback().Create().Before("gorm:create").Register("test-knowledge-claim-failure", func(tx *gorm.DB) {
			if tx.Statement.Table == "product_agent_runs" && failClaim.Swap(false) {
				tx.AddError(errors.New("isolated claim failure"))
			}
		}))
		code, raw, e = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, startBody)
		require.NoError(t, e)
		require.Equal(t, 503, code, string(raw))
		require.Zero(t, calls.Load())
		require.NoError(t, f.owner.Table("knowledge_context_bundles").Select("id,digest").Where("request_key = ?", key).Take(&originalBundle).Error)
		require.NoError(t, f.owner.Table("product_agent_runs").Count(&runs).Error)
		require.Zero(t, runs)
		kf.replaceSource()
	}
	for _, body := range []string{`{}`, `{"targetPlatform":"product"}`, `{"targetPlatform":""}`} {
		code, raw, err := acquisitionHTTPRequest(server, "POST", path, "operator", "B", uuid.NewString(), body)
		require.NoError(t, err)
		require.Equal(t, 400, code, string(raw))
		require.Zero(t, calls.Load())
	}
	if kf != nil {
		type startResponse struct {
			code int
			raw  []byte
			err  error
		}
		responses := make(chan startResponse, 2)
		for range 2 {
			go func() {
				code, raw, err := acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, startBody)
				responses <- startResponse{code, raw, err}
			}()
		}
		for range 2 {
			response := <-responses
			require.NoError(t, response.err)
			require.Equal(t, 200, response.code, string(response.raw))
		}
		require.EqualValues(t, 4, calls.Load(), "concurrent retry acquires one Agent run/model sequence")
	}
	code, raw, err := acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, startBody)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var result productAgentResultDTO
	require.NoError(t, json.Unmarshal(raw, &result))
	if kf != nil {
		require.NotNil(t, result.Knowledge)
		require.Equal(t, "available", result.Knowledge.Status)
		require.Equal(t, "Frozen brand text", result.Knowledge.Citations[0].Excerpt)
		require.Equal(t, kf.source.CurrentReadableRevision.ID, result.Knowledge.Citations[0].RevisionID)
		var run struct{ Payload []byte }
		require.NoError(t, f.owner.Table("product_agent_runs").Select("payload").Take(&run).Error)
		require.NotContains(t, string(run.Payload), "Frozen brand text")
		require.Contains(t, string(run.Payload), originalBundle.ID)
		require.Contains(t, string(run.Payload), originalBundle.Digest)
		var bundles int64
		require.NoError(t, f.owner.Table("knowledge_context_bundles").Where("request_key = ?", key).Count(&bundles).Error)
		require.EqualValues(t, 1, bundles)
	}
	if mode == "configuration" {
		require.Equal(t, &agentconfig.TemplateRef{TemplateID: selectedTemplate.TemplateID, Revision: "1"}, result.TemplateSelection)
		require.Nil(t, result.Knowledge, "template default never opts into Knowledge")
		configurationScope := agent.Scope{OrganizationID: "B", ActorID: "admin"}
		_, err = configuration.Execute(context.Background(), agentconfig.Command{Scope: configurationScope, Key: uuid.NewString(), AgentID: "product.title.agent", Operation: "update-template", TemplateID: selectedTemplate.TemplateID, Expected: 1, Input: agentconfig.TemplateInput{Name: "new v2", TargetPlatform: "temu"}})
		require.NoError(t, err)
		_, err = configuration.Execute(context.Background(), agentconfig.Command{Scope: configurationScope, Key: uuid.NewString(), AgentID: "product.title.agent", Operation: "archive-template", TemplateID: selectedTemplate.TemplateID, Expected: 2})
		require.NoError(t, err)
		_, err = configuration.Execute(context.Background(), agentconfig.Command{Scope: configurationScope, Key: uuid.NewString(), AgentID: "product.title.agent", Operation: "disable", Expected: 1})
		require.NoError(t, err)
		code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", uuid.NewString(), startBody)
		require.NoError(t, err)
		require.Equal(t, 409, code, string(raw))
		require.Contains(t, string(raw), "AGENT_NOT_ENABLED")
		require.EqualValues(t, 4, calls.Load())
		application := module.(productAgentModule).application
		recent, _, recentErr := configuration.Recent(context.Background(), agent.Scope{OrganizationID: "B", ActorID: "operator"}, "product.title.agent", "", 20, application.store, func(b agent.Binding) error { require.Equal(t, op.ProductKey, b.ProductKey); return nil })
		require.NoError(t, recentErr)
		require.Len(t, recent, 1)
		require.Equal(t, result.RunID, recent[0].RunID)
		denied, _, recentErr := configuration.Recent(context.Background(), agent.Scope{OrganizationID: "B", ActorID: "other"}, "product.title.agent", "", 20, application.store, func(agent.Binding) error { return nil })
		require.NoError(t, recentErr)
		require.Empty(t, denied)
	}
	if mode == "release failed" {
		require.Equal(t, agent.StopModelUnknown, result.StopReason)
		require.Positive(t, result.Tokens)
		require.Equal(t, "unknown_reserved", result.UsageStatus)
		require.Zero(t, calls.Load())
		var remaining int64
		require.NoError(t, f.owner.Table("saas_organization_resource_reservations").Where("owner_type = ? AND state = ?", "model_invocation_v1", "reserved").Count(&remaining).Error)
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
		require.NoError(t, f.owner.Table("saas_organization_resource_reservations").Where("owner_type = ? AND state = ?", "model_invocation_v1", "reserved").Count(&remaining).Error)
		require.Zero(t, remaining, "no provider call must not occupy quota")
		var terminal struct{ Outcome string }
		require.NoError(t, f.owner.Table("ai_invocations").Select("outcome").Take(&terminal).Error)
		require.Equal(t, string(aicapability.InvocationFailed), terminal.Outcome)
		if mode == "revoked before dispatch" {
			require.NoError(t, f.owner.Table("saas_organization_resource_reservations").Where("owner_type = ? AND state = ?", "model_invocation_v1", "released").Count(&remaining).Error)
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
	code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, startBody)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.EqualValues(t, 4, calls.Load())
	if kf != nil {
		code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"shein"}`)
		require.NoError(t, err)
		require.Equal(t, 409, code, string(raw))
		require.EqualValues(t, 4, calls.Load())
		code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", key, `{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"`+uuid.NewString()+`"}}`)
		require.NoError(t, err)
		require.Equal(t, 409, code, string(raw))
		require.EqualValues(t, 4, calls.Load())
	}
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
	if kf != nil {
		code, raw, err = acquisitionHTTPRequest(server, "GET", titleBasePath+"/"+view.ID, "operator", "B", "", "")
		require.NoError(t, err)
		require.Equal(t, 200, code, string(raw))
		require.Contains(t, string(raw), "Frozen brand text")
		// A fresh Review handler reloads the durable opaque origin, not copied text.
		reloaded, buildErr := buildProductAgentModule(context.Background(), f.db, deps, auth, settings, nil)
		require.NoError(t, buildErr)
		app = buildIsolatedApplicationHTTPServer(reloaded.(productAgentModule).routes, deps, 2*time.Minute)
		reopened := httptest.NewServer(app.Handler)
		defer reopened.Close()
		code, raw, err = acquisitionHTTPRequest(reopened, "GET", titleBasePath+"/"+view.ID, "operator", "B", "", "")
		require.NoError(t, err)
		require.Equal(t, 200, code, string(raw))
		require.Contains(t, string(raw), kf.source.CurrentReadableRevision.ID)
		require.EqualValues(t, 4, calls.Load())
		f.grants.revoked.Store(true)
		code, raw, err = acquisitionHTTPRequest(reopened, "GET", titleBasePath+"/"+view.ID, "operator", "B", "", "")
		require.NoError(t, err)
		require.Equal(t, 403, code, string(raw))
		require.NotContains(t, string(raw), "Frozen brand text")
		require.NotContains(t, string(raw), "Brand wording")
		f.grants.revoked.Store(false)
		_, disableErr := kf.service.Mutate(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "base_disable", Key: uuid.NewString(), BaseID: kf.base.ID, Version: kf.base.Version})
		require.NoError(t, disableErr)
		// An already claimed Start replays only its original opaque request. It
		// needs no disabled content and must not send another model request.
		code, raw, err = acquisitionHTTPRequest(reopened, "POST", path, "operator", "B", key, startBody)
		require.NoError(t, err)
		require.Equal(t, 200, code, string(raw))
		require.Contains(t, string(raw), result.RunID)
		require.Contains(t, string(raw), `"status":"unavailable"`)
		require.NotContains(t, string(raw), "Frozen brand text")
		require.NotContains(t, string(raw), "Brand wording")
		require.EqualValues(t, 4, calls.Load())
		for _, changed := range []string{`{"targetPlatform":"shein"}`, `{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"` + uuid.NewString() + `"}}`} {
			code, raw, err = acquisitionHTTPRequest(reopened, "POST", path, "operator", "B", key, changed)
			require.NoError(t, err)
			require.Equal(t, 409, code, string(raw))
		}
		code, raw, err = acquisitionHTTPRequest(reopened, "POST", path, "operator", "B", uuid.NewString(), startBody)
		require.NoError(t, err)
		require.Equal(t, 409, code, string(raw))
		require.EqualValues(t, 4, calls.Load(), "disabled context cannot authorize a new claim or dispatch")
		// A bundle without a claimed run cannot use the metadata-only replay
		// seam to pass disablement, even under its original command identity.
		unclaimedBase, createErr := kf.service.Mutate(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "base_create", Key: uuid.NewString(), Name: "Unclaimed guide"})
		require.NoError(t, createErr)
		kf.addSource(unclaimedBase.Base.ID, "Never dispatched", "")
		unclaimedKey := uuid.NewString()
		unclaimedBody := `{"targetPlatform":"shein","knowledgeSelection":{"knowledgeBaseId":"` + unclaimedBase.Base.ID + `"}}`
		var failUnclaimed atomic.Bool
		failUnclaimed.Store(true)
		require.NoError(t, f.owner.Callback().Create().Before("gorm:create").Register("test-disable-unclaimed", func(tx *gorm.DB) {
			if tx.Statement.Table == "product_agent_runs" && failUnclaimed.Swap(false) {
				tx.AddError(errors.New("isolated unclaimed failure"))
			}
		}))
		code, raw, err = acquisitionHTTPRequest(reopened, "POST", path, "operator", "B", unclaimedKey, unclaimedBody)
		require.NoError(t, err)
		require.Equal(t, 503, code, string(raw))
		_, disableErr = kf.service.Mutate(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "base_disable", Key: uuid.NewString(), BaseID: unclaimedBase.Base.ID, Version: unclaimedBase.Base.Version})
		require.NoError(t, disableErr)
		code, raw, err = acquisitionHTTPRequest(reopened, "POST", path, "operator", "B", unclaimedKey, unclaimedBody)
		require.NoError(t, err)
		require.Equal(t, 409, code, string(raw))
		require.Contains(t, string(raw), knowledge.ErrInactive.Error())
		var unclaimedRuns int64
		require.NoError(t, f.owner.Table("product_agent_runs").Where("request_key = ?", unclaimedKey).Count(&unclaimedRuns).Error)
		require.Zero(t, unclaimedRuns)
		require.EqualValues(t, 4, calls.Load())
		code, raw, err = acquisitionHTTPRequest(reopened, "GET", titleBasePath+"/"+view.ID, "operator", "B", "", "")
		require.NoError(t, err)
		require.Equal(t, 200, code, string(raw))
		require.Contains(t, string(raw), `"status":"unavailable"`)
		require.NotContains(t, string(raw), "Frozen brand text")
		require.NotContains(t, string(raw), "Brand wording")
		var activePermits int64
		require.NoError(t, f.owner.Table("knowledge_dispatch_permits").Where("state = ?", "ACTIVE").Count(&activePermits).Error)
		require.Zero(t, activePermits)
	}
	var callsCount int64
	require.NoError(t, f.owner.Table("product_agent_tool_calls").Count(&callsCount).Error)
	require.EqualValues(t, 2, callsCount)
	var usage struct{ Quantity int64 }
	require.NoError(t, f.owner.Table("saas_organization_resource_events").Select("SUM(consumed_delta) AS quantity").Where("source_type = ?", "model_invocation_v1").Scan(&usage).Error)
	require.EqualValues(t, 160, usage.Quantity)
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
