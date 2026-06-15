package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

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

func (h *handler) authorizeSubscriptionUsage(c *gin.Context, moduleCode, metric string, increment int) bool {
	if h.subscriptionService == nil {
		writeSubscriptionRequired(c, listingsubscription.GuardResult{ModuleCode: moduleCode, Reason: "not_configured"})
		return false
	}
	result, err := h.subscriptionService.AuthorizeUsage(c.Request.Context(), requestTenantID(c), moduleCode, metric, increment)
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

func (h *handler) recordSubscriptionUsage(c *gin.Context, moduleCode, metric string, increment int) {
	if h.subscriptionService == nil || increment == 0 || metric == "" {
		return
	}
	_, _ = h.subscriptionService.RecordUsage(c.Request.Context(), requestTenantID(c), moduleCode, metric, increment)
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

func (h *handler) requirePlatformSubscriptionAccess(c *gin.Context) bool {
	userID := strings.TrimSpace(c.GetHeader("X-User-ID"))
	if userID != "" && slices.Contains(h.platformAdminUsers, userID) {
		return true
	}
	allowedRoles := h.platformAdminRoles
	if len(allowedRoles) == 0 {
		allowedRoles = []string{"listingkit_admin", "platform_admin", "admin"}
	}
	for _, role := range splitCSVHeaders(c.GetHeader("X-User-Roles"), c.GetHeader("X-Zitadel-Roles")) {
		if slices.Contains(allowedRoles, role) {
			return true
		}
	}
	c.JSON(http.StatusForbidden, gin.H{
		"error":   "platform_subscription_forbidden",
		"message": "platform subscription management requires a platform admin role",
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

func splitCSVHeaders(values ...string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			item := strings.TrimSpace(part)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}
