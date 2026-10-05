package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/aicapability"
	aistore "task-processor/internal/aicapability/store"
	"task-processor/internal/aiworkbench"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	governed "task-processor/internal/integration/aicapability/einomodel"
	"task-processor/internal/integration/openai"
	resourceadapter "task-processor/internal/integration/orgresource"
	agentstore "task-processor/internal/integration/persistence/agent"
	configstore "task-processor/internal/integration/persistence/agentconfig"
	workstore "task-processor/internal/integration/persistence/aiworkbench"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/knowledge"
	"task-processor/internal/ledger/orgresource"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"
)

type unavailablePublishedReceipt struct{}

type rollbackWorkbenchClaim struct {
	ProductAgentInvocationLedger
	rollback atomic.Bool
}

func (l *rollbackWorkbenchClaim) ClaimInvocation(ctx context.Context, record aicapability.InvocationRecord) (bool, error) {
	if l.rollback.Swap(false) {
		return false, errors.New("synthetic invocation claim rolled back")
	}
	return l.ProductAgentInvocationLedger.ClaimInvocation(ctx, record)
}

type deniedCandidateReviewAuthorizer struct{ review.Authorizer }

func (deniedCandidateReviewAuthorizer) Authorize(string, []string, string) bool { return false }

type failedWorkbenchSettlement struct {
	aicapability.InvocationUsageSettler
	aicapability.InvocationUsageReservation
}

type failedWorkbenchRelease struct {
	aicapability.InvocationUsageSettler
	aicapability.InvocationUsageReservation
}

func (failedWorkbenchRelease) ReleaseAIInvocationUsage(context.Context, string, string) error {
	return errors.New("synthetic commercial release failure")
}

func (failedWorkbenchSettlement) SettleAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error {
	return errors.New("synthetic commercial settlement failure")
}

func (unavailablePublishedReceipt) ReadPublished(context.Context, string) (sourcing.PublishedAcquisition, error) {
	return sourcing.PublishedAcquisition{}, errors.New("published product temporarily unavailable")
}

type taskViewerGrants struct {
	base   *titleGrants
	viewer atomic.Bool
}

func (g *taskViewerGrants) Load(ctx context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	result, err := (agentFixtureGrants{base: g.base}).Load(ctx, source, request)
	if request.Subject == "operator" && g.viewer.Load() {
		for i := range result.Grants {
			if result.Grants[i].OrganizationID == "B" {
				result.Grants[i].Roles = []string{"listingkit_viewer"}
			}
		}
	}
	return result, err
}

func (g *taskViewerGrants) Invalidate(actor, project string) { g.base.Invalidate(actor, project) }

func TestAIWorkbenchChatProposalToBusinessTaskUsesOwners(t *testing.T) {
	t.Run("withoutKnowledge", func(t *testing.T) { testAIWorkbenchOwners(t, false) })
	t.Run("withKnowledge", func(t *testing.T) { testAIWorkbenchOwners(t, true) })
}

