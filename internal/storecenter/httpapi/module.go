package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

const ModuleName = "store-center"

const requestBodyReadTimeout = 30 * time.Second

type routeModule struct{ handler *Handler }

func NewModule(handler *Handler) kernelmodule.Module { return routeModule{handler: handler} }

func (routeModule) Name() string { return ModuleName }

func (m routeModule) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}

func (m routeModule) Register(registry *kernelmodule.Registry) error {
	registry.AddRoutes(
		route(http.MethodGet, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyCachedRead, m.handler.List),
		route(http.MethodPost, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreCreate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Create),
		route(http.MethodPost, "/api/v1/workbench/stores/:store_id/resume", authz.PermissionWorkbenchStoreCreate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.ResumeCreate),
		route(http.MethodGet, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyCachedRead, m.handler.Get),
		route(http.MethodPut, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreUpdate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Update),
		route(http.MethodPost, "/api/v1/workbench/stores/:store_id/disable", authz.PermissionWorkbenchStoreLifecycle, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Disable),
		route(http.MethodPost, "/api/v1/workbench/stores/:store_id/enable", authz.PermissionWorkbenchStoreLifecycle, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Enable),
		route(http.MethodDelete, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreDelete, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Delete),
	)
	if m.handler.serviceLifecycle != nil {
		registry.AddRoutes(
			route(http.MethodPost, "/api/v1/workbench/stores/:store_id/activate", authz.PermissionWorkbenchStoreLifecycle, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Activate),
			route(http.MethodPost, "/api/v1/workbench/stores/:store_id/renew", authz.PermissionWorkbenchStoreLifecycle, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Renew),
			route(http.MethodPost, "/api/v1/workbench/stores/:store_id/reactivate", authz.PermissionWorkbenchStoreLifecycle, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Reactivate),
		)
	}
	return nil
}

func route(method, path, permission string, access httproute.OrganizationAccessPolicy, handler func(*gin.Context)) httproute.Descriptor {
	return httproute.Descriptor{
		Method: method, Path: path, Module: ModuleName, Permission: permission,
		AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: access, Handler: httproute.WithRequestBodyReadTimeout(requestBodyReadTimeout, handler),
	}
}
