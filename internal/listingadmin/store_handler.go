package listingadmin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/tenantbridge"
)

type StoreHandler struct {
	repo StoreRepository
}

type storePageResponse = StorePage

func NewStoreHandler(repo StoreRepository) *StoreHandler {
	return &StoreHandler{repo: repo}
}

func (h *StoreHandler) ListStores(c *gin.Context) {
	query := StoreQuery{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestScopedOwnerUserID(c),
		Page:        queryInt(c, "page", queryInt(c, "pageNo", 1)),
		PageSize:    queryInt(c, "page_size", queryInt(c, "pageSize", 20)),
		Name:        strings.TrimSpace(c.Query("name")),
		Username:    strings.TrimSpace(c.Query("username")),
		ShopType:    strings.TrimSpace(c.Query("shopType")),
		Region:      strings.TrimSpace(c.Query("region")),
		Platform:    strings.TrimSpace(c.Query("platform")),
		SKUGenerate: strings.TrimSpace(c.Query("skuGenerateStrategy")),
		PriceType:   strings.TrimSpace(c.Query("priceType")),
	}
	query.EnableAutoListing = queryBoolPtr(c, "enableAutoListing")
	query.EnableAutoLogin = queryBoolPtr(c, "enableAutoLogin")
	query.EnableDraft = queryBoolPtr(c, "enableDraft")
	query.EnableAutoPrice = queryBoolPtr(c, "enableAutoPrice")
	query.EnableRebargain = queryBoolPtr(c, "enableRebargain")
	query.Status = queryInt16Ptr(c, "status")
	query.Expired = queryBoolPtr(c, "expired")

	page, err := h.repo.ListStores(requestIdentityContext(c), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *StoreHandler) GetStore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	store, err := h.repo.GetStore(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeStoreError(c, err, "store_get_failed")
		return
	}
	c.JSON(http.StatusOK, store)
}

func (h *StoreHandler) CreateStore(c *gin.Context) {
	var req Store
	if !bindJSON(c, &req) {
		return
	}
	req.TenantID = requestTenantID(c)
	req.OwnerUserID = requestUserID(c)
	req.CreatedBy = requestUserID(c)
	req.UpdatedBy = requestUserID(c)
	if err := validateStore(&req, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_store", "message": err.Error()})
		return
	}
	store, err := h.repo.CreateStore(requestIdentityContext(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store_create_failed", "message": err.Error()})
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
	if !bindJSON(c, &req) {
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	req.OwnerUserID = requestUserID(c)
	req.UpdatedBy = requestUserID(c)
	if err := validateStore(&req, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_store", "message": err.Error()})
		return
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
	store, err := h.repo.UpdateStoreStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
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
	if err := h.repo.DeleteStore(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeStoreError(c, err, "store_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *StoreHandler) ListDeletedStores(c *gin.Context) {
	items, err := h.repo.ListDeletedStores(requestIdentityContext(c), requestTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store_deleted_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *StoreHandler) RestoreStore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	store, err := h.repo.RestoreStore(requestIdentityContext(c), requestTenantID(c), id)
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
	if err := h.repo.PermanentlyDeleteStore(requestIdentityContext(c), requestTenantID(c), id); err != nil {
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
	days := queryInt(c, "days", 30)
	store, err := h.repo.ExtendStoreValidity(requestIdentityContext(c), requestTenantID(c), id, days)
	if err != nil {
		writeStoreError(c, err, "store_validity_extend_failed")
		return
	}
	c.JSON(http.StatusOK, store)
}

func (h *StoreHandler) ListSimpleStores(c *gin.Context) {
	page, err := h.repo.ListStores(requestIdentityContext(c), StoreQuery{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestScopedOwnerUserID(c),
		Page:        1,
		PageSize:    200,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store_list_failed", "message": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, gin.H{"id": item.ID, "name": item.Name, "platform": item.Platform, "region": item.Region})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func requestTenantID(c *gin.Context) int64 {
	rawTenantID := ""
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant", "tenant-id"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			rawTenantID = value
			break
		}
	}
	if rawTenantID == "" {
		rawTenantID = strings.TrimSpace(c.Query("tenant_id"))
	}
	if rawTenantID == "" {
		return 0
	}
	value, err := tenantbridge.ResolveLegacyTenantID(c.Request.Context(), rawTenantID)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func requestUserID(c *gin.Context) string {
	for _, header := range []string{"X-User-ID", "X-User-Id", "X-User"} {
		if userID := requestUserIDHeader(c.GetHeader(header)); userID != "" {
			return userID
		}
	}
	return requestUserIDHeader(c.Query("user_id"))
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id", "message": "id must be a positive integer"})
		return 0, false
	}
	return id, true
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryBoolPtr(c *gin.Context, key string) *bool {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func queryInt16Ptr(c *gin.Context, key string) *int16 {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return nil
	}
	out := int16(parsed)
	return &out
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

func writeStoreError(c *gin.Context, err error, code string) {
	writeMappedHandlerError(c, err, code,
		handlerErrorRule{match: ErrStoreNotFound, status: http.StatusNotFound, errorCode: "store_not_found"},
	)
}
