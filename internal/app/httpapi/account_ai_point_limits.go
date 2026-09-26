package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	resourceadapter "task-processor/internal/integration/orgresource"
	zitadelmembership "task-processor/internal/integration/zitadel/membership"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/organization/membership"
	memberhttp "task-processor/internal/organization/membership/httpapi"
)

const memberPointLimitBase = "/api/v1/account/organization/resources/member-ai-point-limits"
const memberPointLimitModuleName = "account-member-ai-point-limits"

func buildMemberPointLimitModule(ctx context.Context, cfg *config.Config, commercialOwnerDB *gorm.DB, deps MembershipDependencies, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	if ctx == nil || cfg == nil || commercialOwnerDB == nil || authorizer == nil {
		return nil, orgresource.ErrInvalidInput
	}
	if err := resourceadapter.VerifyRuntimePermissions(ctx, commercialOwnerDB); err != nil {
		return nil, err
	}
	directory, err := zitadelmembership.NewClient(deps.ProviderOrigin, deps.ReadToken, cfg.ListingKit.Zitadel.ProjectID, nil)
	if err != nil {
		return nil, err
	}
	repository, err := resourceadapter.NewGormMemberLimitRepository(commercialOwnerDB, resourceadapter.TransactionConfig{})
	if err != nil {
		return nil, err
	}
	gate := memberPointLimitAuthorizer{authorizer: authorizer, directory: directory, projectID: cfg.ListingKit.Zitadel.ProjectID}
	service, err := orgresource.NewMemberLimitService(repository, gate)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return memberPointLimitModule{service: service, gate: gate}, nil
}

type memberPointLimitModule struct {
	service *orgresource.MemberLimitService
	gate    memberPointLimitAuthorizer
}

func (memberPointLimitModule) Name() string { return memberPointLimitModuleName }
func (m memberPointLimitModule) Enabled(cfg *config.Config) bool {
	return m.service != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m memberPointLimitModule) Register(reg *kernelmodule.Registry) error {
	if m.service == nil {
		return orgresource.ErrInvalidInput
	}
	reg.AddRoutes(m.routes()...)
	return nil
}
func (m memberPointLimitModule) routes() []httproute.Descriptor {
	return []httproute.Descriptor{
		{Method: http.MethodGet, Path: memberPointLimitBase, Module: memberPointLimitModuleName, Permission: authz.PermissionWorkbenchOrganizationMemberRead, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: memberhttp.ResolveOrganizationTarget, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: m.list},
		{Method: http.MethodPut, Path: memberPointLimitBase + "/:member_id", Module: memberPointLimitModuleName, Permission: authz.PermissionWorkbenchOrganizationMemberManage, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: memberhttp.ResolveMutationTarget, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: m.set},
	}
}
func (m memberPointLimitModule) list(c *gin.Context) {
	identity, ok := m.identity(c, false)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
		writePointLimitError(c, orgresource.ErrInvalidInput)
		return
	}
	page, err := m.gate.directory.List(c.Request.Context(), identity.EffectiveOrganizationID, membership.PageRequest{Limit: 100})
	if err != nil || page.Total != len(page.Items) || len(page.Items) > 100 {
		writePointLimitError(c, orgresource.ErrMemberLimitUnavailable)
		return
	}
	items := make([]memberPointLimitDirectoryView, 0, len(page.Items))
	seen := map[string]bool{}
	for _, member := range page.Items {
		if member.State != "active" {
			continue
		}
		if member.ID == "" || seen[member.ID] || member.OrganizationID != identity.EffectiveOrganizationID || member.ProjectID != m.gate.projectID {
			writePointLimitError(c, orgresource.ErrMemberLimitUnavailable)
			return
		}
		seen[member.ID] = true
		value, err := m.service.ReadMonthlyLimit(c.Request.Context(), orgresource.Principal{ID: identity.UserID, Kind: orgresource.PrincipalTenantHuman}, identity.EffectiveOrganizationID, member.ID)
		configured := true
		if errors.Is(err, orgresource.ErrMemberLimitUnavailable) {
			configured = false
			value = orgresource.MemberLimitSnapshot{OrganizationID: identity.EffectiveOrganizationID, MemberID: member.ID, MonthStart: orgresource.AIPointMonthStart(time.Now())}
		} else if err != nil {
			writePointLimitError(c, err)
			return
		}
		roles := append([]string{}, member.Roles...)
		items = append(items, memberPointLimitDirectoryView{memberPointLimitView: pointLimitView(value, configured), DisplayName: member.DisplayName, LoginName: member.LoginName, Roles: roles})
	}
	writePointLimitJSON(c, http.StatusOK, gin.H{"schemaVersion": "member-ai-point-monthly-limit-v1", "organizationId": identity.EffectiveOrganizationID, "resourceType": "ai_point", "timezone": "UTC", "members": items})
}
func (m memberPointLimitModule) set(c *gin.Context) {
	identity, ok := m.identity(c, true)
	if !ok {
		return
	}
	input, err := decodePointLimitCommand(c.Request)
	if err != nil {
		writePointLimitError(c, orgresource.ErrInvalidInput)
		return
	}
	member := c.Param("member_id")
	if !authidentity.IsBoundedIdentifier(member) || len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		writePointLimitError(c, orgresource.ErrInvalidInput)
		return
	}
	input.OrganizationID = identity.EffectiveOrganizationID
	input.MemberID = member
	input.ActorID = identity.UserID
	input.OperationID = c.GetHeader("Idempotency-Key")
	result, err := m.service.SetMonthlyLimit(c.Request.Context(), orgresource.Principal{ID: identity.UserID, Kind: orgresource.PrincipalTenantHuman}, input)
	if err != nil {
		writePointLimitError(c, err)
		return
	}
	writePointLimitJSON(c, http.StatusOK, pointLimitView(result, true))
}

