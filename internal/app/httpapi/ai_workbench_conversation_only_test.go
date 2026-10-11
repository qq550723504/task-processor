package httpapi

import (
	"context"
	"encoding/json"
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
	"task-processor/internal/aiworkbench"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	workstore "task-processor/internal/integration/persistence/aiworkbench"
	"task-processor/internal/workbenchcontext"
)

type conversationOnlyGrants struct {
	base   *titleGrants
	viewer atomic.Bool
}

func (g *conversationOnlyGrants) Load(ctx context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	result, err := g.base.Load(ctx, source, request)
	for i := range result.Grants {
		result.Grants[i].Roles = []string{authz.EnterpriseRoleKey(result.Grants[i].OrganizationID, 1)}
		if request.Subject == "viewer" || g.viewer.Load() {
			result.Grants[i].Roles = []string{authz.EnterpriseRoleKey(result.Grants[i].OrganizationID, 2)}
		}
	}
	return result, err
}
func (g *conversationOnlyGrants) Invalidate(actor, project string) { g.base.Invalidate(actor, project) }
func (*conversationOnlyGrants) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, key := range keys {
		if key == authz.EnterpriseRoleKey(org, 1) {
			result[key] = []string{"chat", "agents"}
		}
	}
	return result, nil
}

func TestAIWorkbenchConversationOnlyScopedCRUDAndNoExecution(t *testing.T) {
	f := newAcquisitionHTTPFixture(t)
	require.NoError(t, workstore.InstallSchema(f.owner))
	require.NoError(t, f.owner.Exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='ai_workbench_runtime') THEN CREATE ROLE ai_workbench_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF; END $$`).Error)
	require.NoError(t, f.owner.Exec("ALTER ROLE ai_workbench_runtime PASSWORD 'issue588-only'").Error)
	require.NoError(t, workstore.GrantRuntime(f.owner, "ai_workbench_runtime"))
	var name string
	require.NoError(t, f.owner.Raw("SELECT current_database()").Scan(&name).Error)
	require.NoError(t, f.owner.Exec("GRANT CONNECT ON DATABASE "+name+" TO ai_workbench_runtime").Error)
	db, err := gorm.Open(postgres.Open(os.Getenv("ISSUE398_TEST_DSN")+" dbname="+name+" user=ai_workbench_runtime password=issue588-only"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	grants := &conversationOnlyGrants{base: f.grants}
	deps.organizationResolver = workbenchcontext.NewResolver(grants, "project", "v1", nil)
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	auth.SetRolePolicyReader(grants)
	deps.authorizer = auth
	module, err := buildAIWorkbenchModule(context.Background(), AIWorkbenchDependencies{DB: db, ConversationOnly: true, AllowedOrganizationIDs: []string{"B"}, resolver: deps.organizationResolver, authorizer: auth}, nil)
	require.NoError(t, err)
	m := module.(aiWorkbenchModule)
	require.True(t, m.AdmittedOrganization("B"))
	require.False(t, m.AdmittedOrganization("A"))
	require.Equal(t, "UNAVAILABLE", m.PlanningReadiness(context.Background(), "B"))
	require.Equal(t, "UNAVAILABLE", m.TitleReadiness(context.Background(), "B"))
	require.Nil(t, m.application.service)
	require.Nil(t, m.application.plan)
	require.Nil(t, m.application.agent)
	server := httptest.NewServer(buildIsolatedApplicationHTTPServer(m.routes, deps, 15*time.Second).Handler)
	t.Cleanup(server.Close)
	key := uuid.NewString()
	code, raw, err := acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", key, `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	var result struct {
		Conversation aiworkbench.Conversation `json:"conversation"`
		Replay       bool                     `json:"replay"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	id := result.Conversation.ID
	require.NotEmpty(t, id)
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", key, `{}`)
	require.NoError(t, err)
	require.Equal(t, 200, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &result))
	require.True(t, result.Replay)
	require.Equal(t, id, result.Conversation.ID)
	patch := func(revision uint64, body string, expected int) {
		request, err := http.NewRequest("PATCH", server.URL+workbenchChatBase+"/"+id, strings.NewReader(body))
		require.NoError(t, err)
		request.Header.Set("Authorization", "Bearer operator")
		request.Header.Set("X-Requested-Organization-ID", "B")
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("If-Match", strconv.FormatUint(revision, 10))
		response, err := server.Client().Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, expected, response.StatusCode)
		if expected == 200 {
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		}
	}
	patch(1, `{"title":"本地会话接入检查","favorite":true}`, 200)
	require.Equal(t, "本地会话接入检查", result.Conversation.Title)
	require.True(t, result.Conversation.Favorite)
	patch(1, `{"title":"stale"}`, 409)
	patch(2, `{"archived":true}`, 200)
	require.True(t, result.Conversation.Archived)
	patch(3, `{"archived":false}`, 200)
	require.False(t, result.Conversation.Archived)
	for _, test := range []struct {
		actor, org string
		status     int
	}{{"operator", "B", 200}, {"operator2", "B", 404}, {"operator", "A", 503}, {"viewer", "B", 403}} {
		code, raw, err = acquisitionHTTPRequest(server, "GET", workbenchChatBase+"/"+id, test.actor, test.org, "", "")
		require.NoError(t, err)
		require.Equal(t, test.status, code, test.actor+"/"+test.org+": "+string(raw))
	}
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "viewer", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 403, code, string(raw))
	count := func() int64 {
		var n int64
		require.NoError(t, f.owner.Raw(`SELECT (SELECT count(*) FROM ai_workbench.commands)+(SELECT count(*) FROM ai_workbench.messages)+(SELECT count(*) FROM ai_workbench.execution_proposals)+(SELECT count(*) FROM ai_workbench.business_tasks)+(SELECT count(*) FROM ai_workbench.task_action_receipts)`).Scan(&n).Error)
		return n
	}
	before := count()
	for _, path := range []string{workbenchChatBase + "/" + id + "/messages", workbenchChatBase + "/" + id + "/proposals/" + uuid.NewString() + "/confirm", workbenchTaskBase + "/" + uuid.NewString() + "/start", workbenchTaskBase + "/" + uuid.NewString() + "/resume", workbenchTaskBase + "/" + uuid.NewString() + "/review"} {
		code, raw, err = acquisitionHTTPRequest(server, "POST", path, "operator", "B", uuid.NewString(), `{}`)
		require.NoError(t, err)
		require.Equal(t, 503, code, string(raw))
	}
	require.Equal(t, before, count(), "disabled execution must not write any receipts or execution facts")
	require.EqualValues(t, 0, f.fetches.Load())
	grants.viewer.Store(true)
	code, raw, err = acquisitionHTTPRequest(server, "POST", workbenchChatBase, "operator", "B", uuid.NewString(), `{}`)
	require.NoError(t, err)
	require.Equal(t, 403, code, string(raw))
	f.grants.revoked.Store(true)
	code, raw, err = acquisitionHTTPRequest(server, "GET", workbenchChatBase+"/"+id, "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 403, code, string(raw))
}

func TestAIWorkbenchMissingExecutionOwnerHidesProjection(t *testing.T) {
	a := &aiWorkbenchApplication{conversationOnly: true}
	scope := aiworkbench.Scope{OrganizationID: "B", ActorID: "operator"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "B", UserID: "operator", TokenExpiresAt: time.Now().Add(time.Hour)})
	view, err := a.taskView(ctx, scope, aiworkbench.BusinessTask{ID: uuid.NewString(), Scope: scope, Title: "retained intent"}, true)
	require.NoError(t, err)
	require.False(t, view.ProjectionAvailable)
	require.False(t, view.CanStart)
	require.False(t, view.CanResume)
	require.False(t, view.CanReconcile)
	require.False(t, view.CanReview)
	card := a.proposalCard(ctx, aiworkbench.ExecutionProposal{ID: uuid.NewString(), Scope: scope})
	require.False(t, card.DetailsAvailable)
	require.False(t, card.ExecutionAuthorized)
	require.False(t, card.TitleProfileReady)
	_, _, err = a.taskProjectionReader().LookupRun(ctx, agent.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, agent.Binding{}, "key")
	require.ErrorIs(t, err, aiworkbench.ErrUnavailable)
}
