package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type FilterRuleHandler struct {
	repo FilterRuleRepository
}

func NewFilterRuleHandler(repo FilterRuleRepository) *FilterRuleHandler {
	return &FilterRuleHandler{repo: repo}
}

func (h *FilterRuleHandler) ListFilterRules(c *gin.Context) {
	pageNum, pageSize := requestPageParams(c)
	query := FilterRuleQuery{
		TenantID:        requestTenantID(c),
		OwnerUserID:     requestScopedOwnerUserID(c),
		Page:            pageNum,
		PageSize:        pageSize,
		Name:            strings.TrimSpace(c.Query("name")),
		RuleCode:        strings.TrimSpace(c.Query("ruleCode")),
		PriceType:       strings.TrimSpace(c.Query("priceType")),
		FulfillmentType: strings.TrimSpace(c.Query("fulfillmentType")),
	}
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.CategoryID = queryInt64Ptr(c, "categoryId")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListFilterRules(requestIdentityContext(c), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "filter_rule_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *FilterRuleHandler) GetFilterRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	rule, err := h.repo.GetFilterRule(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeFilterRuleError(c, err, "filter_rule_get_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *FilterRuleHandler) CreateFilterRule(c *gin.Context) {
	var req FilterRule
	if !bindJSON(c, &req) {
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateFilterRule(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_filter_rule", "message": err.Error()})
		return
	}
	rule, err := h.repo.CreateFilterRule(requestIdentityContext(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "filter_rule_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *FilterRuleHandler) UpdateFilterRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req FilterRule
	if !bindJSON(c, &req) {
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateFilterRule(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_filter_rule", "message": err.Error()})
		return
	}
	rule, err := h.repo.UpdateFilterRule(requestIdentityContext(c), &req)
	if err != nil {
		writeFilterRuleError(c, err, "filter_rule_update_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *FilterRuleHandler) UpdateFilterRuleStatus(c *gin.Context) {
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
	rule, err := h.repo.UpdateFilterRuleStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeFilterRuleError(c, err, "filter_rule_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *FilterRuleHandler) DeleteFilterRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteFilterRule(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeFilterRuleError(c, err, "filter_rule_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateFilterRule(rule *FilterRule) error {
	switch {
	case rule.TenantID <= 0:
		return errors.New("tenant id is required")
	case strings.TrimSpace(rule.Name) == "":
		return errors.New("name is required")
	case strings.TrimSpace(rule.RuleCode) == "":
		return errors.New("ruleCode is required")
	case rule.PriceMin < 0:
		return errors.New("priceMin cannot be negative")
	case rule.PriceMax < 0:
		return errors.New("priceMax cannot be negative")
	case rule.PriceMax > 0 && rule.PriceMin > rule.PriceMax:
		return errors.New("priceMin cannot exceed priceMax")
	case rule.RatingMin < 0 || rule.RatingMin > 5:
		return errors.New("ratingMin must be between 0 and 5")
	}
	return nil
}

func writeFilterRuleError(c *gin.Context, err error, code string) {
	writeMappedHandlerError(c, err, code,
		handlerErrorRule{match: ErrFilterRuleNotFound, status: http.StatusNotFound, errorCode: "filter_rule_not_found"},
	)
}
