package httpapi

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	resourceadapter "task-processor/internal/integration/orgresource"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
)

const commercialResourcesPath = "/api/v1/workbench/commercial/resources"
const commercialResourcesModuleName = "commercial-resources"

func buildCommercialResourcesModule(ctx context.Context, db *gorm.DB) (kernelmodule.Module, error) {
	if ctx == nil || db == nil {
		return nil, orgresource.ErrInvalidInput
	}
	if err := resourceadapter.VerifyRuntimePermissions(ctx, db); err != nil {
		return nil, err
	}
	reader, err := resourceadapter.NewGormRepository(db, resourceadapter.TransactionConfig{})
	if err != nil {
		return nil, err
	}
	return commercialResourcesModule{reader: reader}, ctx.Err()
}

type commercialResourcesModule struct{ reader orgresource.BalanceReader }

func (commercialResourcesModule) Name() string { return commercialResourcesModuleName }
func (m commercialResourcesModule) Enabled(cfg *config.Config) bool {
	return m.reader != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m commercialResourcesModule) Register(reg *kernelmodule.Registry) error {
	if m.reader == nil {
		return orgresource.ErrBalanceUnavailable
	}
	reg.AddRoutes(m.routes()...)
	return nil
}
func (m commercialResourcesModule) routes() []httproute.Descriptor {
	return []httproute.Descriptor{{Method: http.MethodGet, Path: commercialResourcesPath, Module: commercialResourcesModuleName, Permission: authz.PermissionWorkbenchCommercialRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 15 * time.Second, RejectUnreadRequestBody: true, Handler: m.read}}
}
func (m commercialResourcesModule) read(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		writeWorkbenchProtocolError(c, 401, "AUTHENTICATION_REQUIRED", "Authentication is required")
		return
	}
	if !authidentity.IsBoundedIdentifier(identity.EffectiveOrganizationID) || identity.TenantID != identity.EffectiveOrganizationID {
		writeWorkbenchProtocolError(c, 403, "ORGANIZATION_ACCESS_DENIED", "Organization access is denied")
		return
	}
	var body []byte
	var bodyErr error
	if c.Request.Body != nil {
		body, bodyErr = io.ReadAll(io.LimitReader(c.Request.Body, 1))
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery || bodyErr != nil || len(body) != 0 {
		writeWorkbenchProtocolError(c, 400, "INVALID_REQUEST", "Resource read takes no query")
		return
	}
	result, err := m.reader.ReadBalances(c.Request.Context(), identity.EffectiveOrganizationID)
	if err != nil || result.OrganizationID != identity.EffectiveOrganizationID {
		writeWorkbenchProtocolError(c, 503, "DEPENDENCY_UNAVAILABLE", "Resource balance is unavailable")
		return
	}
	c.JSON(http.StatusOK, result)
}