func testAIWorkbenchOwners(t *testing.T, withKnowledge bool) {
	f := newAcquisitionHTTPFixture(t)
	var kf *consumerKnowledgeFixture
	if withKnowledge {
		kf = newConsumerKnowledgeFixture(t, f.owner)
	}
	op := acquisitionHTTPCall(t, f.server(t), "POST", productAcquisitionBase, "operator", "B", uuid.NewString(),
		`{"source":"https://detail.1688.com/offer/981645030344.html"}`, 200)
	require.NoError(t, reviewstore.InstallSchema(f.owner))
	require.NoError(t, agentstore.InstallSchema(f.owner))
	require.NoError(t, configstore.InstallSchema(f.owner))
	require.NoError(t, workstore.InstallSchema(f.owner))
	const workbenchRole = "ai_workbench_runtime"
	require.NoError(t, f.owner.Exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='ai_workbench_runtime') THEN
		CREATE ROLE ai_workbench_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
		END IF; END $$`).Error)
	require.NoError(t, f.owner.Exec("ALTER ROLE "+workbenchRole+" PASSWORD 'issue588-only'").Error)
	require.NoError(t, workstore.GrantRuntime(f.owner, workbenchRole))
	var testDatabase string
	require.NoError(t, f.owner.Raw("SELECT current_database()").Scan(&testDatabase).Error)
	require.NoError(t, f.owner.Exec("GRANT CONNECT ON DATABASE \""+testDatabase+"\" TO "+workbenchRole).Error)
	workbenchDB, err := gorm.Open(postgres.Open(os.Getenv("ISSUE398_TEST_DSN")+" dbname="+testDatabase+
		" user="+workbenchRole+" password=issue588-only"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	workbenchPool, err := workbenchDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workbenchPool.Close()) })
	configuration, err := configstore.New(f.owner)
	require.NoError(t, err)
	_, err = configuration.Execute(context.Background(), agentconfig.Command{Scope: agent.Scope{OrganizationID: "B", ActorID: "admin"}, Key: uuid.NewString(), AgentID: "product.title.agent", Operation: "enable", Absent: true})
	require.NoError(t, err)
	require.NoError(t, assetstore.AutoMigrate(f.owner))
	assets, err := assetstore.NewRepository(f.owner)
	require.NoError(t, err)
	version, err := strconv.ParseUint(op.CatalogVersion, 10, 64)
	require.NoError(t, err)
	_, err = assets.CommitApproval(context.Background(), productasset.ApprovalCommit{TenantID: "B", ProductKey: op.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: version,
		ActionID: "approved-for-shein", Assets: []productasset.ApprovedAsset{{ID: "approved-shein-main", RunID: "image-run", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: "https://cdn.example.test/shein.png"}}})
	require.NoError(t, err)
	require.NoError(t, aistore.AutoMigrateInvocationLedger(f.owner))
	require.NoError(t, f.owner.AutoMigrate(&openai.AIClientCredential{}))
	require.NoError(t, resourceadapter.AutoMigrate(f.owner))
	limit, err := resourceadapter.NewGormMemberLimitRepository(f.owner, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	_, err = limit.SetMonthlyLimit(context.Background(), orgresource.SetMemberLimitExecution{OrganizationID: "B", MemberID: "member-B-operator", ActorID: "admin", OperationID: "point-limit", Target: 10000000})
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, f.owner.Exec("INSERT INTO saas_organization_resource_buckets (organization_id,resource_type,available,allocated,reserved,consumed,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)",
		"B", "ai_point", 10000000, 0, 0, 0, now, now).Error)
	const approvedGoal = "突出有证据支持的卖点"
	var plannerCalls, titleCalls atomic.Int32
	var titleGoalSeen atomic.Bool
	var invalidPlanner atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		body := json.NewDecoder(r.Body)
		require.NoError(t, body.Decode(&request))
		var content string
		if request.Model == "chat-fixture" {
			plannerCalls.Add(1)
			content = `{"mode":"READY","assistant_text":"已准备好标题优化方案，请确认后执行。","goal_summary":"` + approvedGoal + `"}`
			if invalidPlanner.Load() {
				content = "invalid planner JSON"
			}
		} else {
			var citationID string
			for _, message := range request.Messages {
				var text string
				if json.Unmarshal(message.Content, &text) == nil && strings.Contains(text, approvedGoal) {
					titleGoalSeen.Store(true)
				}
				var prompt struct{ Knowledge *knowledge.ContextBundle }
				if json.Unmarshal(message.Content, &text) == nil && json.Unmarshal([]byte(text), &prompt) == nil && prompt.Knowledge != nil {
					citationID = prompt.Knowledge.Entries[0].Citation.ID
				}
			}
			step := titleCalls.Add(1)
			switch step {
			case 1:
				content = `{"Kind":"tool","Tool":{"ID":"product.asset.inspect","Version":"v1.0.0"}}`
			case 2:
				content = `{"Kind":"tool","Tool":{"ID":"product.canonical.inspect","Version":"v2.0.0"}}`
			case 3:
				content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Reviewed bottle title","EvidenceIDs":["invented"]}]}}`
			default:
				content = `{"Kind":"propose","Candidate":{"Changes":[{"Field":"title","Value":"Reviewed bottle title","EvidenceIDs":["981645030344"]}]},"Confidence":[{"Field":"title","Value":0.8,"Known":true}]}`
			}
			if withKnowledge && step >= 3 {
				require.NotEmpty(t, citationID)
				content = strings.TrimSuffix(content, "}") + `,"ContextCitationIDs":["` + citationID + `"]}`
			}
		}
		encoded, _ := json.Marshal(content)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"isolated-workbench","choices":[{"message":{"content":` + string(encoded) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}`))
	}))
	defer provider.Close()
	credentials := openai.NewOrganizationOnlyCredentialResolver(f.owner)
	for _, row := range []openai.AIClientCredential{
		{TenantID: "B", ClientName: "title", APIKey: "isolated-fixture-key", BaseURL: provider.URL + "/v1", Model: "title-fixture", APIStyle: "openai", Enabled: true, TimeoutSecond: 3},
		{TenantID: "B", ClientName: "chat", APIKey: "isolated-fixture-key", BaseURL: provider.URL + "/v1", Model: "chat-fixture", APIStyle: "openai", Enabled: true, TimeoutSecond: 3},
	} {
		require.NoError(t, credentials.SaveCredential(context.Background(), row))
	}
	titleRow, err := credentials.GetCredential(context.Background(), "B", "", "title")
	require.NoError(t, err)
	chatRow, err := credentials.GetCredential(context.Background(), "B", "", "chat")
	require.NoError(t, err)
	profile := func(client, model, prompt, output string) aicapability.ModelProfile {
		return aicapability.ModelProfile{ClientName: client, ProviderID: "openai-compatible", AdapterKind: "openai-compatible", ModelID: model,
			RoutingPolicyVersion: "route-v1", AdapterPolicyVersion: "adapter-v1", PromptVersion: prompt, OutputSchemaVersion: output,
			UsageMappingVersion: "usage-v1", CostPricingVersion: "fixture-v1", PointTariff: aicapability.ModelPointTariff{PriceVersion: "synthetic-points-v1", InputPointsPerMillionTokens: 1000000, OutputPointsPerMillionTokens: 2000000},
			Currency: "CNY", InputMicrosPerMillion: 300000, OutputMicrosPerMillion: 2000000, MaximumPromptTokens: 1048576, MaximumCompletionTokens: 8192,
			MaximumInputBytes: 1 << 20, MaximumOutputBytes: 16 << 10, DeadlineBound: 3 * time.Second}
	}
	policy := func(row *openai.AIClientCredential, p aicapability.ModelProfile) governed.RoutePolicy {
		return governed.RoutePolicy{Profile: p, AdmittedCredentialVersion: governed.CredentialVersion(*row), AdmittedEndpointIdentityDigest: governed.EndpointIdentityDigest(row.BaseURL)}
	}
	ledger := aistore.NewGormInvocationRecorder(f.owner)
	claimLedger := &rollbackWorkbenchClaim{ProductAgentInvocationLedger: ledger}
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	taskGrants := &taskViewerGrants{base: f.grants}
	deps.organizationResolver = workbenchcontext.NewResolver(taskGrants, "project", "v1", nil)
	auth, err := authz.NewListingKitAuthorizer([]string{"operator"}, nil)
	require.NoError(t, err)
	deps.authorizer = auth
	settings := ProductAgentDependencies{Enabled: true, AllowedOrganizationIDs: []string{"B"}, RunDB: f.owner, PointAccountingDB: f.owner, AssetDB: f.owner,
		ReviewDB: f.owner, Ledger: claimLedger, TextPolicies: map[string]governed.RoutePolicy{"B": policy(titleRow, profile("title", "title-fixture", "product-title-agent-v1", "product-title-action-v1"))},
		Limits: agent.Limits{Steps: 12, ModelCalls: 6, Tokens: 5000000, CostMicros: 5000000, Currency: "CNY", Runtime: time.Minute}}
	if kf != nil {
		settings.Knowledge = kf.service
	}
	agentModule, err := buildProductAgentModule(context.Background(), f.db, deps, auth, settings, nil)
	require.NoError(t, err)
	readinessIdentity := authidentity.AuthenticatedIdentity{TenantID: "B", EffectiveOrganizationID: "B", UserID: "operator", TokenExpiresAt: time.Now().Add(time.Hour)}
	readinessCtx := authidentity.WithAuthenticatedIdentity(context.Background(), readinessIdentity)
	require.EqualValues(t, "AVAILABLE", agentModule.(productAgentModule).application.model.RouteReadinessForVerifiedOrganization(readinessCtx, "B"))
	readinessIdentity.TenantID, readinessIdentity.EffectiveOrganizationID = "A", "A"
	readinessCtx = authidentity.WithAuthenticatedIdentity(context.Background(), readinessIdentity)
	require.EqualValues(t, "UNAVAILABLE", agentModule.(productAgentModule).application.model.RouteReadinessForVerifiedOrganization(readinessCtx, "A"))
	workbenchModule, err := buildAIWorkbenchModule(context.Background(), AIWorkbenchDependencies{DB: workbenchDB,
		PlanningTextPolicies: map[string]governed.RoutePolicy{"B": policy(chatRow, profile("chat", "chat-fixture", "ai-workbench-chat-plan-v1", "ai-workbench-plan-decision-v1"))}}, agentModule.(productAgentModule).application)
	require.NoError(t, err)
	plannerText, ok := workbenchModule.(aiWorkbenchModule).application.plan.model.Text.(*titleGatedPlanningText)
	require.True(t, ok)
	require.Same(t, agentModule.(productAgentModule).application.textAdmission, plannerText.executor.Admission,
		"Chat and title must share provider admission within the application")
	require.True(t, workbenchModule.(aiWorkbenchModule).AdmittedOrganization("B"))
	require.False(t, workbenchModule.(aiWorkbenchModule).AdmittedOrganization("A"))
	require.Equal(t, "AVAILABLE", workbenchModule.(aiWorkbenchModule).PlanningReadiness(context.Background(), "B"))
	require.Equal(t, "UNAVAILABLE", workbenchModule.(aiWorkbenchModule).PlanningReadiness(context.Background(), "A"))
	routes := append(agentModule.(productAgentModule).routes, workbenchModule.(aiWorkbenchModule).routes...)
	server := httptest.NewServer(buildIsolatedApplicationHTTPServer(routes, deps, 2*time.Minute).Handler)
	defer server.Close()

	code, raw, err := acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var created struct {
		Conversation struct {
			ID string `json:"ID"`
		} `json:"conversation"`
	}
	require.NoError(t, json.Unmarshal(raw, &created))
	require.NotEmpty(t, created.Conversation.ID)
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{"favorite":true}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var saved struct {
		Conversation struct {
			ID string `json:"ID"`
		} `json:"conversation"`
	}
	require.NoError(t, json.Unmarshal(raw, &saved))
	for range 2 {
		code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
		require.NoError(t, err)
		require.Equal(t, 200, code, string(raw))
	}
	code, raw, err = acquisitionHTTPRequest(server, "GET", workbenchChatBase+"?limit=1&saved=true", "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var savedPage struct {
		Conversations []struct {
			ID string `json:"ID"`
		} `json:"conversations"`
		Next string `json:"next"`
	}
	require.NoError(t, json.Unmarshal(raw, &savedPage))
	require.Len(t, savedPage.Conversations, 1)
	require.Equal(t, saved.Conversation.ID, savedPage.Conversations[0].ID, "favorite filter runs before limit")
	require.Empty(t, savedPage.Next)
	messageKey := uuid.NewString()
	messageBody := `{"content":"请优化这个商品在 SHEIN 的标题","operationId":"` + op.OperationID + `","targetPlatform":"shein"}`
	if kf != nil {
		messageBody = strings.TrimSuffix(messageBody, "}") + `,"knowledgeBaseId":"` + kf.base.ID + `"}`
	}
	messagePath := workbenchChatBase + "/" + created.Conversation.ID + "/messages"
	claimLedger.rollback.Store(true)
	code, raw, err = acquisitionHTTPRequest(server, "POST", messagePath, "operator", "B", messageKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 202, code, string(raw))
	require.Contains(t, string(raw), `"state":"READY_TO_DISPATCH"`)
	require.Zero(t, plannerCalls.Load(), "a rolled-back claim cannot dispatch")
	frozenCommand, err := workbenchModule.(aiWorkbenchModule).application.store.GetCommand(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, messageKey)
	require.NoError(t, err)
	_, err = ledger.ReadModelInvocation(context.Background(), "B", frozenCommand.PlannerInvocationID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	code, raw, err = acquisitionHTTPRequest(server, "POST", messagePath, "operator", "B", messageKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var planned struct {
		State      string `json:"state"`
		ProposalID string `json:"proposalId"`
	}
	require.NoError(t, json.Unmarshal(raw, &planned))
	require.Equal(t, "COMPLETE", planned.State)
	completedCommand, err := workbenchModule.(aiWorkbenchModule).application.store.GetCommand(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, messageKey)
	require.NoError(t, err)
	require.Equal(t, frozenCommand.PlannerInvocationID, completedCommand.PlannerInvocationID)
	require.Equal(t, frozenCommand.InputHash, completedCommand.InputHash)
	require.Equal(t, frozenCommand.ModelProfile, completedCommand.ModelProfile)
	require.Equal(t, frozenCommand.Deadline, completedCommand.Deadline)
	require.NotEmpty(t, planned.ProposalID)
	require.EqualValues(t, 1, plannerCalls.Load())
	require.Zero(t, titleCalls.Load())
	var proposalView struct {
		Proposals []struct {
			ID                  string `json:"id"`
			TitleProfileReady   bool   `json:"titleProfileReady"`
			ExecutionAuthorized bool   `json:"executionAuthorized"`
		} `json:"proposals"`
	}
	code, raw, err = acquisitionHTTPRequest(server, "GET", workbenchChatBase+"/"+created.Conversation.ID, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &proposalView))
	require.Len(t, proposalView.Proposals, 1)
	require.True(t, proposalView.Proposals[0].TitleProfileReady, "the frozen title profile is currently admitted")
	require.True(t, proposalView.Proposals[0].ExecutionAuthorized)
	taskGrants.viewer.Store(true)
	code, raw, err = acquisitionHTTPRequest(server, "GET", workbenchChatBase+"/"+created.Conversation.ID, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &proposalView))
	require.True(t, proposalView.Proposals[0].TitleProfileReady)
	require.False(t, proposalView.Proposals[0].ExecutionAuthorized, "a configured global user grant cannot replace the effective organization's execution role")
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase+"/"+created.Conversation.ID+"/proposals/"+planned.ProposalID+"/confirm",
		"operator", "B", uuid.NewString(), "")
	require.NoError(t, err)
	require.Equal(t, 403, code, string(raw))
	require.Zero(t, titleCalls.Load(), "the global user grant does not authorize title dispatch")
	taskGrants.viewer.Store(false)
	code, raw, err = acquisitionHTTPRequest(server, "GET", workbenchChatBase+"?limit=1", "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var recentPage struct {
		Conversations []struct {
			ID string `json:"ID"`
		} `json:"conversations"`
	}
	require.NoError(t, json.Unmarshal(raw, &recentPage))
	require.Len(t, recentPage.Conversations, 1)
	require.Equal(t, created.Conversation.ID, recentPage.Conversations[0].ID, "message activity moves an older conversation to recent")
	var count int64
	require.NoError(t, f.owner.Table("ai_workbench.business_tasks").Count(&count).Error)
	require.Zero(t, count)
	var consumedBefore int64
	require.NoError(t, f.owner.Table("saas_organization_resource_buckets").Where("organization_id = ? AND resource_type = ?", "B", "ai_point").
		Pluck("consumed", &consumedBefore).Error)
	// A metered, strictly invalid planning result is terminal without an
	// invented assistant or a second model send on the same command key.
	taskGrants.viewer.Store(false)
	invalidPlanner.Store(true)
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var invalidConversation struct {
		Conversation struct {
			ID string `json:"ID"`
		} `json:"conversation"`
	}
	require.NoError(t, json.Unmarshal(raw, &invalidConversation))
	invalidKey := uuid.NewString()
	invalidPath := workbenchChatBase + "/" + invalidConversation.Conversation.ID + "/messages"
	code, raw, err = acquisitionHTTPRequest(server, "POST", invalidPath, "operator", "B", invalidKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var invalidReceipt struct {
		State string `json:"state"`
	}
	require.NoError(t, json.Unmarshal(raw, &invalidReceipt))
	require.Equal(t, "PLANNER_INVALID_OUTPUT", invalidReceipt.State)
	command, err := workbenchModule.(aiWorkbenchModule).application.store.GetCommand(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, invalidKey)
	require.NoError(t, err)
	fact, err := ledger.ReadModelInvocation(context.Background(), "B", command.PlannerInvocationID)
	require.NoError(t, err)
	require.Equal(t, aicapability.InvocationUsageObservedFailed, fact.Outcome)
	require.Equal(t, aicapability.ErrorStructuredOutputInvalid, fact.ErrorCategory)
	require.True(t, fact.UsageKnown)
	var consumedAfterInvalid int64
	require.NoError(t, f.owner.Table("saas_organization_resource_buckets").Where("organization_id = ? AND resource_type = ?", "B", "ai_point").
		Pluck("consumed", &consumedAfterInvalid).Error)
	require.Greater(t, consumedAfterInvalid, consumedBefore, "invalid observed output must settle its AI points")
	require.EqualValues(t, 2, plannerCalls.Load())
	code, raw, err = acquisitionHTTPRequest(server, "POST", invalidPath, "operator", "B", invalidKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &invalidReceipt))
	require.Equal(t, "PLANNER_INVALID_OUTPUT", invalidReceipt.State)
	require.EqualValues(t, 2, plannerCalls.Load(), "terminal invalid planning must not redispatch")
	messages, err := workbenchModule.(aiWorkbenchModule).application.store.ListMessages(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, invalidConversation.Conversation.ID, 50)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, aiworkbench.AuthorUser, messages[0].Author)

	// The AI ledger writes observed usage before ResourceAIPoint settlement.
	// A failed settlement must never be projected as the settled terminal state.
	pointOwner := agentModule.(productAgentModule).application.points
	ledger.SetUsageSettler(failedWorkbenchSettlement{InvocationUsageSettler: pointOwner, InvocationUsageReservation: pointOwner})
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &invalidConversation))
	unsettledKey := uuid.NewString()
	unsettledPath := workbenchChatBase + "/" + invalidConversation.Conversation.ID + "/messages"
	code, raw, err = acquisitionHTTPRequest(server, "POST", unsettledPath, "operator", "B", unsettledKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 202, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &invalidReceipt))
	require.Equal(t, "READY_TO_DISPATCH", invalidReceipt.State)
	unsettled, err := workbenchModule.(aiWorkbenchModule).application.store.GetCommand(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, unsettledKey)
	require.NoError(t, err)
	fact, err = ledger.ReadModelInvocation(context.Background(), "B", unsettled.PlannerInvocationID)
	require.NoError(t, err)
	require.Equal(t, aicapability.InvocationUsageObservedFailed, fact.Outcome)
	var consumedAfterFailure int64
	require.NoError(t, f.owner.Table("saas_organization_resource_buckets").Where("organization_id = ? AND resource_type = ?", "B", "ai_point").
		Pluck("consumed", &consumedAfterFailure).Error)
	require.Equal(t, consumedAfterInvalid, consumedAfterFailure, "failed settlement must not claim consumed AI points")
	code, raw, err = acquisitionHTTPRequest(server, "POST", unsettledPath, "operator", "B", unsettledKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 202, code, string(raw))
	require.EqualValues(t, 3, plannerCalls.Load(), "unsettled replay cannot resend or claim settlement")
	ledger.SetUsageSettler(pointOwner)

	// A known-zero AI terminal row can exist before the ResourceAIPoint owner
	// releases its reservation. That row alone must not clear the Chat receipt.
	invalidPlanner.Store(false)
	ledger.SetUsageSettler(failedWorkbenchRelease{InvocationUsageSettler: pointOwner, InvocationUsageReservation: pointOwner})
	originalAuthorize := plannerText.executor.Authorize
	var authorizationCalls atomic.Int32
	plannerText.executor.Authorize = func(ctx context.Context, input aicapability.TextInputIdentity) error {
		if authorizationCalls.Add(1) == 2 {
			return errors.New("synthetic pre-send authorization change")
		}
		return originalAuthorize(ctx, input)
	}
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &invalidConversation))
	unreleasedKey := uuid.NewString()
	unreleasedPath := workbenchChatBase + "/" + invalidConversation.Conversation.ID + "/messages"
	noSendBefore := plannerCalls.Load()
	code, raw, err = acquisitionHTTPRequest(server, "POST", unreleasedPath, "operator", "B", unreleasedKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 202, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &invalidReceipt))
	require.Equal(t, "READY_TO_DISPATCH", invalidReceipt.State)
	require.Equal(t, noSendBefore, plannerCalls.Load(), "the provider must not receive a request")
	unreleased, err := workbenchModule.(aiWorkbenchModule).application.store.GetCommand(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, unreleasedKey)
	require.NoError(t, err)
	unreleasedFact, err := ledger.ReadModelInvocation(context.Background(), "B", unreleased.PlannerInvocationID)
	require.NoError(t, err)
	require.Equal(t, aicapability.InvocationFailed, unreleasedFact.Outcome)
	require.True(t, unreleasedFact.UsageKnown)
	require.Zero(t, unreleasedFact.TotalTokens)
	var reservationState string
	require.NoError(t, f.owner.Table("saas_organization_resource_reservations").Where("organization_id = ? AND owner_attempt_id = ?", "B", unreleased.PlannerInvocationID).
		Pluck("state", &reservationState).Error)
	require.Equal(t, "reserved", reservationState, "failed ResourceAIPoint release must remain visible to its owner")
	plannerText.executor.Authorize = originalAuthorize
	code, raw, err = acquisitionHTTPRequest(server, "POST", unreleasedPath, "operator", "B", unreleasedKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 202, code, string(raw))
	require.Equal(t, noSendBefore, plannerCalls.Load(), "same-key replay cannot trigger another provider request")
	ledger.SetUsageSettler(pointOwner)

	// A lost response must replay the frozen receipt even when the planner
	// route becomes unavailable before the retry. No mutable route read wins.
	changedRoute := f.owner.Model(&openai.AIClientCredential{}).
		Where("tenant_id = ? AND user_id = ? AND client_name = ?", "B", "", "chat").
		Update("enabled", false)
	require.NoError(t, changedRoute.Error)
	require.EqualValues(t, 1, changedRoute.RowsAffected)
	code, raw, err = acquisitionHTTPRequest(server, "POST", messagePath, "operator", "B", messageKey, messageBody)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.EqualValues(t, 3, plannerCalls.Load())
	require.NoError(t, f.owner.Model(&openai.AIClientCredential{}).
		Where("tenant_id = ? AND user_id = ? AND client_name = ?", "B", "", "chat").
		Update("enabled", true).Error)
	confirmKey := uuid.NewString()
	confirmPath := workbenchChatBase + "/" + created.Conversation.ID + "/proposals/" + planned.ProposalID + "/confirm"
	code, raw, err = acquisitionHTTPRequest(server, "POST", confirmPath, "operator", "B", confirmKey, "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var confirmed struct {
		Task struct {
			ID       string `json:"id"`
			State    string `json:"state"`
			ReviewID string `json:"reviewId"`
		} `json:"task"`
		Replay bool `json:"replay"`
	}
	require.NoError(t, json.Unmarshal(raw, &confirmed))
	require.NotEmpty(t, confirmed.Task.ID)
	require.False(t, confirmed.Replay)
	require.EqualValues(t, 4, titleCalls.Load(), "confirmation starts the existing Product Agent exactly once")
	require.True(t, titleGoalSeen.Load(), "the confirmed Chat goal must reach the actual title model input")
	require.Equal(t, "WAITING_CONFIRMATION", confirmed.Task.State)
	if kf != nil {
		readKnowledge := func(status string) {
			t.Helper()
			code, raw, err := acquisitionHTTPRequest(server, "GET", workbenchTaskBase+"/"+confirmed.Task.ID, "operator", "B", "", "")
			require.NoError(t, err)
			require.Equal(t, 200, code, string(raw))
			var response struct {
				Task struct {
					Knowledge *productKnowledgeDTO `json:"knowledge"`
					State     string               `json:"state"`
				} `json:"task"`
			}
			require.NoError(t, json.Unmarshal(raw, &response))
			require.NotNil(t, response.Task.Knowledge)
			require.Equal(t, status, response.Task.Knowledge.Status)
			require.Equal(t, "WAITING_CONFIRMATION", response.Task.State)
			if status == "available" {
				require.Len(t, response.Task.Knowledge.Citations, 1)
				require.Equal(t, "Frozen brand text", response.Task.Knowledge.Citations[0].Excerpt)
				require.Equal(t, kf.source.CurrentReadableRevision.ID, response.Task.Knowledge.Citations[0].RevisionID)
			} else {
				require.Empty(t, response.Task.Knowledge.Citations)
				require.NotContains(t, string(raw), "Frozen brand text")
				require.NotContains(t, string(raw), "Brand wording")
			}
		}
		readKnowledge("available")
		_, err := kf.service.Mutate(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"},
			Kind: "source_disable", SourceID: kf.source.ID, Version: kf.source.Version, Key: uuid.NewString()})
		require.NoError(t, err)
		readKnowledge("unavailable")
		require.EqualValues(t, 4, titleCalls.Load(), "fresh protected projection never dispatches")
		return
	}
	// The durable Agent run must be adopted by its frozen request even when
	// the current acquisition binding cannot be read during reconciliation.
	agentApplication := agentModule.(productAgentModule).application
	publishedReader := agentApplication.receipts
	agentApplication.receipts = unavailablePublishedReceipt{}
	startKey := uuid.NewString()
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchTaskBase+"/"+confirmed.Task.ID+"/start", "operator", "B", startKey, "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.EqualValues(t, 4, titleCalls.Load(), "reconciliation never resends the model")
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchTaskBase+"/"+confirmed.Task.ID+"/start", "operator", "B", startKey, "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"replay":true`)
	require.EqualValues(t, 4, titleCalls.Load(), "same-key Start replay never re-enters the Agent owner")
	var unavailableProduct struct {
		Task struct {
			ProductDetailsAvailable bool `json:"productDetailsAvailable"`
			CanStart                bool `json:"canStart"`
			CanResume               bool `json:"canResume"`
			CanReview               bool `json:"canReview"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal(raw, &unavailableProduct))
	require.False(t, unavailableProduct.Task.ProductDetailsAvailable)
	require.False(t, unavailableProduct.Task.CanStart)
	require.False(t, unavailableProduct.Task.CanResume)
	require.False(t, unavailableProduct.Task.CanReview, "review requires the current product binding")
	agentApplication.receipts = publishedReader
	taskGrants.viewer.Store(true)
	readAsViewer := func(wantState string) {
		t.Helper()
		status, body, readErr := acquisitionHTTPRequest(server, "GET", workbenchTaskBase+"/"+confirmed.Task.ID, "operator", "B", "", "")
		require.NoError(t, readErr)
		require.Equal(t, 200, status, string(body))
		var viewer struct {
			Task struct {
				State               string `json:"state"`
				ProjectionAvailable bool   `json:"projectionAvailable"`
				ReviewID            string `json:"reviewId"`
				ReviewState         string `json:"reviewState"`
				CanStart            bool   `json:"canStart"`
				CanReview           bool   `json:"canReview"`
			} `json:"task"`
		}
		require.NoError(t, json.Unmarshal(body, &viewer))
		require.True(t, viewer.Task.ProjectionAvailable)
		require.Equal(t, wantState, viewer.Task.State)
		require.Empty(t, viewer.Task.ReviewID, "Review owner details remain protected")
		require.Empty(t, viewer.Task.ReviewState, "Review owner details remain protected")
		require.False(t, viewer.Task.CanStart)
		require.False(t, viewer.Task.CanReview)
	}
	readAsViewer("WAITING_CONFIRMATION")
	taskGrants.viewer.Store(false)
	require.NoError(t, f.owner.Table("ai_workbench.business_tasks").Count(&count).Error)
	require.EqualValues(t, 1, count)
	titleBefore := titleCalls.Load()
	code, raw, err = acquisitionHTTPRequest(server, "POST", confirmPath, "operator", "B", confirmKey, "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var replayed struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
		Replay bool `json:"replay"`
	}
	require.NoError(t, json.Unmarshal(raw, &replayed))
	require.True(t, replayed.Replay)
	require.Equal(t, confirmed.Task.ID, replayed.Task.ID)
	require.Equal(t, titleBefore, titleCalls.Load())
	staleResumeKey := uuid.NewString()
	for attempt := 0; attempt < 2; attempt++ {
		code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchTaskBase+"/"+confirmed.Task.ID+"/resume",
			"operator", "B", staleResumeKey, `{"revision":"999999","feedback":"stale feedback"}`)
		require.NoError(t, err)
		require.Equal(t, 409, code, string(raw))
		require.JSONEq(t, `{"code":"REVISION_MISMATCH"}`, string(raw))
	}
	require.Equal(t, titleBefore, titleCalls.Load(), "stale Task action and its receipt replay never dispatch a model")
	// A fresh Review authorization rejection happens before its transaction.
	// The Task receipt must report a known failure, including after access is restored.
	normalReviews := agentApplication.reviews
	reviewReader, err := catalogstore.NewBoundedSnapshotReader(f.db, 1<<20)
	require.NoError(t, err)
	reviewSources, err := productsourcing.NewInternalProducer(f.db,
		&productReviewLiveOrganizationAccess{resolver: agentApplication.resolver, now: time.Now}, auth)
	require.NoError(t, err)
	reviewRepo, err := reviewstore.NewRepository(f.db, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	require.NoError(t, err)
	agentApplication.reviews, err = review.NewCandidateService(reviewReader, reviewSources, reviewRepo, deniedCandidateReviewAuthorizer{auth})
	require.NoError(t, err)
	deniedReviewKey := uuid.NewString()
	var reviewsBefore int64
	require.NoError(t, f.owner.Table("product_title_proposals").Count(&reviewsBefore).Error)
	for attempt := 0; attempt < 2; attempt++ {
		code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchTaskBase+"/"+confirmed.Task.ID+"/review",
			"operator", "B", deniedReviewKey, "")
		require.NoError(t, err)
		require.Equal(t, 503, code, string(raw))
		require.JSONEq(t, `{"code":"DEPENDENCY_UNAVAILABLE"}`, string(raw))
		agentApplication.reviews = normalReviews
	}
	var reviewsAfter int64
	require.NoError(t, f.owner.Table("product_title_proposals").Count(&reviewsAfter).Error)
	require.Equal(t, reviewsBefore, reviewsAfter, "pre-transaction rejection and replay do not create a Review")
	require.Equal(t, titleBefore, titleCalls.Load())
	reviewKey := uuid.NewString()
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchTaskBase+"/"+confirmed.Task.ID+"/review", "operator", "B", reviewKey, "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var reviewed struct {
		Task struct {
			ReviewID string `json:"reviewId"`
			State    string `json:"state"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal(raw, &reviewed))
	require.NotEmpty(t, reviewed.Task.ReviewID)
	require.Equal(t, "WAITING_CONFIRMATION", reviewed.Task.State)
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchTaskBase+"/"+confirmed.Task.ID+"/review", "operator", "B", reviewKey, "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"replay":true`)
	require.Contains(t, string(raw), reviewed.Task.ReviewID)
	for _, suffix := range []string{"start", "review", "resume"} {
		body := ""
		if suffix == "resume" {
			body = `{"revision":"2","feedback":"same task"}`
		}
		status, response, callErr := acquisitionHTTPRequest(server, "POST", workbenchTaskBase+"/"+confirmed.Task.ID+"/"+suffix, "operator", "B", "", body)
		require.NoError(t, callErr)
		require.Equal(t, 400, status, string(response))
	}
	proposal := titleCall(t, server, "GET", titleBasePath+"/"+reviewed.Task.ReviewID, "admin", "B", "", "", 200)
	require.Equal(t, "pending", proposal.State)
	accepted := titleDecision(t, server, proposal, "accept", "admin", "", 200)
	applied := titleApply(t, server, accepted, uuid.NewString(), 200)
	require.Equal(t, "applied", applied.State)
	code, raw, err = acquisitionHTTPRequest(server, "GET", workbenchTaskBase+"/"+confirmed.Task.ID, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var finished struct {
		Task struct {
			State string `json:"state"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal(raw, &finished))
	require.Equal(t, "COMPLETED", finished.Task.State)
	taskGrants.viewer.Store(true)
	readAsViewer("COMPLETED")
	require.Equal(t, titleBefore, titleCalls.Load())
	currentChat, err := credentials.GetCredential(context.Background(), "B", "", "chat")
	require.NoError(t, err)
	readyModule, err := buildAIWorkbenchModule(context.Background(), AIWorkbenchDependencies{DB: workbenchDB,
		PlanningTextPolicies: map[string]governed.RoutePolicy{"B": policy(currentChat, profile("chat", "chat-fixture", "ai-workbench-chat-plan-v1", "ai-workbench-plan-decision-v1"))}}, agentModule.(productAgentModule).application)
	require.NoError(t, err)
	require.Equal(t, "AVAILABLE", readyModule.(aiWorkbenchModule).PlanningReadiness(context.Background(), "B"))
	readyRoutes := append(agentModule.(productAgentModule).routes, readyModule.(aiWorkbenchModule).routes...)
	readyServer := httptest.NewServer(buildIsolatedApplicationHTTPServer(readyRoutes, deps, 2*time.Minute).Handler)
	defer readyServer.Close()

	// Persist T1 without dispatching T2, then change the credential before
	// the explicit Task start. The action and its affordance must agree.
	taskGrants.viewer.Store(false)
	invalidPlanner.Store(false)
	code, raw, err = acquisitionHTTPRequest(readyServer, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var secondConversation struct {
		Conversation struct {
			ID string `json:"ID"`
		} `json:"conversation"`
	}
	require.NoError(t, json.Unmarshal(raw, &secondConversation))
	code, raw, err = acquisitionHTTPRequest(readyServer, "POST", workbenchChatBase+"/"+secondConversation.Conversation.ID+"/messages", "operator", "B", uuid.NewString(), messageBody)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var secondPlan struct {
		ProposalID string `json:"proposalId"`
	}
	require.NoError(t, json.Unmarshal(raw, &secondPlan))
	require.NotEmpty(t, secondPlan.ProposalID, string(raw))
	pendingProposal, err := readyModule.(aiWorkbenchModule).application.store.GetProposal(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, secondPlan.ProposalID)
	require.NoError(t, err)
	identity := authidentity.AuthenticatedIdentity{TenantID: "B", EffectiveOrganizationID: "B", HomeOrganizationID: "B",
		UserID: "operator", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)}
	preparedCtx, err := (productReviewCapabilityBinder{}).Bind(authidentity.WithAuthenticatedIdentity(context.Background(), identity), "Bearer isolated-test-token")
	require.NoError(t, err)
	pendingKey := uuid.NewString()
	prepared, err := (workbenchExecution{agent: agentModule.(productAgentModule).application}).Prepare(preparedCtx, pendingProposal, pendingKey)
	require.NoError(t, err)
	pendingTask, replay, err := readyModule.(aiWorkbenchModule).application.store.Confirm(preparedCtx,
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, secondConversation.Conversation.ID, secondPlan.ProposalID, pendingKey, prepared)
	require.NoError(t, err)
	require.False(t, replay)
	pendingPath := workbenchTaskBase + "/" + pendingTask.ID
	code, raw, err = acquisitionHTTPRequest(readyServer, "GET", pendingPath, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"canStart":true`)

	// A still-ready planner cannot advertise or charge for a path whose title
	// route has since been disabled. The same-key receipt remains replayable.
	taskGrants.viewer.Store(false)
	disabledTitle := *titleRow
	disabledTitle.Enabled = false
	require.NoError(t, credentials.SaveCredential(context.Background(), disabledTitle))
	code, raw, err = acquisitionHTTPRequest(readyServer, "GET", pendingPath, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"canStart":false`)
	disabledStartKey := uuid.NewString()
	for attempt := 0; attempt < 2; attempt++ {
		code, raw, err = acquisitionHTTPRequest(readyServer, "POST", pendingPath+"/start", "operator", "B", disabledStartKey, "")
		require.NoError(t, err)
		require.Equal(t, 503, code, string(raw))
		require.JSONEq(t, `{"code":"DEPENDENCY_UNAVAILABLE"}`, string(raw), "known pre-owner failure must not become unknown")
	}
	var pendingRequest agent.Request
	require.NoError(t, json.Unmarshal(pendingTask.ExecutionRequest, &pendingRequest))
	_, foundPendingRun, err := agentModule.(productAgentModule).application.store.Lookup(context.Background(),
		agent.Scope{OrganizationID: "B", ActorID: "operator"}, pendingRequest.Binding, pendingTask.ExecutionRequestKey)
	require.NoError(t, err)
	require.False(t, foundPendingRun, "disabled title route must not claim the frozen run")
	require.Equal(t, "NEEDS_CONFIGURATION", readyModule.(aiWorkbenchModule).PlanningReadiness(context.Background(), "B"))
	code, raw, err = acquisitionHTTPRequest(readyServer, "GET", workbenchChatBase+"/"+created.Conversation.ID, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &proposalView))
	require.False(t, proposalView.Proposals[0].TitleProfileReady, "a disabled title route cannot confirm an old proposal")
	plannerBefore := plannerCalls.Load()
	code, raw, err = acquisitionHTTPRequest(readyServer, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var unavailableConversation struct {
		Conversation struct {
			ID string `json:"ID"`
		} `json:"conversation"`
	}
	require.NoError(t, json.Unmarshal(raw, &unavailableConversation))
	key := uuid.NewString()
	path := workbenchChatBase + "/" + unavailableConversation.Conversation.ID + "/messages"
	body := `{"content":"请优化标题","operationId":"` + op.OperationID + `","targetPlatform":"shein"}`
	for range 2 {
		code, raw, err = acquisitionHTTPRequest(readyServer, "POST", path, "operator", "B", key, body)
		require.NoError(t, err)
		require.Equal(t, 200, code, string(raw))
		require.Contains(t, string(raw), `"state":"FAILED_BEFORE_DISPATCH"`)
	}
	require.Equal(t, plannerBefore, plannerCalls.Load(), "unready title route must not cause a paid planner send")

	// Rotation after the first route check but during the planner reservation
	// must still be rejected by the final guarded transport handoff.
	invalidPlanner.Store(false)
	disabledTitle.Enabled = true
	require.NoError(t, credentials.SaveCredential(context.Background(), disabledTitle))
	code, raw, err = acquisitionHTTPRequest(readyServer, "GET", pendingPath, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"canStart":false`, "credential rotation cannot revive a frozen Task")
	currentTitle, err := credentials.GetCredential(context.Background(), "B", "", "title")
	require.NoError(t, err)
	var rotated atomic.Bool
	rotatingSettings := settings
	rotatingSettings.TextPolicies = map[string]governed.RoutePolicy{"B": policy(currentTitle, profile("title", "title-fixture", "product-title-agent-v1", "product-title-action-v1"))}
	rotatingSettings.Ledger = agentReserveHook{ProductAgentInvocationLedger: ledger, afterReserve: func() {
		result := f.owner.Model(&openai.AIClientCredential{}).
			Where("tenant_id = ? AND user_id = ? AND client_name = ?", "B", "", "title").Update("enabled", false)
		if result.Error != nil || result.RowsAffected != 1 {
			t.Errorf("synthetic title rotation failed: %v rows=%d", result.Error, result.RowsAffected)
		}
		rotated.Store(true)
	}}
	rotatingAgent, err := buildProductAgentModule(context.Background(), f.db, deps, auth, rotatingSettings, nil)
	require.NoError(t, err)
	rotatingWorkbench, err := buildAIWorkbenchModule(context.Background(), AIWorkbenchDependencies{DB: workbenchDB,
		PlanningTextPolicies: map[string]governed.RoutePolicy{"B": policy(currentChat, profile("chat", "chat-fixture", "ai-workbench-chat-plan-v1", "ai-workbench-plan-decision-v1"))}}, rotatingAgent.(productAgentModule).application)
	require.NoError(t, err)
	rotatingRoutes := append(rotatingAgent.(productAgentModule).routes, rotatingWorkbench.(aiWorkbenchModule).routes...)
	rotatingServer := httptest.NewServer(buildIsolatedApplicationHTTPServer(rotatingRoutes, deps, 2*time.Minute).Handler)
	defer rotatingServer.Close()
	require.Equal(t, "AVAILABLE", rotatingWorkbench.(aiWorkbenchModule).PlanningReadiness(context.Background(), "B"))
	code, raw, err = acquisitionHTTPRequest(rotatingServer, "GET", workbenchChatBase+"/"+created.Conversation.ID, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &proposalView))
	require.False(t, proposalView.Proposals[0].TitleProfileReady, "a newly admitted route cannot confirm a proposal frozen to the old profile")
	var consumedBeforeRotation int64
	require.NoError(t, f.owner.Table("saas_organization_resource_buckets").Where("organization_id = ? AND resource_type = ?", "B", "ai_point").
		Pluck("consumed", &consumedBeforeRotation).Error)
	code, raw, err = acquisitionHTTPRequest(rotatingServer, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &unavailableConversation))
	rotationKey := uuid.NewString()
	code, raw, err = acquisitionHTTPRequest(rotatingServer, "POST", workbenchChatBase+"/"+unavailableConversation.Conversation.ID+"/messages", "operator", "B", rotationKey, body)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.True(t, rotated.Load(), "title route must change after reservation and before transport handoff")
	require.Contains(t, string(raw), `"state":"FAILED_BEFORE_DISPATCH"`)
	require.Equal(t, plannerBefore, plannerCalls.Load(), "rotation at final handoff cannot issue a paid planner request")
	rotationReceipt, err := rotatingWorkbench.(aiWorkbenchModule).application.store.GetCommand(context.Background(),
		aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}, rotationKey)
	require.NoError(t, err)
	rotationFact, err := ledger.ReadModelInvocation(context.Background(), "B", rotationReceipt.PlannerInvocationID)
	require.NoError(t, err)
	require.Equal(t, aicapability.InvocationFailed, rotationFact.Outcome)
	require.True(t, rotationFact.UsageKnown)
	require.Zero(t, rotationFact.TotalTokens)
	var consumedAfterRotation int64
	require.NoError(t, f.owner.Table("saas_organization_resource_buckets").Where("organization_id = ? AND resource_type = ?", "B", "ai_point").
		Pluck("consumed", &consumedAfterRotation).Error)
	require.Equal(t, consumedBeforeRotation, consumedAfterRotation, "no-send rotation must not consume AI points")
}
