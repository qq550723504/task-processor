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
	query := ProfitRuleQuery{
		TenantID: requestTenantID(c),
		Page:     queryInt(c, "page", queryInt(c, "pageNo", 1)),
		PageSize: queryInt(c, "page_size", queryInt(c, "pageSize", 20)),
		Name:     strings.TrimSpace(c.Query("name")),
		RuleCode: strings.TrimSpace(c.Query("ruleCode")),
	}
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.CategoryID = queryInt64Ptr(c, "categoryId")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListProfitRules(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "profit_rule_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ProfitRuleHandler) GetProfitRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	rule, err := h.repo.GetProfitRule(c.Request.Context(), requestTenantID(c), id)
	if err != nil {
		writeProfitRuleError(c, err, "profit_rule_get_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *ProfitRuleHandler) CreateProfitRule(c *gin.Context) {
	var req ProfitRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateProfitRule(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_profit_rule", "message": err.Error()})
		return
	}
	rule, err := h.repo.CreateProfitRule(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "profit_rule_create_failed", "message": err.Error()})
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateProfitRule(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_profit_rule", "message": err.Error()})
		return
	}
	rule, err := h.repo.UpdateProfitRule(c.Request.Context(), &req)
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	rule, err := h.repo.UpdateProfitRuleStatus(c.Request.Context(), requestTenantID(c), id, req.Status, req.Remark)
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
	if err := h.repo.DeleteProfitRule(c.Request.Context(), requestTenantID(c), id); err != nil {
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
	if errors.Is(err, ErrProfitRuleNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "profit_rule_not_found", "message": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": code, "message": err.Error()})
}
