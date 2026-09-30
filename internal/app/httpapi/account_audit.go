package httpapi

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sort"
	aistore "task-processor/internal/aicapability/store"
	"task-processor/internal/app/accountaudit"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	resourceadapter "task-processor/internal/integration/orgresource"
	accountprofilestore "task-processor/internal/integration/persistence/accountprofile"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
	store "task-processor/internal/integration/persistence/sourceaccountregistry"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	membership "task-processor/internal/organization/membership"
	registry "task-processor/internal/sourceaccountregistry"
	"task-processor/internal/workbenchcontext"
)

const accountAuditPath = "/api/v1/account/audit"

var accountAuditScope = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var accountAuditLimit = regexp.MustCompile(`^[1-9][0-9]{0,2}$`)

type accountAuditModule struct{ query *accountaudit.Query }

type invocationAuditSources map[string]*gorm.DB

type aiUsageAuditReader struct{ sources invocationAuditSources }

func (r aiUsageAuditReader) ListObservedAIUsageAudit(ctx context.Context, org string, limit int, after *accountaudit.AuditPosition) (accountaudit.UsageAuditPage, error) {
	if limit < 1 || limit > registry.MaxPageLimit {
		return accountaudit.UsageAuditPage{}, registry.ErrInvalid
	}
	var position *aistore.ObservedUsagePosition
	if after != nil {
		position = &aistore.ObservedUsagePosition{At: after.CreatedAt, Key: after.Key}
	}
	page := accountaudit.UsageAuditPage{Items: []accountaudit.UsageAuditEvent{}}
	more := false
	for namespace, db := range r.sources {
		local := position
		read := 0
		for read < limit {
			chunk := limit - read
			if chunk > 50 {
				chunk = 50
			}
			rows, err := aistore.NewGormInvocationRecorder(db).ListObservedUsage(ctx, org, namespace, chunk, local)
			if err != nil {
				return accountaudit.UsageAuditPage{}, err
			}
			read += len(rows.Items)
			for _, row := range rows.Items {
				page.Items = append(page.Items, accountaudit.UsageAuditEvent{OrganizationID: org, EventID: row.Key, MemberID: row.MemberID, InvocationID: row.InvocationID, Quantity: row.Tokens, Time: row.At})
			}
			if rows.Next == nil {
				break
			}
			if len(rows.Items) == 0 {
				return accountaudit.UsageAuditPage{}, registry.ErrUnavailable
			}
			if read == limit {
				more = true
				break
			}
			local = rows.Next
		}
	}
	sort.Slice(page.Items, func(i, j int) bool {
		if !page.Items[i].Time.Equal(page.Items[j].Time) {
			return page.Items[i].Time.After(page.Items[j].Time)
		}
		return page.Items[i].EventID > page.Items[j].EventID
	})
	if len(page.Items) > limit {
		more = true
		page.Items = page.Items[:limit]
	}
	if more && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		page.Next = &accountaudit.AuditPosition{CreatedAt: last.Time, Key: last.EventID}
	}
	return page, nil
}

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

type memberResourceAuditReader struct{ repository orgresource.MemberAuditReader }

