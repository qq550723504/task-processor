package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"testing"
)

type catalogFixture struct{}

func (catalogFixture) ReadCatalog(context.Context) ([]agentconfig.CatalogEntry, error) {
	return []agentconfig.CatalogEntry{{Definition: commercetool.AgentDefinition{ID: "product.title.agent", Version: "v1.0.0"}, ParameterSchema: agentconfig.ParameterSchema}}, nil
}

type repoFixture struct {
	agentconfig.Repository
	calls   int
	command agentconfig.Command
}

func (r *repoFixture) ReadAgent(context.Context, agent.Scope, string) (agentconfig.OrganizationAgent, error) {
	return agentconfig.OrganizationAgent{AgentID: "product.title.agent", Activation: "ENABLED", Revision: "1", ActivationEpoch: "1"}, nil
}
func (r *repoFixture) Execute(ctx context.Context, c agentconfig.Command, checks ...func(context.Context) error) (agentconfig.Receipt, error) {
	r.calls++
	r.command = c
	for _, check := range checks {
		if e := check(ctx); e != nil {
			return agentconfig.Receipt{}, e
		}
	}
	return agentconfig.Receipt{CommandID: uuid.NewString(), AgentID: c.AgentID, Operation: c.Operation, Revision: "2"}, nil
}
func harness(r *repoFixture, permissions map[string]bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &Handler{Repository: r, Catalog: catalogFixture{}, Authorize: func(_ context.Context, p ...string) (agent.Scope, error) {
		for _, v := range p {
			if permissions[v] {
				return agent.Scope{OrganizationID: "B", ActorID: "actor"}, nil
			}
		}
		return agent.Scope{}, agentconfig.ErrForbidden
	}}
	for _, route := range Routes(h) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	return router
}
func call(r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, Base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestConfigureGrantReadsWithoutImplyingUse(t *testing.T) {
	repo := &repoFixture{}
	r := harness(repo, map[string]bool{authz.PermissionWorkbenchAgentConfigure: true})
	response := call(r, "GET", "/product.title.agent", "", nil)
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"canConfigure":true`)
	require.Contains(t, response.Body.String(), `"canUse":false`)
	require.Equal(t, 403, call(r, "GET", "/product.title.agent/recent-runs", "", nil).Code)
	r = harness(repo, map[string]bool{authz.PermissionWorkbenchAgentRead: true})
	require.Equal(t, 403, call(r, "POST", "/product.title.agent/disable", `{}`, map[string]string{"Idempotency-Key": uuid.NewString(), "If-Match": `"1"`}).Code)
	require.Zero(t, repo.calls)
}
func TestDefaultClearAndStrongPreconditions(t *testing.T) {
	repo := &repoFixture{}
	r := harness(repo, map[string]bool{authz.PermissionWorkbenchAgentConfigure: true})
	headers := map[string]string{"Idempotency-Key": uuid.NewString(), "If-Match": `"1"`}
	for _, body := range []string{`{}`, `{"templateId":null}`, `{"revision":null}`, `{"templateId":null,"revision":"1"}`, `{"templateId":null,"revision":null,"revision":null}`} {
		require.Equal(t, http.StatusBadRequest, call(r, "PUT", "/product.title.agent/default-template", body, headers).Code)
	}
	require.Zero(t, repo.calls)
	require.Equal(t, 200, call(r, "PUT", "/product.title.agent/default-template", `{"templateId":null,"revision":null}`, headers).Code)
	require.Nil(t, repo.command.Default)
	for _, match := range []string{`W/"1"`, `"01"`, `1`, `"9223372036854775808"`, `"1","2"`} {
		headers["If-Match"] = match
		require.Equal(t, 400, call(r, "POST", "/product.title.agent/disable", `{}`, headers).Code)
	}
	delete(headers, "If-Match")
	require.Equal(t, 428, call(r, "POST", "/product.title.agent/disable", `{}`, headers).Code)
	require.Equal(t, 400, call(r, "GET", "/market?%zz=1", "", nil).Code)
}

func TestRequiredEvidenceReadinessControlsUseWithoutHidingRuns(t *testing.T) {
	h := &Handler{Repository: &repoFixture{}, Authorize: func(context.Context, ...string) (agent.Scope, error) {
		return agent.Scope{OrganizationID: "B", ActorID: "actor"}, nil
	}}
	for _, readiness := range []string{"REQUIRES_AUTHORIZATION", "AVAILABLE", "REQUIRES_AUTHORIZATION"} {
		h.Capabilities = func(context.Context, agentconfig.CatalogEntry) []agentconfig.Capability {
			return []agentconfig.Capability{{ID: "text.generate", Support: "REQUIRED", Readiness: "AVAILABLE"}, {ID: "product.source-evidence", Support: "REQUIRED", Readiness: readiness}}
		}
		entry, err := h.project(context.Background(), agent.Scope{OrganizationID: "B", ActorID: "actor"}, agentconfig.CatalogEntry{Definition: commercetool.AgentDefinition{ID: "product.title.agent", Version: "v1.0.0"}})
		require.NoError(t, err)
		require.Equal(t, readiness == "AVAILABLE", entry["canUse"])
		require.Equal(t, true, entry["canReadRuns"])
	}
}
