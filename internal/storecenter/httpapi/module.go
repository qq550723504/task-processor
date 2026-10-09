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
		route(http.MethodGet, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyLiveWrite, m.handler.List),
		route(http.MethodPost, "/api/v1/workbench/stores", authz.PermissionWorkbenchStoreCreate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Create),
		route(http.MethodGet, "/api/v1/workbench/stores/:store_id", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyLiveWrite, m.handler.Get),
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
	if m.handler.officialConnections != nil {
		registry.AddRoutes(
			route(http.MethodGet, "/api/v1/workbench/stores/:store_id/connection", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyLiveWrite, m.handler.ReadOfficialConnection),
			route(http.MethodGet, "/api/v1/workbench/stores/:store_id/connection/applications", authz.PermissionWorkbenchStoreRead, httproute.OrganizationAccessPolicyLiveWrite, m.handler.OfficialApplications),
			route(http.MethodPost, "/api/v1/workbench/stores/:store_id/connection/begin", authz.PermissionWorkbenchStoreUpdate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.BeginOfficialConnection),
			route(http.MethodPost, "/api/v1/workbench/stores/:store_id/connection/complete", authz.PermissionWorkbenchStoreUpdate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.CompleteOfficialConnection),
			route(http.MethodPost, "/api/v1/workbench/stores/:store_id/connection/query", authz.PermissionWorkbenchStoreUpdate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.ResumeOfficialConnectionQuery),
			route(http.MethodPost, "/api/v1/workbench/stores/:store_id/connection/disconnect", authz.PermissionWorkbenchStoreUpdate, httproute.OrganizationAccessPolicyLiveWrite, m.handler.DisconnectOfficialConnection),
		)
	}
	return nil
}

func route(method, path, permission string, access httproute.OrganizationAccessPolicy, handler func(*gin.Context)) httproute.Descriptor {
	return httproute.Descriptor{
		Method: method, Path: path, Module: ModuleName, Permission: permission,
		AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: access, Handler: httproute.WithRequestBodyReadTimeout(requestBodyReadTimeout, handler),
	}
}
