package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) GetCurrentSubscription(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	h.subscriptionHandler.GetCurrentSubscription(c)
}

func (h *handler) ListSubscriptionModules(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	h.subscriptionHandler.ListModules(c)
}

func (h *handler) ListSubscriptionEntitlements(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	h.subscriptionHandler.ListEntitlements(c)
}

func (h *handler) UpsertSubscriptionEntitlement(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	h.subscriptionHandler.UpsertEntitlement(c)
}

func (h *handler) requireSubscription(c *gin.Context, moduleCode string) bool {
	if h.subscriptionService == nil {
		writeSubscriptionRequired(c, listingsubscription.GuardResult{ModuleCode: moduleCode, Reason: "not_configured"})
		return false
	}
	result, err := h.subscriptionService.Check(c.Request.Context(), requestTenantID(c), moduleCode)
	if err == nil && result.Allowed {
		return true
	}
	if errors.Is(err, listingsubscription.ErrSubscriptionQuotaExceed) {
		writeQuotaExceeded(c, result)
		return false
	}
	writeSubscriptionRequired(c, result)
	return false
}

func (h *handler) requireSubscriptionUsage(c *gin.Context, moduleCode, metric string, increment int) bool {
	if h.subscriptionService == nil {
		writeSubscriptionRequired(c, listingsubscription.GuardResult{ModuleCode: moduleCode, Reason: "not_configured"})
		return false
	}
	result, err := h.subscriptionService.CheckUsage(c.Request.Context(), requestTenantID(c), moduleCode, metric, increment)
	if err == nil && result.Allowed {
		return true
	}
	if errors.Is(err, listingsubscription.ErrSubscriptionQuotaExceed) {
		writeQuotaExceeded(c, result)
		return false
	}
	writeSubscriptionRequired(c, result)
	return false
}

func (h *handler) requireSubscriptionHandler(c *gin.Context) bool {
	if h.subscriptionHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "subscription_repository_unavailable",
		"message": "ListingKit subscription repository is not configured",
	})
	return false
}

func writeSubscriptionRequired(c *gin.Context, result listingsubscription.GuardResult) {
	c.JSON(http.StatusPaymentRequired, gin.H{
		"error":       "subscription_required",
		"module_code": result.ModuleCode,
		"message":     "subscription module is not active for this tenant",
		"reason":      result.Reason,
	})
}

func writeQuotaExceeded(c *gin.Context, result listingsubscription.GuardResult) {
	c.JSON(http.StatusPaymentRequired, gin.H{
		"error":       "quota_exceeded",
		"module_code": result.ModuleCode,
		"metric":      result.Metric,
		"limit":       result.Limit,
		"used":        result.Used,
		"message":     "subscription quota exceeded",
	})
}
