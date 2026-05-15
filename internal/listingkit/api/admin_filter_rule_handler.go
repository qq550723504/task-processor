package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) ListAdminFilterRules(c *gin.Context) {
	if !h.requireFilterRuleHandler(c) {
		return
	}
	h.filterRuleHandler.ListFilterRules(c)
}

func (h *handler) GetAdminFilterRule(c *gin.Context) {
	if !h.requireFilterRuleHandler(c) {
		return
	}
	h.filterRuleHandler.GetFilterRule(c)
}

func (h *handler) CreateAdminFilterRule(c *gin.Context) {
	if !h.requireFilterRuleHandler(c) {
		return
	}
	h.filterRuleHandler.CreateFilterRule(c)
}

func (h *handler) UpdateAdminFilterRule(c *gin.Context) {
	if !h.requireFilterRuleHandler(c) {
		return
	}
	h.filterRuleHandler.UpdateFilterRule(c)
}

func (h *handler) UpdateAdminFilterRuleStatus(c *gin.Context) {
	if !h.requireFilterRuleHandler(c) {
		return
	}
	h.filterRuleHandler.UpdateFilterRuleStatus(c)
}

func (h *handler) DeleteAdminFilterRule(c *gin.Context) {
	if !h.requireFilterRuleHandler(c) {
		return
	}
	h.filterRuleHandler.DeleteFilterRule(c)
}

func (h *handler) requireFilterRuleHandler(c *gin.Context) bool {
	if h.filterRuleHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "filter_rule_repository_unavailable",
		"message": "ListingKit filter rule repository is not configured",
	})
	return false
}