func (r memberResourceAuditReader) ListRecentAudit(ctx context.Context, organization string, limit int, actor, operation string, after *accountaudit.AuditPosition) (accountaudit.AdditionalAuditPage, error) {
	var position *orgresource.MemberAuditPosition
	if after != nil {
		id, err := strconv.ParseInt(after.Key, 10, 64)
		if err != nil || id < 1 {
			return accountaudit.AdditionalAuditPage{}, registry.ErrInvalid
		}
		position = &orgresource.MemberAuditPosition{CreatedAt: after.CreatedAt, ID: id}
	}
	rows, err := r.repository.ListMemberAudit(ctx, organization, limit, actor, operation, position)
	if err != nil {
		return accountaudit.AdditionalAuditPage{}, err
	}
	page := accountaudit.AdditionalAuditPage{Items: make([]accountaudit.AdditionalAuditEvent, 0, len(rows.Items))}
	for _, row := range rows.Items {
		if row.OrganizationID != organization || row.ID < 1 {
			return accountaudit.AdditionalAuditPage{}, registry.ErrUnavailable
		}
		eventType, objectType := "account_member_resource.changed", "member_resource"
		if row.Action == "set_member_ai_point_limit" {
			eventType, objectType = "account_member_ai_point_limit.changed", "member_ai_point_limit"
		}
		page.Items = append(page.Items, accountaudit.AdditionalAuditEvent{EventType: eventType, ObjectType: objectType, ObjectReference: row.MemberID, Actor: row.ActorID, Operation: row.Action, Time: row.CreatedAt, Version: row.Version, Key: fmt.Sprintf("%020d", row.ID), RelationReference: row.OperationID, Resource: &accountaudit.ResourceDetail{Type: row.ResourceType, Quantity: strconv.FormatInt(row.Quantity, 10)}})
	}
	if rows.Next != nil {
		page.Next = &accountaudit.AuditPosition{CreatedAt: rows.Next.CreatedAt, Key: fmt.Sprintf("%020d", rows.Next.ID)}
	}
	return page, nil
}

func membershipAuditKey(projectID, actorID, operationKey string) string {
	encode := hex.EncodeToString
	return operationKey + "." + encode([]byte(actorID)) + "." + encode([]byte(projectID))
}

func membershipAuditPosition(after *accountaudit.AuditPosition) (*membership.AuditPosition, error) {
	if after == nil {
		return nil, nil
	}
	parts := strings.Split(after.Key, ".")
	if len(parts) != 3 {
		return nil, registry.ErrInvalid
	}
	decode := hex.DecodeString
	actorBytes, actorErr := decode(parts[1])
	projectBytes, projectErr := decode(parts[2])
	position := &membership.AuditPosition{CreatedAt: after.CreatedAt, OperationKey: parts[0], ActorID: string(actorBytes), ProjectID: string(projectBytes)}
	if actorErr != nil || projectErr != nil || !position.Valid() {
		return nil, registry.ErrInvalid
	}
	return position, nil
}

