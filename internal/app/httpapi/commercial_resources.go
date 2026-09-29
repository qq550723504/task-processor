package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
const commercialResourceEventsPath = commercialResourcesPath + "/events"
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
	return commercialResourcesModule{reader: reader, events: reader}, ctx.Err()
}

type commercialResourcesModule struct {
	reader orgresource.BalanceReader
	events orgresource.EventReader
}

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
	return []httproute.Descriptor{
		{Method: http.MethodGet, Path: commercialResourcesPath, Module: commercialResourcesModuleName, Permission: authz.PermissionWorkbenchCommercialRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 15 * time.Second, RejectUnreadRequestBody: true, Handler: m.read},
		{Method: http.MethodGet, Path: commercialResourceEventsPath, Module: commercialResourcesModuleName, Permission: authz.PermissionWorkbenchCommercialRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 15 * time.Second, RejectUnreadRequestBody: true, Handler: m.readEvents},
	}
}

func (m commercialResourcesModule) readEvents(c *gin.Context) {
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
	query, err := parseResourceEventQuery(c.Request.URL.RawQuery)
	if err != nil || c.Request.URL.ForceQuery {
		writeWorkbenchProtocolError(c, 400, "INVALID_REQUEST", "Resource filters are invalid")
		return
	}
	query.OrganizationID = identity.EffectiveOrganizationID
	if m.events == nil {
		writeWorkbenchProtocolError(c, 503, "DEPENDENCY_UNAVAILABLE", "Resource history is unavailable")
		return
	}
	result, err := m.events.ListEvents(c.Request.Context(), query)
	if errors.Is(err, orgresource.ErrInvalidInput) {
		writeWorkbenchProtocolError(c, 400, "INVALID_REQUEST", "Resource cursor is invalid")
		return
	}
	if err != nil || result.OrganizationID != query.OrganizationID {
		writeWorkbenchProtocolError(c, 503, "DEPENDENCY_UNAVAILABLE", "Resource history is unavailable")
		return
	}
	c.JSON(http.StatusOK, result)
}

func readCommercialEmptyBody(c *gin.Context) bool {
	if c.Request.Body == nil {
		return true
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	if err != nil || len(body) > 0 {
		writeWorkbenchProtocolError(c, 400, "INVALID_REQUEST", "Read takes no body")
		return false
	}
	return true
}

func parseResourceEventQuery(raw string) (orgresource.EventQuery, error) {
	query := orgresource.EventQuery{Limit: 50}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return query, orgresource.ErrInvalidInput
	}
	for key, value := range values {
		if len(value) != 1 || value[0] == "" {
			return query, orgresource.ErrInvalidInput
		}
		switch key {
		case "limit":
			query.Limit, err = strconv.Atoi(value[0])
			if err != nil || query.Limit < 1 || query.Limit > 50 {
				return query, orgresource.ErrInvalidInput
			}
		case "cursor":
			if len(value[0]) > 1024 {
				return query, orgresource.ErrInvalidInput
			}
			query.Cursor = value[0]
		case "resource_type":
			query.ResourceType = orgresource.ResourceType(value[0])
			if query.ResourceType != orgresource.ResourceStoreRenewalPeriod && query.ResourceType != orgresource.ResourceAIPoint && query.ResourceType != orgresource.ResourceDataRow {
				return query, orgresource.ErrInvalidInput
			}
		case "from", "until":
			parsed, err := time.Parse(time.RFC3339Nano, value[0])
			if err != nil {
				return query, orgresource.ErrInvalidInput
			}
			parsed = parsed.UTC()
			if key == "from" {
				query.From = &parsed
			} else {
				query.Until = &parsed
			}
		default:
			return query, orgresource.ErrInvalidInput
		}
	}
	if query.From != nil && query.Until != nil && !query.From.Before(*query.Until) {
		return query, orgresource.ErrInvalidInput
	}
	return query, nil
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
