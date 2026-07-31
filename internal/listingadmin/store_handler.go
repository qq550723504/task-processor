package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type StoreHandler struct {
	repo StoreRepository
}

const platformStoreAccessContextKey = "listingkit.platform_store_access"

func NewStoreHandler(repo StoreRepository) *StoreHandler {
	return &StoreHandler{repo: repo}
}

// MarkPlatformStoreAccess records that the route middleware has granted the
// request platform-store permission. It supports configured ZITADEL platform
// roles instead of relying only on the built-in role names.
func MarkPlatformStoreAccess(c *gin.Context) {
	if c != nil {
		c.Set(platformStoreAccessContextKey, true)
	}
}

func (h *StoreHandler) ListStores(c *gin.Context) {
	scope := requestListScope(c)
	if hasPlatformStoreAccess(c) {
		// The platform store management route is already protected by the
		// platform-admin permission. Its list must span every tenant instead of
		// inheriting the current user's tenant from X-Tenant-ID.
		scope.TenantID = 0
		scope.OwnerUserID = ""
	}
	query := applyListQueryScope(&StoreQuery{
		Name:        strings.TrimSpace(c.Query("name")),
		Username:    strings.TrimSpace(c.Query("username")),
		ShopType:    strings.TrimSpace(c.Query("shopType")),
		Region:      strings.TrimSpace(c.Query("region")),
		Platform:    strings.TrimSpace(c.Query("platform")),
		SKUGenerate: strings.TrimSpace(c.Query("skuGenerateStrategy")),
		PriceType:   strings.TrimSpace(c.Query("priceType")),
	}, scope)
	var ok bool
	query.EnableAutoListing, ok = queryBoolPtrStrict(c, "enableAutoListing", "invalid_enable_auto_listing")
	if !ok {
		return
	}
	query.EnableAutoLogin, ok = queryBoolPtrStrict(c, "enableAutoLogin", "invalid_enable_auto_login")
	if !ok {
		return
	}
	query.EnableDraft, ok = queryBoolPtrStrict(c, "enableDraft", "invalid_enable_draft")
	if !ok {
		return
	}
	query.EnableAutoPrice, ok = queryBoolPtrStrict(c, "enableAutoPrice", "invalid_enable_auto_price")
	if !ok {
		return
	}
	query.EnableRebargain, ok = queryBoolPtrStrict(c, "enableRebargain", "invalid_enable_rebargain")
	if !ok {
		return
	}
	query.Status, ok = queryInt16PtrStrict(c, "status", "invalid_status")
	if !ok {
		return
	}
	query.Expired, ok = queryBoolPtrStrict(c, "expired", "invalid_expired")
	if !ok {
		return
	}

	page, err := h.repo.ListStores(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "store_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *StoreHandler) GetStore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	store, err := h.repo.GetStore(requestIdentityContext(c), adminStoreTenantID(c), id)
	if err != nil {
		writeStoreError(c, err, "store_get_failed")
		return
	}
	c.JSON(http.StatusOK, store)
}

func (h *StoreHandler) CreateStore(c *gin.Context) {
	var req Store
	if !bindAndValidateJSON(c, &req, "invalid_store", func(value *Store) {
		userID := requestUserID(c)
		value.TenantID = requestTenantID(c)
		value.OwnerUserID = userID
		value.CreatedBy = userID
		value.UpdatedBy = userID
	}, func(value *Store) error {
		return validateStore(value, true)
	}) {
		return
	}
	store, err := h.repo.CreateStore(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "store_create_failed", err)
		return
	}
	c.JSON(http.StatusCreated, store)
}

func (h *StoreHandler) UpdateStore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req Store
	if !bindAndValidateJSON(c, &req, "invalid_store", func(value *Store) {
		userID := requestUserID(c)
		value.ID = id
		value.TenantID = requestTenantID(c)
		value.OwnerUserID = userID
		value.UpdatedBy = userID
	}, func(value *Store) error {
		return validateStore(value, false)
	}) {
		return
	}
	if hasPlatformStoreAccess(c) {
		// Store IDs are globally unique. Let platform administrators update a
		// store from any tenant while regular users remain tenant-scoped.
		req.TenantID = 0
	}
	store, err := h.repo.UpdateStore(requestIdentityContext(c), &req)
	if err != nil {
		writeStoreError(c, err, "store_update_failed")
		return
	}
	c.JSON(http.StatusOK, store)
}

func (h *StoreHandler) UpdateStoreStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int16  `json:"status"`
		Remark string `json:"remark"`
	}
	if !bindJSON(c, &req) {
		return
	}
	store, err := h.repo.UpdateStoreStatus(requestIdentityContext(c), adminStoreTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeStoreError(c, err, "store_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, store)
}

func (h *StoreHandler) DeleteStore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteStore(requestIdentityContext(c), adminStoreTenantID(c), id); err != nil {
		writeStoreError(c, err, "store_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *StoreHandler) ListDeletedStores(c *gin.Context) {
	items, err := h.repo.ListDeletedStores(requestIdentityContext(c), adminStoreTenantID(c))
	if err != nil {
		writeInternalHandlerError(c, "store_deleted_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *StoreHandler) RestoreStore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	store, err := h.repo.RestoreStore(requestIdentityContext(c), adminStoreTenantID(c), id)
	if err != nil {
		writeStoreError(c, err, "store_restore_failed")
		return
	}
	c.JSON(http.StatusOK, store)
}

func (h *StoreHandler) PermanentlyDeleteStore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.PermanentlyDeleteStore(requestIdentityContext(c), adminStoreTenantID(c), id); err != nil {
		writeStoreError(c, err, "store_permanent_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *StoreHandler) ExtendStoreValidity(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	days, ok := queryPositiveInt(c, "days", 30, "invalid_days")
	if !ok {
		return
	}
	store, err := h.repo.ExtendStoreValidity(requestIdentityContext(c), adminStoreTenantID(c), id, days)
	if err != nil {
		writeStoreError(c, err, "store_validity_extend_failed")
		return
	}
	c.JSON(http.StatusOK, store)
}

func (h *StoreHandler) ListSimpleStores(c *gin.Context) {
	scope := requestListScope(c)
	query := applyListQueryScope(&StoreQuery{}, scope)
	query.Page = 1
	query.PageSize = 200
	page, err := h.repo.ListStores(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "store_list_failed", err)
		return
	}
	items := make([]gin.H, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, gin.H{"id": item.ID, "name": item.Name, "platform": item.Platform, "region": item.Region})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func validateStore(store *Store, requirePassword bool) error {
	switch {
	case store.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(store.Name) == "":
		return errors.New("name is required")
	case strings.TrimSpace(store.Username) == "":
		return errors.New("username is required")
	case requirePassword && strings.TrimSpace(store.Password) == "":
		return errors.New("password is required")
	case strings.TrimSpace(store.Platform) == "":
		return errors.New("platform is required")
	case strings.TrimSpace(store.ShopType) == "":
		return errors.New("shopType is required")
	}
	return nil
}

var writeStoreError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrStoreNotFound, status: http.StatusNotFound, errorCode: "store_not_found"},
)

func adminStoreTenantID(c *gin.Context) int64 {
	if hasPlatformStoreAccess(c) {
		return 0
	}
	return requestTenantID(c)
}

func hasPlatformStoreAccess(c *gin.Context) bool {
	if c != nil {
		if value, ok := c.Get(platformStoreAccessContextKey); ok {
			if allowed, ok := value.(bool); ok && allowed {
				return true
			}
		}
	}
	return requestHasPlatformAdminAccess(requestIdentityContext(c))
}