func (m memberPointLimitModule) identity(c *gin.Context, write bool) (authidentity.AuthenticatedIdentity, bool) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || identity.EffectiveMemberID == "" {
		writePointLimitJSON(c, http.StatusUnauthorized, gin.H{"code": "AUTHENTICATION_REQUIRED", "message": "Current identity required", "requestId": "", "fieldErrors": []any{}})
		return identity, false
	}
	resolve := memberhttp.ResolveOrganizationTarget
	permission := authz.PermissionWorkbenchOrganizationMemberRead
	if write {
		resolve = memberhttp.ResolveMutationTarget
		permission = authz.PermissionWorkbenchOrganizationMemberManage
	}
	org, err := resolve(c.Request)
	if err != nil {
		writePointLimitError(c, orgresource.ErrInvalidInput)
		return identity, false
	}
	if identity.TenantID != org || identity.EffectiveOrganizationID != org || m.gate.authorizer == nil || !m.gate.authorizer.Authorize(identity.UserID, identity.Roles, permission) {
		writePointLimitError(c, orgresource.ErrForbidden)
		return identity, false
	}
	return identity, true
}

func decodePointLimitCommand(r *http.Request) (orgresource.SetMemberLimitExecution, error) {
	const maxBody = 8 << 10
	invalid := func() (orgresource.SetMemberLimitExecution, error) {
		return orgresource.SetMemberLimitExecution{}, orgresource.ErrInvalidInput
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || r.ContentLength <= 0 || r.ContentLength > maxBody || len(r.TransferEncoding) > 0 || r.URL.RawQuery != "" || r.URL.ForceQuery {
		return invalid()
	}
	var body struct {
		Target          string `json:"target"`
		ExpectedVersion string `json:"expectedVersion"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		return invalid()
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) {
		return invalid()
	}
	parse := func(value string) (int64, error) {
		if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
			return 0, orgresource.ErrInvalidInput
		}
		return strconv.ParseInt(value, 10, 64)
	}
	target, err := parse(body.Target)
	if err != nil || target < 0 {
		return invalid()
	}
	version, err := parse(body.ExpectedVersion)
	if err != nil || version < 0 {
		return invalid()
	}
	return orgresource.SetMemberLimitExecution{Target: target, ExpectedVersion: version}, nil
}

type memberPointLimitView struct {
	OrganizationID string `json:"organizationId"`
	MemberID       string `json:"memberId"`
	Configured     bool   `json:"configured"`
	MonthlyLimit   string `json:"monthlyLimit"`
	Reserved       string `json:"reserved"`
	Consumed       string `json:"consumed"`
	Remaining      string `json:"remaining"`
	Version        string `json:"version"`
	MonthStart     string `json:"monthStart"`
	MonthEnd       string `json:"monthEnd"`
}

type memberPointLimitDirectoryView struct {
	memberPointLimitView
	DisplayName string   `json:"displayName"`
	LoginName   string   `json:"loginName"`
	Roles       []string `json:"roles"`
}

func pointLimitView(v orgresource.MemberLimitSnapshot, configured bool) memberPointLimitView {
	return memberPointLimitView{OrganizationID: v.OrganizationID, MemberID: v.MemberID, Configured: configured, MonthlyLimit: strconv.FormatInt(v.MonthlyLimit, 10), Reserved: strconv.FormatInt(v.Reserved, 10), Consumed: strconv.FormatInt(v.Consumed, 10), Remaining: strconv.FormatInt(v.MonthlyLimit-v.Reserved-v.Consumed, 10), Version: strconv.FormatInt(v.Version, 10), MonthStart: v.MonthStart.UTC().Format(time.RFC3339), MonthEnd: v.MonthStart.UTC().AddDate(0, 1, 0).Format(time.RFC3339)}
}
func writePointLimitJSON(c *gin.Context, status int, value any) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(status, value)
}
func writePointLimitError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, orgresource.ErrForbidden):
		status, code = http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, orgresource.ErrInvalidInput):
		status, code = http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, orgresource.ErrMemberLimitExceeded):
		status, code = http.StatusConflict, "CONSUMED_FLOOR"
	case errors.Is(err, orgresource.ErrIdempotencyKeyConflict) || errors.Is(err, orgresource.ErrMemberLimitVersionConflict):
		status, code = http.StatusConflict, "CONFLICT"
	}
	writePointLimitJSON(c, status, gin.H{"code": code, "message": "Member AI point limit request could not be completed", "requestId": "", "fieldErrors": []any{}})
}

// This authorizer consumes identity refreshed by the existing live-write route
// boundary and checks the target in the current identity-provider directory.
// No Token allocation is used as permission to spend enterprise AI points.
type memberPointLimitAuthorizer struct {
	authorizer *authz.ListingKitAuthorizer
	directory  membership.Directory
	projectID  string
}

func (a memberPointLimitAuthorizer) AuthorizeMemberLimitRead(ctx context.Context, p orgresource.Principal, org, member string) error {
	return a.authorize(ctx, p, org, member, authz.PermissionWorkbenchOrganizationMemberRead)
}
func (a memberPointLimitAuthorizer) AuthorizeMemberLimitWrite(ctx context.Context, p orgresource.Principal, org, member string) error {
	return a.authorize(ctx, p, org, member, authz.PermissionWorkbenchOrganizationMemberManage)
}
func (a memberPointLimitAuthorizer) authorize(ctx context.Context, p orgresource.Principal, org, member, permission string) error {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || a.authorizer == nil || a.directory == nil || a.projectID == "" || p.Kind != orgresource.PrincipalTenantHuman || identity.UserID != p.ID || identity.TenantID != org || identity.EffectiveOrganizationID != org || identity.EffectiveMemberID == "" || !a.authorizer.Authorize(identity.UserID, identity.Roles, permission) {
		return orgresource.ErrForbidden
	}
	target, err := a.directory.Read(ctx, org, member)
	if err != nil || target.ID != member || target.OrganizationID != org || target.ProjectID != a.projectID || target.UserID == "" || target.State != "active" {
		return orgresource.ErrForbidden
	}
	return nil
}
