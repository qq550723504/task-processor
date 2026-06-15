package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) UpsertSubscriptionEntitlement(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	var req listingsubscription.EntitlementInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_subscription_entitlement", "message": err.Error()})
		return
	}
	entitlement, err := h.subscriptionService.UpsertEntitlementWithAudit(c.Request.Context(), requestTenantID(c), strings.TrimSpace(c.Param("module_code")), req, c.GetHeader("X-User-ID"), "")
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, listingsubscription.ErrModuleNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "subscription_entitlement_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entitlement)
}

func (h *handler) ListPlatformTenantSubscriptions(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	items, err := h.subscriptionService.ListTenantOverviews(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription_tenant_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *handler) ListPlatformSubscriptionPlans(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	items, err := h.subscriptionService.ListPlans(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription_plan_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *handler) UpsertPlatformSubscriptionPlan(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	var req listingsubscription.PlanInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_subscription_plan", "message": err.Error()})
		return
	}
	if pathCode := strings.TrimSpace(c.Param("plan_code")); pathCode != "" {
		req.Code = pathCode
	}
	bundle, err := h.subscriptionService.UpsertPlan(c.Request.Context(), req, c.GetHeader("X-User-ID"))
	if err != nil {
		h.writeSubscriptionPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (h *handler) UpsertPlatformSubscriptionPlanModule(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	planCode := strings.TrimSpace(c.Param("plan_code"))
	moduleCode := strings.TrimSpace(c.Param("module_code"))
	var req listingsubscription.PlanModuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_subscription_plan_module", "message": err.Error()})
		return
	}
	req.ModuleCode = moduleCode
	bundle, err := h.subscriptionService.UpsertPlanModule(c.Request.Context(), planCode, moduleCode, req, c.GetHeader("X-User-ID"))
	if err != nil {
		h.writeSubscriptionPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (h *handler) DeletePlatformSubscriptionPlanModule(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	bundle, err := h.subscriptionService.DeletePlanModule(
		c.Request.Context(),
		strings.TrimSpace(c.Param("plan_code")),
		strings.TrimSpace(c.Param("module_code")),
		c.GetHeader("X-User-ID"),
	)
	if err != nil {
		h.writeSubscriptionPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (h *handler) SetPlatformSubscriptionPlanStatus(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_subscription_plan_status", "message": err.Error()})
		return
	}
	bundle, err := h.subscriptionService.SetPlanActive(c.Request.Context(), strings.TrimSpace(c.Param("plan_code")), req.Active, c.GetHeader("X-User-ID"))
	if err != nil {
		h.writeSubscriptionPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (h *handler) ListPlatformSubscriptionPlanTenants(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	items, err := h.subscriptionService.ListPlanTenants(c.Request.Context(), strings.TrimSpace(c.Param("plan_code")))
	if err != nil {
		h.writeSubscriptionPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *handler) ListPlatformSubscriptionPlanAuditLogs(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	items, err := h.subscriptionService.ListPlanAuditLogs(c.Request.Context(), strings.TrimSpace(c.Param("plan_code")), 50)
	if err != nil {
		h.writeSubscriptionPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *handler) GetPlatformTenantSubscription(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	tenantID := strings.TrimSpace(c.Param("tenant_id"))
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id_required", "message": "tenant id is required"})
		return
	}
	summary, err := h.subscriptionService.GetTenantSummary(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription_summary_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *handler) ApplyPlatformTenantSubscriptionPlan(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	tenantID := strings.TrimSpace(c.Param("tenant_id"))
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id_required", "message": "tenant id is required"})
		return
	}
	var req listingsubscription.PlanApplyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_subscription_plan", "message": err.Error()})
		return
	}
	subscription, err := h.subscriptionService.ApplyPlan(c.Request.Context(), tenantID, req, c.GetHeader("X-User-ID"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, listingsubscription.ErrModuleNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "subscription_plan_apply_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, subscription)
}

func (h *handler) UpsertPlatformTenantSubscriptionEntitlement(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	tenantID := strings.TrimSpace(c.Param("tenant_id"))
	moduleCode := strings.TrimSpace(c.Param("module_code"))
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id_required", "message": "tenant id is required"})
		return
	}
	var req listingsubscription.EntitlementInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_subscription_entitlement", "message": err.Error()})
		return
	}
	entitlement, err := h.subscriptionService.UpsertEntitlementWithAudit(c.Request.Context(), tenantID, moduleCode, req, c.GetHeader("X-User-ID"), "")
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, listingsubscription.ErrModuleNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "subscription_entitlement_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entitlement)
}

func (h *handler) SetPlatformTenantSubscriptionUsage(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	tenantID := strings.TrimSpace(c.Param("tenant_id"))
	moduleCode := strings.TrimSpace(c.Param("module_code"))
	var req listingsubscription.UsageAdjustmentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_subscription_usage", "message": err.Error()})
		return
	}
	if req.PeriodKey == "" {
		req.PeriodKey = strings.TrimSpace(c.Param("period_key"))
	}
	if req.Metric == "" {
		req.Metric = strings.TrimSpace(c.Param("metric"))
	}
	counter, err := h.subscriptionService.SetUsage(c.Request.Context(), tenantID, moduleCode, req, c.GetHeader("X-User-ID"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, listingsubscription.ErrModuleNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "subscription_usage_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, counter)
}

func (h *handler) ListPlatformTenantSubscriptionAuditLogs(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	if !h.requirePlatformSubscriptionAccess(c) {
		return
	}
	items, err := h.subscriptionService.ListAuditLogs(c.Request.Context(), strings.TrimSpace(c.Param("tenant_id")), 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription_audit_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
