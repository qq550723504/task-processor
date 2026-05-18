package sheinlogin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Health(c.Request.Context()))
}

func (h *Handler) ListAccounts(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	items, err := h.svc.ListAccounts(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *Handler) Login(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	var req LoginRequest
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	result, err := h.svc.Login(c.Request.Context(), tenantID, storeID, req)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	statusCode := http.StatusOK
	if result.WaitingForVerifyCode {
		statusCode = http.StatusAccepted
	}
	c.JSON(statusCode, result)
}

func (h *Handler) Status(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	status, err := h.svc.Status(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

func (h *Handler) ListWarehouses(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	items, err := h.svc.ListWarehouses(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *Handler) SubmitVerifyCode(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	var req VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "verify code is required"})
		return
	}
	if err := h.svc.SubmitVerifyCode(c.Request.Context(), tenantID, storeID, req.Code, req.ExpireSeconds); err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) CancelVerifyCodeWait(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	cancelled, err := h.svc.CancelVerifyCodeWait(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "cancelled": cancelled})
}

func (h *Handler) ClearCookie(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	if err := h.svc.ClearCookie(c.Request.Context(), tenantID, storeID); err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) ClearLastFailure(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	if err := h.svc.ClearLastFailure(c.Request.Context(), tenantID, storeID); err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) GetLastFailure(c *gin.Context) {
	tenantID, ok := requireTenantID(c)
	if !ok {
		return
	}
	storeID, ok := parseStoreID(c)
	if !ok {
		return
	}
	detail, err := h.svc.GetLastFailureDetail(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(statusCodeForTenantScopedError(err), gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

func parseStoreID(c *gin.Context) (int64, bool) {
	value := c.Param("store_id")
	storeID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || storeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid store_id"})
		return 0, false
	}
	return storeID, true
}

func requireTenantID(c *gin.Context) (int64, bool) {
	tenantID, err := requestTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": err.Error()})
		return 0, false
	}
	return tenantID, true
}

func statusCodeForTenantScopedError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "tenant id is required") {
		return http.StatusUnauthorized
	}
	if strings.Contains(message, "account not found for tenant") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
