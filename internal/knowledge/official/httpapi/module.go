package httpapi

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/knowledge/official"
	"time"
)

const Base = "/api/v1/workbench/official-knowledge"

type Handler struct{ catalog *official.Catalog }

func NewHandler(catalog *official.Catalog) (*Handler, error) {
	if catalog == nil {
		return nil, official.ErrUnavailable
	}
	return &Handler{catalog}, nil
}
func failure(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": "官方知识请求未完成", "requestId": c.GetHeader("X-Request-ID"), "fieldErrors": []any{}})
}
func admit(c *gin.Context) bool {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) != 0 || c.Request.Body != nil && c.Request.Body != http.NoBody {
		failure(c, 400, official.ErrInvalid.Error())
		return false
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || identity.EffectiveOrganizationID == "" {
		failure(c, 401, "AUTHENTICATION_REQUIRED")
		return false
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		failure(c, 400, official.ErrInvalid.Error())
		return false
	}
	return true
}
func (h *Handler) List(c *gin.Context) {
	if !admit(c) {
		return
	}
	c.JSON(200, gin.H{"items": h.catalog.List()})
}
func (h *Handler) Get(c *gin.Context) {
	if !admit(c) {
		return
	}
	a, err := h.catalog.Get(c.Param("article_id"), c.Param("revision"))
	if err != nil {
		status := 400
		if errors.Is(err, official.ErrNotFound) {
			status = 404
		}
		failure(c, status, err.Error())
		return
	}
	c.JSON(200, a)
}
func Routes(h *Handler) []httproute.Descriptor {
	return []httproute.Descriptor{
		{Method: http.MethodGet, Path: Base, Module: "official-knowledge", Permission: authz.PermissionWorkbenchKnowledgeRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: true, Handler: h.List},
		{Method: http.MethodGet, Path: Base + "/:article_id/revisions/:revision", Module: "official-knowledge", Permission: authz.PermissionWorkbenchKnowledgeRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 10 * time.Second, RejectUnreadRequestBody: true, Handler: h.Get},
	}
}

type routeModule struct{ handler *Handler }

func NewModule(h *Handler) kernelmodule.Module { return routeModule{h} }
func (routeModule) Name() string               { return "official-knowledge" }
func (m routeModule) Enabled(c *config.Config) bool {
	return m.handler != nil && c != nil && c.Workbench.Enabled
}
func (m routeModule) Register(r *kernelmodule.Registry) error {
	for _, d := range Routes(m.handler) {
		r.AddRoutes(d)
	}
	return nil
}
func ValidateDescriptor(d httproute.Descriptor) error {
	for _, expected := range Routes(&Handler{}) {
		if d.Method == expected.Method && d.Path == expected.Path && d.Module == expected.Module && d.Permission == expected.Permission && d.AuthPolicy == expected.AuthPolicy && d.OrganizationAccessPolicy == expected.OrganizationAccessPolicy && d.OrganizationTargetResolver == nil && d.RequestTimeout == expected.RequestTimeout && d.RejectUnreadRequestBody && d.Handler != nil {
			return nil
		}
	}
	return errors.New("official knowledge route loses live access or bounded read contract")
}
