package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	billing "task-processor/internal/commercial/billing"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	resourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	zitadelmembership "task-processor/internal/integration/zitadel/membership"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/organization/membership"
	memberhttp "task-processor/internal/organization/membership/httpapi"
	"task-processor/internal/storecenter"
)

const memberResourcesBase = "/api/v1/account/organization/resources/member-resources"
const memberResourcesModuleName = "account-member-resources"

type memberPositionRepository interface {
	orgresource.MemberAllocationRepository
	ListPositions(context.Context, string) ([]orgresource.MemberResourcePosition, error)
	ReadTransferReplay(context.Context, orgresource.MemberResourceTransfer) (orgresource.MemberResourceTransferResult, bool, error)
}
type memberStoreDirectory interface {
	AssignedStoreCounts(context.Context, string, []string) (map[string]int64, error)
	ListMemberStores(context.Context, string, string, storecenter.StoreListQuery) (storecenter.StorePage, error)
	ReadMemberStoreGrant(context.Context, string, string, string) (storecenter.MemberStoreGrantView, error)
	SetMemberGrant(context.Context, storecenter.MemberStoreGrantCommand) error
	ReadMemberGrantReceipt(context.Context, storecenter.MemberStoreGrantCommand) (storecenter.MemberStoreGrantView, bool, error)
}
type memberDataPrices interface {
	billing.OfferCatalog
	billing.QuoteEngine
	ListDataRowOffers(context.Context) ([]billing.Offer, error)
}
type memberResourcesModule struct {
	positions memberPositionRepository
	service   *orgresource.MemberAllocationService
	gate      memberResourceGate
	stores    memberStoreDirectory
	prices    memberDataPrices
}

func buildMemberResourcesModule(ctx context.Context, cfg *config.Config, resources, stores *gorm.DB, deps MembershipDependencies, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	if cfg == nil || resources == nil || authorizer == nil {
		return nil, orgresource.ErrInvalidInput
	}
	if err := resourceadapter.VerifyRuntimePermissions(ctx, resources); err != nil {
		return nil, err
	}
	positions, err := resourceadapter.NewGormMemberAllocationRepository(resources, resourceadapter.TransactionConfig{})
	if err != nil {
		return nil, err
	}
	directory, err := zitadelmembership.NewClient(deps.ProviderOrigin, deps.ReadToken, cfg.ListingKit.Zitadel.ProjectID, nil)
	if err != nil {
		return nil, err
	}
	gate := memberResourceGate{base: memberPointLimitAuthorizer{authorizer: authorizer, directory: directory, projectID: cfg.ListingKit.Zitadel.ProjectID}}
	service, err := orgresource.NewMemberAllocationService(positions, gate)
	if err != nil {
		return nil, err
	}
	prices, err := commercialstore.New(resources)
	if err != nil {
		return nil, err
	}
	module := memberResourcesModule{positions: positions, service: service, gate: gate, prices: prices}
	if stores != nil {
		native, err := storecenter.NewMemberScopedStoreRepository(stores, currentStoreMemberAuthorizer{authorizer: authorizer})
		if err != nil {
			return nil, err
		}
		module.stores = native
	}
	return module, ctx.Err()
}
func (memberResourcesModule) Name() string { return memberResourcesModuleName }
func (m memberResourcesModule) Enabled(cfg *config.Config) bool {
	return m.service != nil && cfg != nil && cfg.Workbench.Enabled
}
func (m memberResourcesModule) Register(reg *kernelmodule.Registry) error {
	if m.service == nil {
		return orgresource.ErrInvalidInput
	}
	reg.AddRoutes(m.routes()...)
	return nil
}
func (m memberResourcesModule) routes() []httproute.Descriptor {
	route := func(method, path string, write bool, handler gin.HandlerFunc) httproute.Descriptor {
		permission := authz.PermissionWorkbenchOrganizationMemberRead
		target := memberhttp.ResolveOrganizationTarget
		if write {
			permission = authz.PermissionWorkbenchOrganizationMemberManage
			target = memberhttp.ResolveMutationTarget
		}
		return httproute.Descriptor{Method: method, Path: memberResourcesBase + path, Module: memberResourcesModuleName, Permission: permission, AuthPolicy: httproute.AuthPolicyCurrentIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, OrganizationTargetResolver: target, RejectUnreadRequestBody: true, RequestTimeout: 15 * time.Second, Handler: handler}
	}
	return []httproute.Descriptor{route("GET", "", false, m.list), route("GET", "/data-prices", false, m.dataPrices), route("POST", "/data-quotes", true, m.dataQuote), route("POST", "/members/:member_id/transfers", true, m.transfer), route("GET", "/members/:member_id/stores", false, m.memberStores), route("GET", "/members/:member_id/stores/:store_id/grant", false, m.readGrant), route("PUT", "/members/:member_id/stores/:store_id/grant", true, m.setGrant)}
}

