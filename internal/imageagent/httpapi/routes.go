package httpapi

import (
	"net/http"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
)

const ModuleName = "image-agent"

func AppendRouteDescriptors(routes []httproute.Descriptor, handler *Handler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	verified := httproute.AuthPolicyVerifiedIdentity
	return append(routes,
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/image-agent/runs", Module: ModuleName, Permission: authz.PermissionImageAgentWrite, AuthPolicy: verified, Handler: handler.Create},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/image-agent/runs/:run_id", Module: ModuleName, Permission: authz.PermissionImageAgentRead, AuthPolicy: verified, Handler: handler.Get},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/image-agent/runs/:run_id/plan", Module: ModuleName, Permission: authz.PermissionImageAgentWrite, AuthPolicy: verified, Handler: handler.ReplacePlan},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/image-agent/runs/:run_id/slots/:slot_id/retry", Module: ModuleName, Permission: authz.PermissionImageAgentWrite, AuthPolicy: verified, Handler: handler.RetrySlot},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/image-agent/runs/:run_id/results/approve", Module: ModuleName, Permission: authz.PermissionImageAgentWrite, AuthPolicy: verified, Handler: handler.ApproveResults},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/image-agent/runs/:run_id/cancel", Module: ModuleName, Permission: authz.PermissionImageAgentWrite, AuthPolicy: verified, Handler: handler.Cancel},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/image-agent/runs/:run_id/events", Module: ModuleName, Permission: authz.PermissionImageAgentRead, AuthPolicy: verified, Handler: handler.Events},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/image-agent/runs/:run_id/commands/:action_id/resume", Module: ModuleName, Permission: authz.PermissionImageAgentWrite, AuthPolicy: verified, Handler: handler.Resume},
	)
}
