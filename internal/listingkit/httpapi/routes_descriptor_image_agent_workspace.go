package httpapi

import (
	"net/http"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
)

func AppendImageAgentWorkspaceRouteDescriptors(routes []httproute.Descriptor, handler ImageAgentWorkspaceRouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	verified := httproute.AuthPolicyVerifiedIdentity
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks/:task_id/image-agent-assets", Module: "listing-kit", Permission: authz.PermissionImageAgentRead, AuthPolicy: verified, Handler: handler.GetImageAgentAssets},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/listing-kits/tasks/:task_id/image-agent-runs", Module: "listing-kit", Permission: authz.PermissionImageAgentWrite, AuthPolicy: verified, Handler: handler.CreateImageAgentRun},
	)
}
