package api

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
	"task-processor/internal/tenantbridge"
)

const subscriptionTenantContextKey = "listingkit.subscription_tenant_id"

func (h *handler) requireSubscription(c *gin.Context, moduleCode string) bool {
	if h.subscriptionService == nil {
		writeSubscriptionRequired(c, listingsubscription.GuardResult{ModuleCode: moduleCode, Reason: "not_configured"})
		return false
	}
	result, err := h.checkSubscriptionWithLegacyFallback(c, func(tenantID string) (listingsubscription.GuardResult, error) {
		return h.subscriptionService.Check(c.Request.Context(), tenantID, moduleCode)
	})
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
	result, err := h.checkSubscriptionWithLegacyFallback(c, func(tenantID string) (listingsubscription.GuardResult, error) {
		return h.subscriptionService.CheckUsage(c.Request.Context(), tenantID, moduleCode, metric, increment)
	})
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
	result, err := h.checkSubscriptionWithLegacyFallback(c, func(tenantID string) (listingsubscription.GuardResult, error) {
		return h.subscriptionService.AuthorizeUsage(c.Request.Context(), tenantID, moduleCode, metric, increment)
	})
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
	_, _ = h.subscriptionService.RecordUsage(c.Request.Context(), subscriptionTenantID(c), moduleCode, metric, increment)
}

func (h *handler) checkSubscriptionWithLegacyFallback(c *gin.Context, check func(tenantID string) (listingsubscription.GuardResult, error)) (listingsubscription.GuardResult, error) {
	tenantID := requestTenantID(c)
	result, err := check(tenantID)
	if err == nil && result.Allowed {
		c.Set(subscriptionTenantContextKey, tenantID)
		return result, nil
	}
	if !shouldTryLegacySubscriptionFallback(err, result) {
		return result, err
	}
	legacyTenantID, ok := resolveLegacySubscriptionTenantID(c, tenantID)
	if !ok {
		return result, err
	}
	fallbackResult, fallbackErr := check(legacyTenantID)
	if fallbackErr == nil && fallbackResult.Allowed {
		c.Set(subscriptionTenantContextKey, legacyTenantID)
		return fallbackResult, nil
	}
	return fallbackResult, fallbackErr
}

func shouldTryLegacySubscriptionFallback(err error, result listingsubscription.GuardResult) bool {
	return errors.Is(err, listingsubscription.ErrSubscriptionRequired) && result.Reason == "not_configured"
}

func resolveLegacySubscriptionTenantID(c *gin.Context, tenantID string) (string, bool) {
	legacyTenantID, err := tenantbridge.ResolveLegacyTenantID(c.Request.Context(), tenantID)
	if err != nil || legacyTenantID <= 0 {
		return "", false
	}
	resolved := strconv.FormatInt(legacyTenantID, 10)
	if resolved == strings.TrimSpace(tenantID) {
		return "", false
	}
	return resolved, true
}

func subscriptionTenantID(c *gin.Context) string {
	if value, ok := c.Get(subscriptionTenantContextKey); ok {
		if tenantID, ok := value.(string); ok && strings.TrimSpace(tenantID) != "" {
			return strings.TrimSpace(tenantID)
		}
	}
	return requestTenantID(c)
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
