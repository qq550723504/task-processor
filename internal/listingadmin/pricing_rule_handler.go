package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type PricingRuleHandler struct{ repo PricingRuleRepository }

func NewPricingRuleHandler(repo PricingRuleRepository) *PricingRuleHandler {
	return &PricingRuleHandler{repo: repo}
}

func (h *PricingRuleHandler) ListPricingRules(c *gin.Context) {
	query := PricingRuleQuery{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestScopedOwnerUserID(c),
		Page:        queryInt(c, "page", queryInt(c, "pageNo", 1)),
		PageSize:    queryInt(c, "page_size", queryInt(c, "pageSize", 20)),
		Name:        strings.TrimSpace(c.Query("name")),
		RuleCode:    strings.TrimSpace(c.Query("ruleCode")),
		RuleType:    strings.TrimSpace(c.Query("ruleType")),
	}
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.CategoryID = queryInt64Ptr(c, "categoryId")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListPricingRules(requestIdentityContext(c), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pricing_rule_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *PricingRuleHandler) GetPricingRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	rule, err := h.repo.GetPricingRule(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writePricingRuleError(c, err, "pricing_rule_get_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *PricingRuleHandler) CreatePricingRule(c *gin.Context) {
	var req PricingRule
	if !bindJSON(c, &req) {
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validatePricingRule(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_pricing_rule", "message": err.Error()})
		return
	}
	rule, err := h.repo.CreatePricingRule(requestIdentityContext(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pricing_rule_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *PricingRuleHandler) UpdatePricingRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req PricingRule
	if !bindJSON(c, &req) {
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validatePricingRule(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_pricing_rule", "message": err.Error()})
		return
	}
	rule, err := h.repo.UpdatePricingRule(requestIdentityContext(c), &req)
	if err != nil {
		writePricingRuleError(c, err, "pricing_rule_update_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *PricingRuleHandler) UpdatePricingRuleStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int16  `json:"status"`
		Remark string `json:"remark"`
	}
	if !bindJSON(c, &req) {
		return
	}
	rule, err := h.repo.UpdatePricingRuleStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writePricingRuleError(c, err, "pricing_rule_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *PricingRuleHandler) DeletePricingRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeletePricingRule(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writePricingRuleError(c, err, "pricing_rule_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validatePricingRule(rule *PricingRule) error {
	switch {
	case rule.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(rule.Name) == "":
		return errors.New("name is required")
	case strings.TrimSpace(rule.RuleCode) == "":
		return errors.New("ruleCode is required")
	case strings.TrimSpace(rule.RuleType) == "":
		return errors.New("ruleType is required")
	case rule.PriceMin < 0:
		return errors.New("priceMin cannot be negative")
	case rule.PriceMax < 0:
		return errors.New("priceMax cannot be negative")
	case rule.PriceMax > 0 && rule.PriceMin > rule.PriceMax:
		return errors.New("priceMin cannot exceed priceMax")
	case rule.RuleValue < 0:
		return errors.New("ruleValue cannot be negative")
	}
	return nil
}

func writePricingRuleError(c *gin.Context, err error, code string) {
	writeMappedHandlerError(c, err, code,
		handlerErrorRule{match: ErrPricingRuleNotFound, status: http.StatusNotFound, errorCode: "pricing_rule_not_found"},
	)
}