func (r membershipAuditReader) ListRecentAudit(ctx context.Context, organizationID string, limit int, actor, operation string, after *accountaudit.AuditPosition) (accountaudit.AdditionalAuditPage, error) {
	position, err := membershipAuditPosition(after)
	if err != nil {
		return accountaudit.AdditionalAuditPage{}, err
	}
	items, next, err := r.repository.ListRecentAudit(ctx, organizationID, limit, actor, operation, position)
	if err != nil {
		return accountaudit.AdditionalAuditPage{}, err
	}
	page := accountaudit.AdditionalAuditPage{Items: make([]accountaudit.AdditionalAuditEvent, 0, len(items))}
	for _, item := range items {
		page.Items = append(page.Items, accountaudit.AdditionalAuditEvent{EventType: "organization_membership.changed", Actor: item.ActorID, Time: item.CreatedAt, ObjectType: "organization_member", ObjectReference: item.TargetUserID, Operation: item.Operation, Version: item.Revision, Key: membershipAuditKey(item.ProjectID, item.ActorID, item.OperationKey), RelationReference: item.OperationKey})
	}
	if next != nil {
		page.Next = &accountaudit.AuditPosition{CreatedAt: next.CreatedAt, Key: membershipAuditKey(next.ProjectID, next.ActorID, next.OperationKey)}
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
	modules.AddRoutes(httproute.Descriptor{Method: http.MethodGet, Path: accountAuditPath + "/summary", Module: m.Name(), Permission: authz.PermissionWorkbenchSourceAccountRead, AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: accountOrganizationTarget, RejectUnreadRequestBody: true, RequestTimeout: registry.Timeout, Handler: m.readSummary})
	return nil
}

func (m accountAuditModule) readSummary(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writeAccountAuditError(c, registry.ErrInvalid)
		return
	}
	summary, err := m.query.ReadSummary(c.Request.Context())
	if err != nil {
		writeAccountAuditError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
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
	content := strings.TrimSpace(values.Get("query"))
	if values.Has("query") {
		if content == "" || len(content) > 80 || len(values["query"]) != 1 {
			return accountaudit.Filter{}, registry.ErrInvalid
		}
		for _, ch := range content {
			if unicode.IsControl(ch) {
				return accountaudit.Filter{}, registry.ErrInvalid
			}
		}
	}
	member := values.Get("member")
	if member != "" && (len(values["member"]) != 1 || !accountAuditScope.MatchString(member)) {
		return accountaudit.Filter{}, registry.ErrInvalid
	}
	period := values.Get("period")
	if period != "" && (len(values["period"]) != 1 || period != "7d" && period != "30d" && period != "all") {
		return accountaudit.Filter{}, registry.ErrInvalid
	}
	actor := values.Get("actor")
	if actor != "" && (len(values["actor"]) != 1 || !accountAuditScope.MatchString(actor)) {
		return accountaudit.Filter{}, registry.ErrInvalid
	}
	operation := values.Get("operation")
	if operation != "" && (len(values["operation"]) != 1 || operation != string(registry.OperationRegister) && operation != string(registry.OperationEnable) && operation != string(registry.OperationDisable) && operation != "allocate_member_resource" && operation != "reclaim_member_resource" && operation != "set_member_ai_point_limit" && operation != "update" && operation != "invite" && operation != "role" && operation != "remove") {
		return accountaudit.Filter{}, registry.ErrInvalid
	}
	filter := accountaudit.Filter{ActorSubject: actor, Content: content, MemberID: member, Period: period}
	if operation == "allocate_member_resource" || operation == "reclaim_member_resource" || operation == "set_member_ai_point_limit" {
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
	if len(raw) > 4096 {
		return 0, "", registry.ErrInvalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return 0, "", registry.ErrInvalid
	}
	for key, value := range values {
		if (key != "limit" && key != "cursor" && key != "actor" && key != "operation" && key != "query" && key != "period" && key != "member") || len(value) != 1 || value[0] == "" {
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
	case errors.Is(err, accountaudit.ErrSummaryNotConfigured):
		status, code = 503, "SUMMARY_NOT_CONFIGURED"
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
func buildAccountAuditModule(ctx context.Context, sourceDB, membershipDB, resourceDB *gorm.DB, sources invocationAuditSources, authorizer *authz.ListingKitAuthorizer, membershipProjectID string) (kernelmodule.Module, error) {
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
	profileRepository, err := accountprofilestore.New(sourceDB)
	if err != nil {
		return nil, err
	}
	var profileHistory accountaudit.AdditionalHistory = profileAuditReader{repository: profileRepository}
	var membershipHistory accountaudit.AdditionalHistory
	if membershipDB != nil {
		membershipRepository, membershipErr := memberstore.NewRepository(ctx, membershipDB, membershipProjectID)
		if membershipErr != nil {
			return nil, membershipErr
		}
		membershipHistory = membershipAuditReader{repository: membershipRepository}
	}
	var pointHistory accountaudit.ImagePointHistory
	var memberResourceHistory accountaudit.AdditionalHistory
	if resourceDB != nil {
		if err := resourceadapter.VerifyRuntimePermissions(ctx, resourceDB); err != nil {
			return nil, err
		}
		resourceRepository, resourceErr := resourceadapter.NewGormRepository(resourceDB, resourceadapter.TransactionConfig{})
		err = resourceErr
		if err != nil {
			return nil, err
		}
		pointHistory = resourceRepository
		memberResourceHistory = memberResourceAuditReader{repository: resourceRepository}
	}
	query, err := accountaudit.NewCurrentAuditSources(history, profileHistory, membershipHistory, aiUsageAuditReader{sources: sources}, pointHistory, memberResourceHistory)
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
	module, err := buildAccountAuditModule(ctx, db, nil, nil, nil, authorizer, "")
	if err != nil {
		return nil, err
	}
	modules := kernelmodule.NewRegistry()
	if err = module.Register(modules); err != nil {
		return nil, err
	}
	return buildIsolatedApplicationHTTPServer(modules.Routes(), routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer}, registry.Timeout), nil
}
