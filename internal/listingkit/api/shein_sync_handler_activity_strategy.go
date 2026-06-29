package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingadmin"
)

const sheinActivityStrategyTypePromotion = "PROMOTION"
const sheinActivityStrategyTypeTimeLimited = "TIME_LIMITED"
const sheinActivityStrategyPlatform = "SHEIN"

func (h *handler) GetSheinActivityStrategy(c *gin.Context) {
	if h.operationStrategyRepository == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "operation_strategy_unavailable", "message": "operation strategy repository is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}
	activityType, err := normalizeSheinActivityStrategyType(c.Query("activity_type"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_activity_type", "message": err.Error()})
		return
	}
	strategy, err := h.operationStrategyRepository.GetActiveActivityStrategy(ctx, tenantID, storeID, sheinActivityStrategyPlatform, activityType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_activity_strategy_get_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"configured": strategy != nil,
		"strategy":   sheinActivityStrategyDTO(strategy, tenantID, storeID, activityType),
	})
}

func (h *handler) UpdateSheinActivityStrategy(c *gin.Context) {
	if h.operationStrategyRepository == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "operation_strategy_unavailable", "message": "operation strategy repository is not configured"})
		return
	}

	storeID, tenantID, ctx, ok := parseSheinScopedRequest(c)
	if !ok {
		return
	}
	var req updateSheinActivityStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if err := validateSheinActivityStrategyRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_activity_strategy", "message": err.Error()})
		return
	}
	activityType, err := normalizeSheinActivityStrategyType(firstNonBlank(req.ActivityType, c.Query("activity_type")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_activity_type", "message": err.Error()})
		return
	}

	existing, err := h.operationStrategyRepository.GetActiveActivityStrategy(ctx, tenantID, storeID, sheinActivityStrategyPlatform, activityType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_activity_strategy_load_failed", "message": err.Error()})
		return
	}
	strategy := buildSheinActivityOperationStrategy(tenantID, storeID, activityType, existing, req)
	if existing == nil {
		strategy, err = h.operationStrategyRepository.SaveActivityStrategy(ctx, strategy)
	} else {
		strategy, err = h.operationStrategyRepository.SaveActivityStrategy(ctx, strategy)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_activity_strategy_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"configured": true,
		"strategy":   sheinActivityStrategyDTO(strategy, tenantID, storeID, activityType),
	})
}

func normalizeSheinActivityStrategyType(raw string) (string, error) {
	activityType := strings.ToUpper(strings.TrimSpace(raw))
	if activityType == "" {
		return sheinActivityStrategyTypePromotion, nil
	}
	switch activityType {
	case sheinActivityStrategyTypePromotion, sheinActivityStrategyTypeTimeLimited:
		return activityType, nil
	default:
		return "", errors.New("activity_type must be PROMOTION or TIME_LIMITED")
	}
}

func validateSheinActivityStrategyRequest(req updateSheinActivityStrategyRequest) error {
	priceMode := strings.ToUpper(strings.TrimSpace(req.ActivityPriceMode))
	if priceMode == "" {
		priceMode = "DISCOUNT"
	}
	if req.ActivityStockRatio == nil || *req.ActivityStockRatio <= 0 || *req.ActivityStockRatio > 1 {
		return errors.New("activity_stock_ratio must be between 0 and 1")
	}
	switch priceMode {
	case "DISCOUNT":
		if req.ActivityDiscountRate == nil || *req.ActivityDiscountRate <= 0 || *req.ActivityDiscountRate >= 1 {
			return errors.New("activity_discount_rate must be between 0 and 1")
		}
	case "PROFIT":
		if req.ActivityMinProfitRate == nil || *req.ActivityMinProfitRate < 0 || *req.ActivityMinProfitRate >= 1 {
			return errors.New("activity_min_profit_rate must be between 0 and 1")
		}
	default:
		return errors.New("activity_price_mode must be DISCOUNT or PROFIT")
	}
	return nil
}

func buildSheinActivityOperationStrategy(tenantID, storeID int64, activityType string, existing *listingadmin.OperationStrategy, req updateSheinActivityStrategyRequest) *listingadmin.OperationStrategy {
	strategy := &listingadmin.OperationStrategy{
		TenantID:          tenantID,
		StoreID:           storeID,
		Name:              "SHEIN 活动报名",
		Platform:          sheinActivityStrategyPlatform,
		Status:            0,
		ActivityEnabled:   true,
		ActivityType:      activityType,
		ActivityPriceMode: strings.ToUpper(strings.TrimSpace(req.ActivityPriceMode)),
	}
	if strategy.ActivityPriceMode == "" {
		strategy.ActivityPriceMode = "DISCOUNT"
	}
	if existing != nil {
		strategy.ID = existing.ID
		if strings.TrimSpace(existing.Name) != "" {
			strategy.Name = existing.Name
		}
	}
	strategy.ActivityDiscountRate = req.ActivityDiscountRate
	strategy.ActivityStockRatio = req.ActivityStockRatio
	strategy.ActivityMinProfitRate = req.ActivityMinProfitRate
	strategy.FixedPriceAdjustment = req.FixedPriceAdjustment
	return strategy
}

func sheinActivityStrategyDTO(strategy *listingadmin.OperationStrategy, tenantID, storeID int64, activityType string) *sheinActivityStrategyResponse {
	if strategy == nil {
		return nil
	}
	return &sheinActivityStrategyResponse{
		ID:                    strategy.ID,
		TenantID:              tenantID,
		StoreID:               storeID,
		ActivityType:          activityType,
		ActivityPriceMode:     strings.ToUpper(strings.TrimSpace(strategy.ActivityPriceMode)),
		ActivityDiscountRate:  sheinActivityFloat64(strategy.ActivityDiscountRate),
		ActivityStockRatio:    sheinActivityFloat64(strategy.ActivityStockRatio),
		ActivityMinProfitRate: sheinActivityFloat64(strategy.ActivityMinProfitRate),
		FixedPriceAdjustment:  sheinActivityFloat64(strategy.FixedPriceAdjustment),
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sheinActivityFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
