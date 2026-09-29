package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"time"
)

const ModuleName = "knowledge"

type routeModule struct{ handler *Handler }

func NewModule(handler *Handler) kernelmodule.Module { return routeModule{handler} }
func (routeModule) Name() string                     { return ModuleName }
func (m routeModule) Enabled(c *config.Config) bool {
	return m.handler != nil && c != nil && c.Workbench.Enabled
}
func (m routeModule) Register(registry *kernelmodule.Registry) error {
	for _, d := range Routes(m.handler) {
		registry.AddRoutes(d)
	}
	return nil
}
func Routes(h *Handler) []httproute.Descriptor {
	read, manage := authz.PermissionWorkbenchKnowledgeRead, authz.PermissionWorkbenchKnowledgeManage
	base := "/api/v1/workbench/knowledge-bases"
	source := "/api/v1/workbench/knowledge-sources/:source_id"
	var result []httproute.Descriptor
	for _, r := range []struct {
		method, path, permission string
		handler                  gin.HandlerFunc
	}{
		{http.MethodGet, base, read, h.ListBases}, {http.MethodPost, base, manage, h.CreateBase},
		{http.MethodGet, base + "/:knowledge_base_id", read, h.GetBase}, {http.MethodPut, base + "/:knowledge_base_id", manage, h.UpdateBase}, {http.MethodPost, base + "/:knowledge_base_id/disable", manage, h.DisableBase},
		{http.MethodGet, base + "/:knowledge_base_id/sources", read, h.ListSources}, {http.MethodPost, base + "/:knowledge_base_id/sources", manage, h.CreateSource},
		{http.MethodGet, source, read, h.GetSource}, {http.MethodPost, source + "/revisions", manage, h.CreateRevision}, {http.MethodPost, source + "/disable", manage, h.DisableSource},
		{http.MethodGet, source + "/revisions/:revision_id/preview", read, h.Preview},
	} {
		result = append(result, httproute.Descriptor{Method: r.method, Path: r.path, Module: ModuleName, Permission: r.permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 40 * time.Second, RejectUnreadRequestBody: r.method == http.MethodGet, Handler: httproute.WithRequestBodyReadTimeout(30*time.Second, r.handler)})
	}
	return result
}
