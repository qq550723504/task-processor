package listingsubscription

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetCurrentSubscription(c *gin.Context) {
	h.writeSummary(c)
}

func (h *Handler) ListModules(c *gin.Context) {
	modules, err := h.service.ListModules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription_modules_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": modules})
}

func (h *Handler) ListEntitlements(c *gin.Context) {
	h.writeSummary(c)
}

func (h *Handler) UpsertEntitlement(c *gin.Context) {
	moduleCode := strings.TrimSpace(c.Param("module_code"))
	var req EntitlementInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	entitlement, err := h.service.UpsertEntitlement(c.Request.Context(), RequestTenantID(c), moduleCode, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrModuleNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "subscription_module_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "subscription_entitlement_invalid", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, entitlement)
}

func (h *Handler) writeSummary(c *gin.Context) {
	summary, err := h.service.GetSummary(c.Request.Context(), RequestTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription_summary_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func RequestTenantID(c *gin.Context) string {
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant", "tenant-id"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(c.Query("tenant_id")); value != "" {
		return value
	}
	return "default"
}
