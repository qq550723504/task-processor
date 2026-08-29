package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestImageAgentRoutesDeclareExactZitadelPolicies(t *testing.T) {
	routes := AppendRouteDescriptors(nil, requireHandler(t, &stubApplication{}))
	require.Len(t, routes, 10)
	want := map[string]string{
		http.MethodPost + " /api/v1/image-agent/runs":                                                  authz.PermissionImageAgentWrite,
		http.MethodGet + " /api/v1/image-agent/runs/:run_id":                                           authz.PermissionImageAgentRead,
		http.MethodPut + " /api/v1/image-agent/runs/:run_id/plan":                                      authz.PermissionImageAgentWrite,
		http.MethodPost + " /api/v1/image-agent/runs/:run_id/slots/:slot_id/attempts/:attempt/recover": authz.PermissionImageAgentWrite,
		http.MethodPost + " /api/v1/image-agent/runs/:run_id/slots/:slot_id/retry":                     authz.PermissionImageAgentWrite,
		http.MethodPost + " /api/v1/image-agent/runs/:run_id/results/approve":                          authz.PermissionImageAgentWrite,
		http.MethodPost + " /api/v1/image-agent/runs/:run_id/cancel":                                   authz.PermissionImageAgentWrite,
		http.MethodPost + " /api/v1/image-agent/runs/:run_id/restart":                                  authz.PermissionImageAgentWrite,
		http.MethodGet + " /api/v1/image-agent/runs/:run_id/events":                                    authz.PermissionImageAgentRead,
		http.MethodPost + " /api/v1/image-agent/runs/:run_id/commands/:action_id/resume":               authz.PermissionImageAgentWrite,
	}
	for _, route := range routes {
		require.Equal(t, ModuleName, route.Module)
		require.Equal(t, httproute.AuthPolicyVerifiedIdentity, route.AuthPolicy)
		require.Equal(t, want[route.Method+" "+route.Path], route.Permission)
		delete(want, route.Method+" "+route.Path)
	}
	require.Empty(t, want)
}

func TestImageAgentModuleRegistersRoutes(t *testing.T) {
	built, err := BuildModule(&stubApplication{})
	require.NoError(t, err)
	require.True(t, built.Module.Enabled(&config.Config{}))
	registry := kernelmodule.NewRegistry()
	require.NoError(t, built.Module.Register(registry))
	require.Len(t, registry.Routes(), 10)
}
