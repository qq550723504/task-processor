package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/sourceaccountregistry"
)

const ModuleName = "source-account-registry"

type routeModule struct{ handler *Handler }

func NewModule(handler *Handler) kernelmodule.Module { return routeModule{handler: handler} }
func (routeModule) Name() string                     { return ModuleName }

func (m routeModule) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}

func (m routeModule) Register(registry *kernelmodule.Registry) error {
	registry.AddRoutes(
		route(http.MethodPost, "/api/v1/workbench/source-accounts", authz.PermissionWorkbenchSourceAccountManage, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Create),
		route(http.MethodGet, "/api/v1/workbench/source-accounts", authz.PermissionWorkbenchSourceAccountRead, httproute.OrganizationAccessPolicyCachedRead, m.handler.List),
		route(http.MethodGet, "/api/v1/workbench/source-accounts/:source_account_id", authz.PermissionWorkbenchSourceAccountRead, httproute.OrganizationAccessPolicyCachedRead, m.handler.Get),
		route(http.MethodPost, "/api/v1/workbench/source-accounts/:source_account_id/disable", authz.PermissionWorkbenchSourceAccountManage, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Disable),
		route(http.MethodPost, "/api/v1/workbench/source-accounts/:source_account_id/enable", authz.PermissionWorkbenchSourceAccountManage, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Enable),
	)
	return nil
}

func route(method, path, permission string, policy httproute.OrganizationAccessPolicy, handler func(*gin.Context)) httproute.Descriptor {
	return httproute.Descriptor{
		Method: method, Path: path, Module: ModuleName, Permission: permission,
		AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: policy,
		RequestTimeout: sourceaccountregistry.Timeout,
		Handler:        httproute.WithRequestBodyReadTimeout(sourceaccountregistry.Timeout, handler),
	}
}
