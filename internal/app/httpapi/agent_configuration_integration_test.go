package httpapi

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/agentconfig"
	confighttp "task-processor/internal/agentconfig/httpapi"
	"task-processor/internal/authz"
	configstore "task-processor/internal/integration/persistence/agentconfig"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

func TestAgentConfigurationUsesCurrentEnterpriseWithoutTextRuntime(t *testing.T) {
	f := newAcquisitionHTTPFixture(t)
	require.NoError(t, configstore.InstallSchema(f.owner))
	auth, e := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, e)
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	deps.organizationResolver = workbenchcontext.NewResolver(agentFixtureGrants{f.grants}, "project", "v1", nil)
	deps.authorizer = auth
	module, e := buildAgentConfigurationModule(context.Background(), f.owner, deps.organizationResolver, auth, nil, nil)
	require.NoError(t, e)
	reg := kernelmodule.NewRegistry()
	require.NoError(t, module.Register(reg))
	app := buildIsolatedApplicationHTTPServer(reg.Routes(), deps, 10*time.Second)
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	call := func(actor, org, method, path, body, key, match string) (int, []byte) {
		r, e := http.NewRequest(method, server.URL+confighttp.Base+path, strings.NewReader(body))
		require.NoError(t, e)
		r.Header.Set("Authorization", "Bearer "+actor)
		r.Header.Set("X-Requested-Organization-ID", org)
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if match == "*" {
			r.Header.Set("If-None-Match", "*")
		} else if match != "" {
			r.Header.Set("If-Match", `"`+match+`"`)
		}
		resp, e := http.DefaultClient.Do(r)
		require.NoError(t, e)
		defer resp.Body.Close()
		raw, e := io.ReadAll(resp.Body)
		require.NoError(t, e)
		require.Contains(t, resp.Header.Get("Cache-Control"), "no-store")
		return resp.StatusCode, raw
	}
	code, raw := call("admin", "B", "GET", "/product.title.agent", "", "", "")
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"activation":"NOT_ENABLED"`)
	require.Contains(t, string(raw), `"canUse":false`)
	require.Contains(t, string(raw), `"support":"REQUIRED"`)
	key := uuid.NewString()
	code, raw = call("viewer", "B", "POST", "/product.title.agent/enable", `{}`, key, "*")
	require.Equal(t, 403, code, string(raw))
	code, raw = call("admin", "B", "POST", "/product.title.agent/enable", `{}`, key, "*")
	require.Equal(t, 200, code, string(raw))
	receipt := string(raw)
	code, raw = call("admin", "B", "POST", "/product.title.agent/disable", `{}`, uuid.NewString(), "1")
	require.Equal(t, 200, code, string(raw))
	code, raw = call("admin", "B", "POST", "/product.title.agent/enable", `{}`, key, "*")
	require.Equal(t, 200, code, string(raw))
	require.Equal(t, receipt, string(raw))
	code, raw = call("viewer", "B", "GET", "/mine", "", "", "")
	require.Equal(t, 200, code, string(raw))
	require.Contains(t, string(raw), `"activation":"DISABLED"`)
	code, raw = call("admin", "B", "POST", "/product.title.agent/templates", `{"name":"模板","targetPlatform":"shein"}`, uuid.NewString(), "")
	require.Equal(t, 200, code, string(raw))
	var created agentconfig.Receipt
	require.NoError(t, json.Unmarshal(raw, &created))
	code, raw = call("admin", "A", "GET", "/product.title.agent/templates/"+created.TemplateID, "", "", "")
	require.Equal(t, 404, code, string(raw))
	f.grants.revoked.Store(true)
	code, raw = call("admin", "B", "POST", "/product.title.agent/enable", `{}`, uuid.NewString(), "2")
	require.Equal(t, 403, code, string(raw))
}
func TestAgentConfigurationRejectsDifferentServingPools(t *testing.T) {
	_, e := buildAgentConfigurationModule(context.Background(), &gorm.DB{}, nil, nil, nil, &productAgentApplication{config: ProductAgentDependencies{RunDB: &gorm.DB{}}})
	require.ErrorIs(t, e, agentconfig.ErrUnavailable)
}
