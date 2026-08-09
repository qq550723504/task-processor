package api

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
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
	return authorizeSubscriptionUsage(c, h.subscriptionService, moduleCode, metric, increment)
}

func authorizeSubscriptionUsage(c *gin.Context, service *listingsubscription.Service, moduleCode, metric string, increment int) bool {
	if service == nil {
		writeSubscriptionRequired(c, listingsubscription.GuardResult{ModuleCode: moduleCode, Reason: "not_configured"})
		return false
	}
	result, err := checkSubscriptionWithLegacyFallback(c, service, func(tenantID string) (listingsubscription.GuardResult, error) {
		return service.AuthorizeUsage(c.Request.Context(), tenantID, moduleCode, metric, increment)
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
	return checkSubscriptionWithLegacyFallback(c, h.subscriptionService, check)
}

func checkSubscriptionWithLegacyFallback(c *gin.Context, service *listingsubscription.Service, check func(tenantID string) (listingsubscription.GuardResult, error)) (listingsubscription.GuardResult, error) {
	if service == nil {
		return listingsubscription.GuardResult{}, listingsubscription.ErrSubscriptionRequired
	}
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
	identity, ok := authenticatedActor(c)
	if !ok {
		return false
	}
	return h.authorizePlatformSubscriptionIdentity(c, identity)
}

func (h *handler) requirePlatformSubscriptionActor(c *gin.Context) (listingkit.AuthenticatedIdentity, bool) {
	identity, ok := authenticatedActor(c)
	if !ok || !h.authorizePlatformSubscriptionIdentity(c, identity) {
		return listingkit.AuthenticatedIdentity{}, false
	}
	return identity, true
}

func (h *handler) authorizePlatformSubscriptionIdentity(c *gin.Context, identity listingkit.AuthenticatedIdentity) bool {
	if slices.Contains(h.platformAdminUsers, identity.UserID) {
		return true
	}
	allowedRoles := h.platformAdminRoles
	if len(allowedRoles) == 0 {
		allowedRoles = []string{"platform_admin", "admin"}
	}
	for _, role := range identity.Roles {
		if slices.Contains(allowedRoles, strings.TrimSpace(role)) {
			return true
		}
	}
	c.JSON(http.StatusForbidden, gin.H{
		"error":   "listingkit_permission_denied",
		"message": "ListingKit platform administration requires a platform admin role",
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
