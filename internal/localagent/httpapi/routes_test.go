package httpapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/localagent"
)

func TestLocalAgentRoutesRequireZitadelAndPermission(t *testing.T) {
	routes := AppendRouteDescriptors(nil, NewHandler(localagent.NewService(nil)))
	require.Len(t, routes, 4)
	for _, route := range routes {
		require.Equal(t, ModuleName, route.Module)
		require.Equal(t, authz.PermissionLocalAgentWrite, route.Permission)
		require.True(t, listingkithttpapi.RouteRequiresZitadelAuth(route))
		require.NotNil(t, listingkithttpapi.NewRouteRoleMiddleware(route))
	}
	require.Equal(t, []string{
		http.MethodPost + " /api/v1/local-agent/1688-jobs",
		http.MethodPost + " /api/v1/local-agent/1688-jobs/claim",
		http.MethodPost + " /api/v1/local-agent/1688-jobs/:job_id/claim",
		http.MethodPost + " /api/v1/local-agent/1688-jobs/:job_id/result",
	}, routeKeys(routes))
}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, route.Method+" "+route.Path)
	}
	return keys
}
