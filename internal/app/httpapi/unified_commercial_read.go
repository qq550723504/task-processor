package httpapi

import (
	"context"
	"errors"
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
	"task-processor/internal/storecenter"
)

type unifiedCommercialModule struct {
	resources orgresource.BalanceReader
	stores    serviceSummaryReader
}
type serviceSummaryReader interface {
	ReadServiceSummary(context.Context, string, time.Time) (storecenter.ServiceSummary, error)
}

func buildUnifiedCommercialRead(ctx context.Context, resourceDB, storeDB *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	if ctx == nil || authorizer == nil {
		return nil, errors.New("commercial read authorization unavailable")
	}
	module := unifiedCommercialModule{}
	var err error
	if resourceDB != nil {
		module.resources, err = resourceadapter.NewGormRepository(resourceDB, resourceadapter.TransactionConfig{})
		if err != nil {
			return nil, err
		}
	}
	if storeDB != nil {
		module.stores, err = storecenter.NewGormStoreRepository(storeDB)
		if err != nil {
			return nil, err
		}
	}
	return module, nil
}

func (unifiedCommercialModule) Name() string { return "commercial-read" }
func (unifiedCommercialModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Workbench.Enabled
}
func (m unifiedCommercialModule) Register(registry *kernelmodule.Registry) error {
	registry.AddRoutes(httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/overview", Module: m.Name(), Permission: authz.PermissionWorkbenchCommercialRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 15 * time.Second, RejectUnreadRequestBody: true, Handler: m.read})
	return nil
}

func (m unifiedCommercialModule) read(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	if !readCommercialEmptyBody(c) {
		return
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) {
		writeWorkbenchProtocolError(c, 401, "AUTHENTICATION_REQUIRED", "Authentication is required")
		return
	}
	if !authidentity.IsBoundedIdentifier(identity.EffectiveOrganizationID) || identity.TenantID != identity.EffectiveOrganizationID {
		writeWorkbenchProtocolError(c, 403, "ORGANIZATION_ACCESS_DENIED", "Organization access is denied")
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeWorkbenchProtocolError(c, 400, "INVALID_REQUEST", "Overview takes no query")
		return
	}
	resources := map[string]any{"state": "unavailable", "value": nil}
	stores := map[string]any{"state": "unavailable", "value": nil}
	now := time.Now().UTC()
	if m.resources != nil {
		if value, err := m.resources.ReadBalances(c.Request.Context(), identity.EffectiveOrganizationID); err == nil && value.OrganizationID == identity.EffectiveOrganizationID {
			resources = map[string]any{"state": "available", "value": value}
		}
	}
	if m.stores != nil {
		if value, err := m.stores.ReadServiceSummary(c.Request.Context(), identity.EffectiveOrganizationID, now); err == nil {
			stores = map[string]any{"state": "available", "value": value}
		}
	}
	c.JSON(http.StatusOK, map[string]any{"schema_version": "unified-base-prepaid-v1", "organization_id": identity.EffectiveOrganizationID, "observed_at": now, "base_plan": map[string]any{"code": "base_payg", "name": "基础方案", "subscription_required": false, "store_period_days": 30, "ai_limit_period": "utc_calendar_month", "resource_expiry": "none"}, "resources": resources, "store_services": stores})
}