type memberResourceGate struct{ base memberPointLimitAuthorizer }

func (g memberResourceGate) identity(ctx context.Context, org string, write bool) (authidentity.AuthenticatedIdentity, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || g.base.authorizer == nil || id.TenantID != org || id.EffectiveOrganizationID != org || !authidentity.IsBoundedIdentifier(id.EffectiveMemberID) {
		return id, orgresource.ErrForbidden
	}
	grant := false
	for _, current := range id.OrganizationGrants {
		if current.OrganizationID == org && current.AuthorizationID == id.EffectiveMemberID {
			grant = true
			break
		}
	}
	permission := authz.PermissionWorkbenchOrganizationMemberRead
	if write {
		permission = authz.PermissionWorkbenchOrganizationMemberManage
	}
	if !grant || !authz.AllowedOrganization(ctx, g.base.authorizer, id.UserID, id.EffectiveOrganizationID, id.Roles, permission) || (write && !g.base.authorizer.IsTenantAdmin(id.UserID, id.Roles)) {
		return id, orgresource.ErrForbidden
	}
	return id, nil
}
func (g memberResourceGate) AuthorizeMemberResourceTransfer(ctx context.Context, p orgresource.Principal, c orgresource.MemberResourceTransfer) error {
	id, err := g.identity(ctx, c.OrganizationID, true)
	if err != nil || p.ID != id.UserID {
		return orgresource.ErrForbidden
	}
	if c.Action == orgresource.MemberResourceReclaim {
		return nil
	}
	return g.base.authorize(ctx, p, c.OrganizationID, c.MemberID, authz.PermissionWorkbenchOrganizationMemberManage)
}
func (g memberResourceGate) AuthorizeMemberResourceRead(ctx context.Context, p orgresource.Principal, org, member string) error {
	id, err := g.identity(ctx, org, false)
	if err != nil || p.ID != id.UserID || (!g.base.authorizer.IsTenantAdmin(id.UserID, id.Roles) && id.EffectiveMemberID != member) {
		return orgresource.ErrForbidden
	}
	return nil
}
func (m memberResourcesModule) identity(c *gin.Context, write bool) (authidentity.AuthenticatedIdentity, bool) {
	target := memberhttp.ResolveOrganizationTarget
	if write {
		target = memberhttp.ResolveMutationTarget
	}
	org, err := target(c.Request)
	if err != nil {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return authidentity.AuthenticatedIdentity{}, false
	}
	id, err := m.gate.identity(c.Request.Context(), org, write)
	if err != nil {
		writeMemberResourceError(c, err)
		return id, false
	}
	return id, true
}

type memberPositionView struct {
	Free     string `json:"free"`
	Reserved string `json:"reserved"`
	Consumed string `json:"consumed"`
	Version  string `json:"version"`
	Recorded bool   `json:"recorded"`
}

func positionView(p orgresource.MemberResourcePosition) memberPositionView {
	return memberPositionView{Free: strconv.FormatInt(p.Free, 10), Reserved: strconv.FormatInt(p.Reserved, 10), Consumed: strconv.FormatInt(p.Consumed, 10), Version: strconv.FormatInt(p.Version, 10), Recorded: p.Version > 0}
}

type memberResourceDirectoryEntry struct {
	MemberID    string             `json:"memberId"`
	UserID      string             `json:"userId"`
	DisplayName string             `json:"displayName"`
	LoginName   string             `json:"loginName"`
	State       string             `json:"state"`
	Roles       []string           `json:"roles"`
	StoreCount  *string            `json:"storeCount"`
	Periods     memberPositionView `json:"periods"`
	DataRows    memberPositionView `json:"dataRows"`
}

