package httpapi

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
)

func TestImageAgentWorkspaceRoutesUseVerifiedIdentityAndImageAgentPermissions(t *testing.T) {
	routes := AppendImageAgentWorkspaceRouteDescriptors(nil, imageAgentWorkspaceRouteHandlerStub{})
	require.Len(t, routes, 2)
	require.Equal(t, []httproute.Descriptor{
		{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/image-agent-assets", Module: "listing-kit", Permission: authz.PermissionImageAgentRead, AuthPolicy: httproute.AuthPolicyVerifiedIdentity},
		{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/image-agent-runs", Module: "listing-kit", Permission: authz.PermissionImageAgentWrite, AuthPolicy: httproute.AuthPolicyVerifiedIdentity},
	}, descriptorsWithoutHandlers(routes))
}

func descriptorsWithoutHandlers(routes []httproute.Descriptor) []httproute.Descriptor {
	out := make([]httproute.Descriptor, len(routes))
	for index, route := range routes {
		route.Handler = nil
		out[index] = route
	}
	return out
}

type imageAgentWorkspaceRouteHandlerStub struct{}

func (imageAgentWorkspaceRouteHandlerStub) GetImageAgentAssets(*gin.Context) {}
func (imageAgentWorkspaceRouteHandlerStub) CreateImageAgentRun(*gin.Context) {}
