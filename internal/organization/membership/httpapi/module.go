package httpapi

import (
	"github.com/gin-gonic/gin"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"time"
)

const ModuleName = "organization-membership"

type routeModule struct{ handler *Handler }

func NewModule(handler *Handler) kernelmodule.Module { return routeModule{handler} }
func (routeModule) Name() string                     { return ModuleName }
func (m routeModule) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m routeModule) Register(reg *kernelmodule.Registry) error {
	for _, item := range []struct {
		path    string
		handler gin.HandlerFunc
	}{
		{"/api/v1/account/members", m.handler.List},
		{"/api/v1/account/members/:member_id", m.handler.Read},
	} {
		reg.AddRoutes(httproute.Descriptor{Method: "GET", Path: item.path, Module: ModuleName, Permission: authz.PermissionWorkbenchOrganizationMemberRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: ResolveOrganizationTarget, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: item.handler})
	}
	if m.handler.commands != nil {
		for _, item := range []struct {
			method, path string
			handler      gin.HandlerFunc
		}{
			{"POST", "/api/v1/account/members/invitations", m.handler.Invite},
			{"POST", "/api/v1/account/members/:member_id/role", m.handler.ChangeRole},
			{"POST", "/api/v1/account/members/:member_id/remove", m.handler.Remove},
			{"GET", "/api/v1/account/member-operations/:operation_id", m.handler.GetOperation},
			{"POST", "/api/v1/account/member-operations/:operation_id/verify", m.handler.VerifyOperation},
		} {
			reg.AddRoutes(httproute.Descriptor{Method: item.method, Path: item.path, Module: ModuleName, Permission: authz.PermissionWorkbenchOrganizationMemberManage, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: ResolveMutationTarget, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: httproute.WithRequestBodyReadTimeout(10*time.Second, item.handler)})
		}
	}
	return nil
}
