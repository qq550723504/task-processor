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
	scope := requestListScope(c)
	query := applyListQueryScope(&PricingRuleQuery{
		Name:     strings.TrimSpace(c.Query("name")),
		RuleCode: strings.TrimSpace(c.Query("ruleCode")),
		RuleType: strings.TrimSpace(c.Query("ruleType")),
	}, scope)
	var ok bool
	query.StoreID, ok = queryInt64PtrStrict(c, "storeId", "invalid_store_id")
	if !ok {
		return
	}
	query.CategoryID, ok = queryInt64PtrStrict(c, "categoryId", "invalid_category_id")
	if !ok {
		return
	}
	query.Status, ok = queryInt16PtrStrict(c, "status", "invalid_status")
	if !ok {
		return
	}

	page, err := h.repo.ListPricingRules(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "pricing_rule_list_failed", err)
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
	if !bindAndValidateJSON(c, &req, "invalid_pricing_rule", func(value *PricingRule) {
		value.TenantID = requestTenantID(c)
	}, validatePricingRule) {
		return
	}
	rule, err := h.repo.CreatePricingRule(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "pricing_rule_create_failed", err)
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
	if !bindAndValidateJSON(c, &req, "invalid_pricing_rule", func(value *PricingRule) {
		value.ID = id
		value.TenantID = requestTenantID(c)
	}, validatePricingRule) {
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

var writePricingRuleError = newMappedHandlerErrorWriter(
	handlerErrorRule{match: ErrPricingRuleNotFound, status: http.StatusNotFound, errorCode: "pricing_rule_not_found"},
)
