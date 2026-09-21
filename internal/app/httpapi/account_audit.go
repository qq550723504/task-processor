package httpapi

import (
	"context"
	"errors"
	"fmt"
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
	accountallocationstore "task-processor/internal/integration/persistence/accountallocation"
	accountprofilestore "task-processor/internal/integration/persistence/accountprofile"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
	store "task-processor/internal/integration/persistence/sourceaccountregistry"
	kernelmodule "task-processor/internal/kernel/module"
	membership "task-processor/internal/organization/membership"
	registry "task-processor/internal/sourceaccountregistry"
	"task-processor/internal/workbenchcontext"
)

const accountAuditPath = "/api/v1/account/audit"

var accountAuditScope = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var accountAuditLimit = regexp.MustCompile(`^[1-9][0-9]{0,2}$`)

type accountAuditModule struct{ query *accountaudit.Query }

type profileAuditReader struct {
	repository *accountprofilestore.Repository
}

func (r profileAuditReader) ListRecentAudit(ctx context.Context, organizationID string, limit int, actor, operation string, after *accountaudit.AuditPosition) (accountaudit.AdditionalAuditPage, error) {
	var position *accountprofilestore.AuditPosition
	if after != nil {
		id, err := strconv.ParseInt(after.Key, 10, 64)
		if err != nil || id < 1 {
			return accountaudit.AdditionalAuditPage{}, registry.ErrInvalid
		}
		position = &accountprofilestore.AuditPosition{CreatedAt: after.CreatedAt, ID: id}
	}
	items, next, err := r.repository.ListRecentAudit(ctx, organizationID, limit, actor, operation, position)
	if err != nil {
		return accountaudit.AdditionalAuditPage{}, err
	}
	page := accountaudit.AdditionalAuditPage{Items: make([]accountaudit.AdditionalAuditEvent, 0, len(items))}
	for _, item := range items {
		page.Items = append(page.Items, accountaudit.AdditionalAuditEvent{EventType: "account_business_profile.updated", Actor: item.ActorID, Time: item.CreatedAt, ObjectType: "account_business_profile", ObjectReference: item.UserID, Operation: item.Operation, Version: item.Version, Key: fmt.Sprintf("%020d", item.ID)})
	}
	if next != nil {
		page.Next = &accountaudit.AuditPosition{CreatedAt: next.CreatedAt, Key: fmt.Sprintf("%020d", next.ID)}
	}
	return page, nil
}

type membershipAuditReader struct{ repository *memberstore.Repository }

func (r membershipAuditReader) ListRecentAudit(ctx context.Context, organizationID string, limit int, actor, operation string, after *accountaudit.AuditPosition) (accountaudit.AdditionalAuditPage, error) {
	var position *membership.AuditPosition
	if after != nil {
		position = &membership.AuditPosition{CreatedAt: after.CreatedAt, OperationKey: after.Key}
	}
	items, next, err := r.repository.ListRecentAudit(ctx, organizationID, limit, actor, operation, position)
	if err != nil {
		return accountaudit.AdditionalAuditPage{}, err
	}
	page := accountaudit.AdditionalAuditPage{Items: make([]accountaudit.AdditionalAuditEvent, 0, len(items))}
	for _, item := range items {
		page.Items = append(page.Items, accountaudit.AdditionalAuditEvent{EventType: "organization_membership.changed", Actor: item.ActorID, Time: item.CreatedAt, ObjectType: "organization_member", ObjectReference: item.TargetUserID, Operation: item.Operation, Version: item.Revision, Key: item.OperationKey})
	}
	if next != nil {
		page.Next = &accountaudit.AuditPosition{CreatedAt: next.CreatedAt, Key: next.OperationKey}
	}
	return page, nil
}

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
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: accountOrganizationTarget, RejectUnreadRequestBody: true, RequestTimeout: registry.Timeout, Handler: m.read})
	return nil
}
func accountOrganizationTarget(request *http.Request) (string, error) {
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
	filter, err := accountAuditFilterInput(c.Request.URL.Query())
	if err != nil {
		writeAccountAuditError(c, err)
		return
	}
	page, err := m.query.ReadFiltered(c.Request.Context(), limit, cursor, filter)
	if err != nil {
		writeAccountAuditError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func accountAuditFilterInput(values url.Values) (accountaudit.Filter, error) {
	actor := values.Get("actor")
	if actor != "" && (len(values["actor"]) != 1 || !accountAuditScope.MatchString(actor)) {
		return accountaudit.Filter{}, registry.ErrInvalid
	}
	operation := values.Get("operation")
	if operation != "" && (len(values["operation"]) != 1 || operation != string(registry.OperationRegister) && operation != string(registry.OperationEnable) && operation != string(registry.OperationDisable) && operation != "set_target" && operation != "revoke" && operation != "update" && operation != "invite" && operation != "role" && operation != "remove") {
		return accountaudit.Filter{}, registry.ErrInvalid
	}
	filter := accountaudit.Filter{ActorSubject: actor}
	if operation == "set_target" || operation == "revoke" {
		filter.ResourceOperation = operation
	} else if operation == "update" {
		filter.ProfileOperation = operation
	} else if operation == "invite" || operation == "role" || operation == "remove" {
		filter.MembershipOperation = operation
	} else {
		filter.Kind = registry.OperationKind(operation)
	}
	return filter, nil
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
		if (key != "limit" && key != "cursor" && key != "actor" && key != "operation") || len(value) != 1 || value[0] == "" {
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
func buildAccountAuditModule(ctx context.Context, sourceDB, commercialDB, membershipDB *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	repository, err := store.NewRepository(ctx, sourceDB)
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
	allocationRepository, err := accountallocationstore.New(commercialDB)
	if err != nil {
		return nil, err
	}
	profileRepository, err := accountprofilestore.New(sourceDB)
	if err != nil {
		return nil, err
	}
	var profileHistory accountaudit.AdditionalHistory = profileAuditReader{repository: profileRepository}
	var membershipHistory accountaudit.AdditionalHistory
	if membershipDB != nil {
		membershipRepository, membershipErr := memberstore.NewRepository(ctx, membershipDB)
		if membershipErr != nil {
			return nil, membershipErr
		}
		membershipHistory = membershipAuditReader{repository: membershipRepository}
	}
	query, err := accountaudit.NewWithAuditSources(history, allocationRepository, profileHistory, membershipHistory)
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
	module, err := buildAccountAuditModule(ctx, db, db, nil, authorizer)
	if err != nil {
		return nil, err
	}
	modules := kernelmodule.NewRegistry()
	if err = module.Register(modules); err != nil {
		return nil, err
	}
	return buildIsolatedApplicationHTTPServer(modules.Routes(), routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer}, registry.Timeout), nil
}
