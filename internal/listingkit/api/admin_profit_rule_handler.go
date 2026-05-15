package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) ListAdminProfitRules(c *gin.Context) {
	if !h.requireProfitRuleHandler(c) {
		return
	}
	h.profitRuleHandler.ListProfitRules(c)
}

func (h *handler) GetAdminProfitRule(c *gin.Context) {
	if !h.requireProfitRuleHandler(c) {
		return
	}
	h.profitRuleHandler.GetProfitRule(c)
}

func (h *handler) CreateAdminProfitRule(c *gin.Context) {
	if !h.requireProfitRuleHandler(c) {
		return
	}
	h.profitRuleHandler.CreateProfitRule(c)
}

func (h *handler) UpdateAdminProfitRule(c *gin.Context) {
	if !h.requireProfitRuleHandler(c) {
		return
	}
	h.profitRuleHandler.UpdateProfitRule(c)
}

func (h *handler) UpdateAdminProfitRuleStatus(c *gin.Context) {
	if !h.requireProfitRuleHandler(c) {
		return
	}
	h.profitRuleHandler.UpdateProfitRuleStatus(c)
}

func (h *handler) DeleteAdminProfitRule(c *gin.Context) {
	if !h.requireProfitRuleHandler(c) {
		return
	}
	h.profitRuleHandler.DeleteProfitRule(c)
}

func (h *handler) requireProfitRuleHandler(c *gin.Context) bool {
	if h.profitRuleHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "profit_rule_repository_unavailable",
		"message": "ListingKit profit rule repository is not configured",
	})
	return false
}
