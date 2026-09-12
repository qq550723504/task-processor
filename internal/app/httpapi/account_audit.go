package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"task-processor/internal/app/accountaudit"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	store "task-processor/internal/integration/persistence/sourceaccountregistry"
	kernelmodule "task-processor/internal/kernel/module"
	registry "task-processor/internal/sourceaccountregistry"
	"task-processor/internal/workbenchcontext"
)

const accountAuditPath = "/api/v1/account/audit"

var accountAuditScope = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var accountAuditLimit = regexp.MustCompile(`^[1-9][0-9]{0,2}$`)

type accountAuditModule struct{ query *accountaudit.Query }

func (accountAuditModule) Name() string { return "account-audit" }
func (m accountAuditModule) Enabled(cfg *config.Config) bool {
	return m.query != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m accountAuditModule) Register(modules *kernelmodule.Registry) error {
	if m.query == nil {
		return registry.ErrUnavailable
	}
	modules.AddRoutes(httproute.Descriptor{Method: http.MethodGet, Path: accountAuditPath, Module: m.Name(), Permission: authz.PermissionWorkbenchSourceAccountRead, AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		// LiveWrite is the existing resolver policy for fresh grants. This GET
		// requires only read permission and never invokes a mutation.
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: accountAuditTarget, RejectUnreadRequestBody: true, RequestTimeout: registry.Timeout, Handler: m.read})
	return nil
}
func accountAuditTarget(request *http.Request) (string, error) {
	values := request.Header.Values("X-Requested-Organization-ID")
	if len(values) == 0 {
		return "", workbenchcontext.ErrOrganizationSelectionRequired
	}
	if len(values) != 1 || !accountAuditScope.MatchString(values[0]) {
		return "", registry.ErrInvalid
	}
	return values[0], nil
}
func (m accountAuditModule) read(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	limit, cursor, err := accountAuditPageInput(c.Request.URL.RawQuery)
	if err != nil {
		writeAccountAuditError(c, err)
		return
	}
	page, err := m.query.Read(c.Request.Context(), limit, cursor)
	if err != nil {
		writeAccountAuditError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}
func accountAuditPageInput(raw string) (int, string, error) {
	if len(raw) > 2300 {
		return 0, "", registry.ErrInvalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return 0, "", registry.ErrInvalid
	}
	for key, value := range values {
		if (key != "limit" && key != "cursor") || len(value) != 1 || value[0] == "" {
			return 0, "", registry.ErrInvalid
		}
	}
	limit := 20
	if values.Has("limit") {
		if !accountAuditLimit.MatchString(values.Get("limit")) {
			return 0, "", registry.ErrInvalid
		}
		limit, _ = strconv.Atoi(values.Get("limit"))
	}
	if limit < 1 || limit > registry.MaxPageLimit {
		return 0, "", registry.ErrInvalid
	}
	return limit, values.Get("cursor"), nil
}
func writeAccountAuditError(c *gin.Context, err error) {
	status, code := 503, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, registry.ErrAuthenticationRequired):
		status, code = 401, "AUTHENTICATION_REQUIRED"
	case errors.Is(err, registry.ErrForbidden):
		status, code = 403, "PERMISSION_DENIED"
	case errors.Is(err, registry.ErrInvalid):
		status, code = 400, "INVALID_REQUEST"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		status, code = 504, "DEADLINE_EXCEEDED"
	}
	writeWorkbenchProtocolError(c, status, code, "Operation history request could not be completed")
}
func buildAccountAuditModule(ctx context.Context, db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	repository, err := store.NewRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	service, err := registry.NewService(repository, authorizer)
	if err != nil {
		return nil, err
	}
	history, err := registry.NewHistoryService(service, repository)
	if err != nil {
		return nil, err
	}
	query, err := accountaudit.New(history)
	if err != nil {
		return nil, err
	}
	return accountAuditModule{query: query}, nil
}

// NewAccountAuditApplication is explicit isolated composition. It owns no DB,
// schema, listener, identity provider or default runtime registration.
func NewAccountAuditApplication(ctx context.Context, db *gorm.DB, verifier zitadelruntime.Verifier, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer) (*http.Server, error) {
	if ctx == nil || db == nil || verifier == nil || resolver == nil || authorizer == nil {
		return nil, registry.ErrUnavailable
	}
	module, err := buildAccountAuditModule(ctx, db, authorizer)
	if err != nil {
		return nil, err
	}
	modules := kernelmodule.NewRegistry()
	if err = module.Register(modules); err != nil {
		return nil, err
	}
	return buildIsolatedApplicationHTTPServer(modules.Routes(), routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer}, registry.Timeout), nil
}