func (m memberResourcesModule) list(c *gin.Context) {
	id, ok := m.identity(c, false)
	if !ok {
		return
	}
	if !emptyResourceRead(c) {
		return
	}
	org := id.EffectiveOrganizationID
	page, err := m.gate.base.directory.List(c.Request.Context(), org, membership.PageRequest{Limit: 100})
	if err != nil || page.Total != len(page.Items) || len(page.Items) > 100 {
		writeMemberResourceError(c, orgresource.ErrBalanceUnavailable)
		return
	}
	positions, err := m.positions.ListPositions(c.Request.Context(), org)
	if err != nil {
		writeMemberResourceError(c, err)
		return
	}
	admin := m.gate.base.authorizer.IsTenantAdmin(id.UserID, id.Roles)
	entries := map[string]*memberResourceDirectoryEntry{}
	for _, member := range page.Items {
		if member.OrganizationID != org || member.ProjectID != m.gate.base.projectID || !authidentity.IsBoundedIdentifier(member.ID) || entries[member.ID] != nil {
			writeMemberResourceError(c, orgresource.ErrBalanceUnavailable)
			return
		}
		if !admin && member.ID != id.EffectiveMemberID {
			continue
		}
		entries[member.ID] = &memberResourceDirectoryEntry{MemberID: member.ID, UserID: member.UserID, DisplayName: member.DisplayName, LoginName: member.LoginName, State: member.State, Roles: append([]string{}, member.Roles...), Periods: positionView(orgresource.MemberResourcePosition{}), DataRows: positionView(orgresource.MemberResourcePosition{})}
	}
	for _, p := range positions {
		if !admin && p.MemberID != id.EffectiveMemberID {
			continue
		}
		entry := entries[p.MemberID]
		if entry == nil {
			entry = &memberResourceDirectoryEntry{MemberID: p.MemberID, State: "departed", Roles: []string{}, Periods: positionView(orgresource.MemberResourcePosition{}), DataRows: positionView(orgresource.MemberResourcePosition{})}
			entries[p.MemberID] = entry
		}
		if p.ResourceType == orgresource.ResourceStoreRenewalPeriod {
			entry.Periods = positionView(p)
		} else if p.ResourceType == orgresource.ResourceDataRow {
			entry.DataRows = positionView(p)
		} else {
			writeMemberResourceError(c, orgresource.ErrBalanceUnavailable)
			return
		}
	}
	if len(entries) > 100 {
		writeMemberResourceError(c, orgresource.ErrBalanceUnavailable)
		return
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if m.stores != nil {
		counts, err := m.stores.AssignedStoreCounts(c.Request.Context(), org, keys)
		if err != nil {
			writeMemberResourceError(c, err)
			return
		}
		for _, key := range keys {
			value, exists := counts[key]
			if !exists || value < 0 {
				writeMemberResourceError(c, orgresource.ErrBalanceUnavailable)
				return
			}
			number := strconv.FormatInt(value, 10)
			entries[key].StoreCount = &number
		}
	}
	result := make([]memberResourceDirectoryEntry, 0, len(keys))
	for _, key := range keys {
		result = append(result, *entries[key])
	}
	writePointLimitJSON(c, 200, gin.H{"schemaVersion": "member-resource-directory-v1", "organizationId": org, "observedAt": time.Now().UTC(), "members": result})
}

type memberTransferInput struct {
	ResourceType    string `json:"resourceType"`
	Action          string `json:"action"`
	Quantity        string `json:"quantity"`
	ExpectedVersion string `json:"expectedVersion"`
	QuoteID         string `json:"quoteId,omitempty"`
}

func (m memberResourcesModule) transfer(c *gin.Context) {
	id, ok := m.identity(c, true)
	if !ok {
		return
	}
	var input memberTransferInput
	if err := decodeMemberResourceBody(c.Request, &input); err != nil {
		writeMemberResourceError(c, err)
		return
	}
	quantity, err := resourceInteger(input.Quantity, true)
	version, vErr := resourceInteger(input.ExpectedVersion, false)
	member, key := c.Param("member_id"), c.GetHeader("Idempotency-Key")
	if err != nil || vErr != nil || !authidentity.IsBoundedIdentifier(member) || !authidentity.IsBoundedIdentifier(key) || len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	action := orgresource.MemberResourceAllocate
	if input.Action == "reclaim" {
		action = orgresource.MemberResourceReclaim
	} else if input.Action != "allocate" {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	command := orgresource.MemberResourceTransfer{OrganizationID: id.EffectiveOrganizationID, MemberID: member, ActorID: id.UserID, OperationID: key, ResourceType: orgresource.ResourceType(input.ResourceType), Action: action, Quantity: quantity, ExpectedVersion: version, QuoteID: input.QuoteID}
	if !orgresource.ValidMemberResourceTransfer(command) {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	result, replayed, err := m.positions.ReadTransferReplay(c.Request.Context(), command)
	if err != nil {
		writeMemberResourceError(c, err)
		return
	}
	if !replayed {
		if command.ResourceType == orgresource.ResourceDataRow && action == orgresource.MemberResourceAllocate {
			if m.prices == nil || !authidentity.IsBoundedIdentifier(command.QuoteID) {
				writeMemberResourceError(c, billing.ErrOfferUnavailable)
				return
			}
			quote, err := m.prices.ReadQuote(c.Request.Context(), command.OrganizationID, command.QuoteID)
			if err != nil || quote.OrganizationID != command.OrganizationID || quote.ProductKind != billing.ProductDataRow || quote.ResourceType != orgresource.ResourceDataRow || quote.ResourceQuantity != quantity || !time.Now().Before(quote.ExpiresAt) {
				writeMemberResourceError(c, billing.ErrQuoteExpired)
				return
			}
		} else if input.QuoteID != "" {
			writeMemberResourceError(c, orgresource.ErrInvalidInput)
			return
		}
		result, err = m.service.Transfer(c.Request.Context(), orgresource.Principal{ID: id.UserID, Kind: orgresource.PrincipalTenantHuman}, command)
		if err != nil {
			writeMemberResourceError(c, err)
			return
		}
	}
	writePointLimitJSON(c, 200, gin.H{"organizationId": id.EffectiveOrganizationID, "memberId": member, "resourceType": input.ResourceType, "operationId": key, "position": positionView(result.Position), "unallocated": strconv.FormatInt(result.Unallocated, 10), "allocated": strconv.FormatInt(result.Allocated, 10), "grossCredit": strconv.FormatInt(result.GrossCredit, 10), "debtRepaid": strconv.FormatInt(result.DebtRepaid, 10), "netCredit": strconv.FormatInt(result.NetCredit, 10), "replayed": result.Replayed})
}
func emptyResourceRead(c *gin.Context) bool {
	if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery || c.Request.Body != nil {
		if c.Request.Body != nil {
			body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
			if err != nil || len(body) > 0 {
				writeMemberResourceError(c, orgresource.ErrInvalidInput)
				return false
			}
		}
		if c.Request.URL.RawQuery != "" || c.Request.URL.ForceQuery {
			writeMemberResourceError(c, orgresource.ErrInvalidInput)
			return false
		}
	}
	return true
}
func resourceInteger(raw string, positive bool) (int64, error) {
	if raw == "" || len(raw) > 19 || strings.TrimSpace(raw) != raw || (len(raw) > 1 && raw[0] == '0') {
		return 0, orgresource.ErrInvalidInput
	}
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, orgresource.ErrInvalidInput
		}
	}
	number, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || number < 0 || (positive && number == 0) {
		return 0, orgresource.ErrInvalidInput
	}
	return number, nil
}
func decodeMemberResourceBody(r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || r.URL.RawQuery != "" || r.URL.ForceQuery || r.Body == nil {
		return orgresource.ErrInvalidInput
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	if err != nil || len(data) > 8192 || len(data) == 0 {
		return orgresource.ErrInvalidInput
	}
	if !utf8.Valid(data) {
		return orgresource.ErrInvalidInput
	}
	object := json.NewDecoder(strings.NewReader(string(data)))
	token, err := object.Token()
	if err != nil || token != json.Delim('{') {
		return orgresource.ErrInvalidInput
	}
	seen := map[string]bool{}
	for object.More() {
		token, err := object.Token()
		if err != nil {
			return orgresource.ErrInvalidInput
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return orgresource.ErrInvalidInput
		}
		seen[key] = true
		var value json.RawMessage
		if err := object.Decode(&value); err != nil {
			return orgresource.ErrInvalidInput
		}
	}
	if token, err := object.Token(); err != nil || token != json.Delim('}') {
		return orgresource.ErrInvalidInput
	}
	var extra any
	if object.Decode(&extra) != io.EOF {
		return orgresource.ErrInvalidInput
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return orgresource.ErrInvalidInput
	}
	return nil
}
func writeMemberResourceError(c *gin.Context, err error) {
	status, code := 503, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, orgresource.ErrForbidden):
		status, code = 403, "FORBIDDEN"
	case errors.Is(err, orgresource.ErrInvalidInput):
		status, code = 400, "INVALID_REQUEST"
	case errors.Is(err, orgresource.ErrInsufficientBalance):
		status, code = 409, "RESOURCE_INSUFFICIENT_BALANCE"
	case errors.Is(err, orgresource.ErrResourceDebtOutstanding):
		status, code = 409, "RESOURCE_DEBT_OUTSTANDING"
	case errors.Is(err, orgresource.ErrMemberResourceVersionConflict) || errors.Is(err, orgresource.ErrIdempotencyKeyConflict) || errors.Is(err, storecenter.ErrVersionConflict) || errors.Is(err, storecenter.ErrAlreadyExists):
		status, code = 409, "CONFLICT"
	case errors.Is(err, billing.ErrOfferUnavailable):
		code = "DATA_PRICE_UNAVAILABLE"
	case errors.Is(err, billing.ErrQuoteExpired):
		status, code = 409, "QUOTE_EXPIRED"
	case errors.Is(err, storecenter.ErrNotFound):
		status, code = 404, "STORE_NOT_FOUND"
	}
	writePointLimitJSON(c, status, gin.H{"code": code, "message": "Member resource request could not be completed", "requestId": "", "fieldErrors": []any{}})
}
