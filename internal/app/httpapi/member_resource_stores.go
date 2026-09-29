package httpapi

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"io"
	"net/url"
	"strconv"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
	"time"
)

func (m memberResourcesModule) memberStores(c *gin.Context) {
	id, ok := m.identity(c, false)
	if !ok {
		return
	}
	member := c.Param("member_id")
	if err := m.gate.AuthorizeMemberResourceRead(c.Request.Context(), orgresource.Principal{ID: id.UserID, Kind: orgresource.PrincipalTenantHuman}, id.EffectiveOrganizationID, member); err != nil {
		writeMemberResourceError(c, err)
		return
	}
	if m.stores == nil {
		writeMemberResourceError(c, storecenter.ErrDependencyUnavailable)
		return
	}
	query, qErr := url.ParseQuery(c.Request.URL.RawQuery)
	if qErr != nil || c.Request.URL.ForceQuery {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	if c.Request.Body != nil {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
		if err != nil || len(body) > 0 {
			writeMemberResourceError(c, orgresource.ErrInvalidInput)
			return
		}
	}
	page, size := int64(1), int64(20)
	for key, values := range query {
		if len(values) != 1 || (key != "page" && key != "pageSize") {
			writeMemberResourceError(c, orgresource.ErrInvalidInput)
			return
		}
		number, err := resourceInteger(values[0], true)
		if err != nil || number > 1000000 || (key == "pageSize" && number > 100) {
			writeMemberResourceError(c, orgresource.ErrInvalidInput)
			return
		}
		if key == "page" {
			page = number
		} else {
			size = number
		}
	}
	stores, err := m.stores.ListMemberStores(c.Request.Context(), id.EffectiveOrganizationID, member, storecenter.StoreListQuery{Page: int(page), PageSize: int(size)})
	if err != nil {
		writeMemberResourceError(c, err)
		return
	}
	items := make([]gin.H, 0, len(stores.Stores))
	for _, store := range stores.Stores {
		grant, err := m.stores.ReadMemberStoreGrant(c.Request.Context(), id.EffectiveOrganizationID, store.ID(), member)
		if err != nil {
			writeMemberResourceError(c, err)
			return
		}
		items = append(items, gin.H{"id": store.ID(), "name": store.Name(), "recordStatus": store.RecordStatus(), "version": store.Version(), "grantVersion": strconv.FormatInt(grant.Version, 10), "serviceExpiresAt": store.Snapshot().ServiceExpiresAt})
	}
	writePointLimitJSON(c, 200, gin.H{"organizationId": id.EffectiveOrganizationID, "memberId": member, "page": page, "pageSize": size, "total": strconv.FormatInt(stores.Total, 10), "items": items, "observedAt": time.Now().UTC()})
}
func (m memberResourcesModule) readGrant(c *gin.Context) {
	id, ok := m.identity(c, false)
	if !ok {
		return
	}
	if !emptyResourceRead(c) {
		return
	}
	member, store := c.Param("member_id"), c.Param("store_id")
	if !authidentity.IsBoundedIdentifier(member) || uuid.Validate(store) != nil || m.stores == nil {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	grant, err := m.stores.ReadMemberStoreGrant(c.Request.Context(), id.EffectiveOrganizationID, store, member)
	if err != nil {
		writeMemberResourceError(c, err)
		return
	}
	writePointLimitJSON(c, 200, gin.H{"organizationId": id.EffectiveOrganizationID, "memberId": member, "storeId": store, "active": grant.Active, "version": strconv.FormatInt(grant.Version, 10)})
}
func (m memberResourcesModule) setGrant(c *gin.Context) {
	id, ok := m.identity(c, true)
	if !ok {
		return
	}
	member, store, key := c.Param("member_id"), c.Param("store_id"), c.GetHeader("Idempotency-Key")
	if !authidentity.IsBoundedIdentifier(member) || uuid.Validate(store) != nil || uuid.Validate(key) != nil || len(c.Request.Header.Values("Idempotency-Key")) != 1 || m.stores == nil {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	var input struct {
		Active          *bool  `json:"active"`
		ExpectedVersion string `json:"expectedVersion"`
	}
	if err := decodeMemberResourceBody(c.Request, &input); err != nil || input.Active == nil {
		writeMemberResourceError(c, orgresource.ErrInvalidInput)
		return
	}
	version, err := resourceInteger(input.ExpectedVersion, false)
	if err != nil {
		writeMemberResourceError(c, err)
		return
	}
	command := storecenter.MemberStoreGrantCommand{OrganizationID: id.EffectiveOrganizationID, StoreID: store, MemberID: member, OperationID: key, Active: *input.Active, ExpectedVersion: version}
	receipt, found, err := m.stores.ReadMemberGrantReceipt(c.Request.Context(), command)
	if err != nil {
		writeMemberResourceError(c, err)
		return
	}
	if !found && *input.Active {
		if err := m.gate.base.authorize(c.Request.Context(), orgresource.Principal{ID: id.UserID, Kind: orgresource.PrincipalTenantHuman}, id.EffectiveOrganizationID, member, authz.PermissionWorkbenchOrganizationMemberManage); err != nil {
			writeMemberResourceError(c, err)
			return
		}
	}
	if !found {
		if err := m.stores.SetMemberGrant(c.Request.Context(), command); err != nil {
			writeMemberResourceError(c, err)
			return
		}
		receipt, found, err = m.stores.ReadMemberGrantReceipt(c.Request.Context(), command)
		if err != nil || !found {
			writeMemberResourceError(c, storecenter.ErrDependencyUnavailable)
			return
		}
	}
	writePointLimitJSON(c, 200, gin.H{"organizationId": id.EffectiveOrganizationID, "memberId": member, "storeId": store, "operationId": key, "active": receipt.Active, "version": strconv.FormatInt(receipt.Version, 10)})
}
