package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListAdminPricingRules(c *gin.Context) {
	if !h.requirePricingRuleHandler(c) {
		return
	}
	h.pricingRuleHandler.ListPricingRules(c)
}

func (h *handler) GetAdminPricingRule(c *gin.Context) {
	if !h.requirePricingRuleHandler(c) {
		return
	}
	h.pricingRuleHandler.GetPricingRule(c)
}

func (h *handler) CreateAdminPricingRule(c *gin.Context) {
	if !h.requirePricingRuleHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.pricingRuleHandler.CreatePricingRule(c)
}

func (h *handler) UpdateAdminPricingRule(c *gin.Context) {
	if !h.requirePricingRuleHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.pricingRuleHandler.UpdatePricingRule(c)
}

func (h *handler) UpdateAdminPricingRuleStatus(c *gin.Context) {
	if !h.requirePricingRuleHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.pricingRuleHandler.UpdatePricingRuleStatus(c)
}

func (h *handler) DeleteAdminPricingRule(c *gin.Context) {
	if !h.requirePricingRuleHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.pricingRuleHandler.DeletePricingRule(c)
}

func (h *handler) requirePricingRuleHandler(c *gin.Context) bool {
	if h.pricingRuleHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "pricing_rule_repository_unavailable",
		"message": "ListingKit pricing rule repository is not configured",
	})
	return false
}
