package httpapi

import (
	"net/http"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
)

const ModuleName = "local-agent"

func AppendRouteDescriptors(routes []httproute.Descriptor, handler *Handler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite, Handler: handler.Create},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs/claim", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite, Handler: handler.Claim},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs/:job_id/claim", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite, Handler: handler.ClaimJob},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs/:job_id/result", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite, Handler: handler.SubmitResult},
	)
}
