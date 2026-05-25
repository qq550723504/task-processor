package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProfitRuleHandler struct {
	repo ProfitRuleRepository
}

func NewProfitRuleHandler(repo ProfitRuleRepository) *ProfitRuleHandler {
	return &ProfitRuleHandler{repo: repo}
}

func (h *ProfitRuleHandler) ListProfitRules(c *gin.Context) {
	scope := requestListScope(c)
	query := applyListQueryScope(&ProfitRuleQuery{
		Name:     strings.TrimSpace(c.Query("name")),
		RuleCode: strings.TrimSpace(c.Query("ruleCode")),
	}, scope)
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.CategoryID = queryInt64Ptr(c, "categoryId")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListProfitRules(requestIdentityContext(c), *query)
	if err != nil {
		writeInternalHandlerError(c, "profit_rule_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ProfitRuleHandler) GetProfitRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	rule, err := h.repo.GetProfitRule(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeProfitRuleError(c, err, "profit_rule_get_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *ProfitRuleHandler) CreateProfitRule(c *gin.Context) {
	var req ProfitRule
	if !bindAndValidateJSON(c, &req, "invalid_profit_rule", func(value *ProfitRule) {
		value.TenantID = requestTenantID(c)
	}, validateProfitRule) {
		return
	}
	rule, err := h.repo.CreateProfitRule(requestIdentityContext(c), &req)
	if err != nil {
		writeInternalHandlerError(c, "profit_rule_create_failed", err)
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *ProfitRuleHandler) UpdateProfitRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req ProfitRule
	if !bindAndValidateJSON(c, &req, "invalid_profit_rule", func(value *ProfitRule) {
		value.ID = id
		value.TenantID = requestTenantID(c)
	}, validateProfitRule) {
		return
	}
	rule, err := h.repo.UpdateProfitRule(requestIdentityContext(c), &req)
	if err != nil {
		writeProfitRuleError(c, err, "profit_rule_update_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *ProfitRuleHandler) UpdateProfitRuleStatus(c *gin.Context) {
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
	rule, err := h.repo.UpdateProfitRuleStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeProfitRuleError(c, err, "profit_rule_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *ProfitRuleHandler) DeleteProfitRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteProfitRule(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeProfitRuleError(c, err, "profit_rule_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateProfitRule(rule *ProfitRule) error {
	switch {
	case rule.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(rule.Name) == "":
		return errors.New("name is required")
	case strings.TrimSpace(rule.RuleCode) == "":
		return errors.New("ruleCode is required")
	case rule.SalePriceMultiplier < 0:
		return errors.New("salePriceMultiplier cannot be negative")
	case rule.DiscountPriceMultiplier < 0:
		return errors.New("discountPriceMultiplier cannot be negative")
	}
	return nil
}

func writeProfitRuleError(c *gin.Context, err error, code string) {
	writeMappedHandlerError(c, err, code,
		handlerErrorRule{match: ErrProfitRuleNotFound, status: http.StatusNotFound, errorCode: "profit_rule_not_found"},
	)
}
