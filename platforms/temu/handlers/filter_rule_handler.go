package handlers

import (
	"fmt"
	"regexp"
	"strconv"

	"task-processor/common/amazon/model"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// FilterRuleHandler 筛选规则处理器
type FilterRuleHandler struct {
	logger           *logrus.Entry
	filterRuleClient api.FilterRuleAPI
}

// NewFilterRuleHandler 创建新的筛选规则处理器
func NewFilterRuleHandler(filterRuleClient api.FilterRuleAPI) *FilterRuleHandler {
	return &FilterRuleHandler{
		logger:           logrus.WithField("handler", "FilterRuleHandler"),
		filterRuleClient: filterRuleClient,
	}
}

// Name 返回处理器名称
func (h *FilterRuleHandler) Name() string {
	return "筛选规则处理器"
}

// Handle 处理任务 - 在任务开始时筛选主产品
func (h *FilterRuleHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始应用筛选规则 - 主产品筛选")

	if ctx.AmazonProduct == nil {
		return fmt.Errorf("Amazon产品数据为空")
	}

	// 获取筛选规则
	rules, err := h.getFilterRules(ctx)
	if err != nil {
		// 详细记录错误信息，但不阻断流程
		h.logger.WithFields(logrus.Fields{
			"tenant_id":   ctx.Task.TenantID,
			"store_id":    ctx.Task.StoreID,
			"category_id": ctx.Task.CategoryID,
			"error":       err.Error(),
		}).Warn("获取筛选规则失败，跳过筛选")
		return nil // 不阻断流程，继续执行
	}

	// 检查是否有启用的规则
	if rules == nil || len(*rules) == 0 {
		h.logger.Info("未配置筛选规则，跳过筛选")
		return nil
	}

	// 保存第一个启用的规则到 context，供后续保存使用
	for _, rule := range *rules {
		if rule.Status == 0 { // 0表示启用
			ctx.SetData("filter_rule", &rule)
			h.logger.Infof("保存筛选规则到context: ID=%d, Name=%s", rule.ID, rule.Name)
			break
		}
	}

	// 应用筛选规则到主产品
	if !h.checkProductAgainstRules(ctx.AmazonProduct, rules, ctx) {
		h.logger.Warnf("主产品 %s 不符合筛选规则，任务终止", ctx.AmazonProduct.Asin)
		return fmt.Errorf("TERMINATED: 主产品不符合筛选规则")
	}

	h.logger.Infof("主产品通过筛选规则检查: %s", ctx.AmazonProduct.Asin)
	return nil
}

// FilterVariants 筛选变体产品（在获取变体后调用）
func (h *FilterRuleHandler) FilterVariants(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始应用筛选规则 - 变体筛选")

	if len(ctx.AmazonVariants) == 0 {
		h.logger.Info("没有变体需要筛选")
		return nil
	}

	// 获取筛选规则
	rules, err := h.getFilterRules(ctx)
	if err != nil {
		// 详细记录错误信息，但不阻断流程
		h.logger.WithFields(logrus.Fields{
			"tenant_id":   ctx.Task.TenantID,
			"store_id":    ctx.Task.StoreID,
			"category_id": ctx.Task.CategoryID,
			"error":       err.Error(),
		}).Warn("获取筛选规则失败，跳过变体筛选")
		return nil
	}

	// 检查是否有启用的规则
	if rules == nil || len(*rules) == 0 {
		h.logger.Info("未配置筛选规则，跳过变体筛选")
		return nil
	}

	// 筛选变体
	var filteredVariants []*model.Product
	originalCount := len(ctx.AmazonVariants)

	for _, variant := range ctx.AmazonVariants {
		if h.checkProductAgainstRules(variant, rules, ctx) {
			filteredVariants = append(filteredVariants, variant)
			h.logger.Debugf("变体通过筛选: %s", variant.Asin)
		} else {
			h.logger.Infof("变体被筛选规则过滤: %s", variant.Asin)
		}
	}

	// 更新变体列表
	ctx.AmazonVariants = filteredVariants
	filteredCount := len(filteredVariants)

	h.logger.Infof("变体筛选完成: 原始数量=%d, 筛选后数量=%d, 过滤数量=%d",
		originalCount, filteredCount, originalCount-filteredCount)

	// 如果所有变体都被过滤掉，记录警告但不阻断流程
	if filteredCount == 0 {
		h.logger.Warn("所有变体都被筛选规则过滤，将只处理主产品")
	}

	return nil
}

// getFilterRules 获取筛选规则
func (h *FilterRuleHandler) getFilterRules(ctx *pipeline.TaskContext) (*[]api.FilterRuleRespDTO, error) {
	if ctx.Task == nil {
		return nil, fmt.Errorf("任务信息为空")
	}

	req := &api.FilterRuleReqDTO{
		TenantID: ctx.Task.TenantID,
		StoreID:  ctx.Task.StoreID,
	}

	// 如果有分类信息，添加到请求中
	if ctx.Task.CategoryID > 0 {
		req.CategoryID = ctx.Task.CategoryID
	}

	h.logger.WithFields(logrus.Fields{
		"tenant_id":   req.TenantID,
		"store_id":    req.StoreID,
		"category_id": req.CategoryID,
	}).Debug("正在获取筛选规则")

	rules, err := h.filterRuleClient.GetFilterRule(req)
	if err != nil {
		// 包装错误信息，提供更多上下文
		return nil, fmt.Errorf("获取过滤规则失败: %w", err)
	}

	return rules, nil
}

// checkProductAgainstRules 检查产品是否符合筛选规则
func (h *FilterRuleHandler) checkProductAgainstRules(product *model.Product, rules *[]api.FilterRuleRespDTO, ctx *pipeline.TaskContext) bool {
	if rules == nil || len(*rules) == 0 {
		h.logger.Debug("没有筛选规则，产品通过")
		return true
	}

	for _, rule := range *rules {
		// 跳过禁用的规则
		if rule.Status != 0 {
			h.logger.Debugf("跳过禁用的规则: %s (ID: %d)", rule.Name, rule.ID)
			continue
		}

		if !h.checkSingleRule(product, &rule, ctx) {
			h.logger.Infof("产品 %s 不符合规则 '%s': %s", product.Asin, rule.Name, rule.Description)
			return false
		}
	}

	return true
}

// checkSingleRule 检查单个规则
func (h *FilterRuleHandler) checkSingleRule(product *model.Product, rule *api.FilterRuleRespDTO, ctx *pipeline.TaskContext) bool {
	// 价格检查
	if !h.checkPriceRule(product, rule, ctx) {
		return false
	}

	// 评分检查
	if !h.checkRatingRule(product, rule) {
		return false
	}

	// 评论数量检查
	if !h.checkReviewCountRule(product, rule) {
		return false
	}

	// 库存检查
	if !h.checkStockRule(product, rule) {
		return false
	}

	// 配送方式检查
	if !h.checkFulfillmentTypeRule(product, rule) {
		return false
	}

	return true
}

// checkPriceRule 检查价格规则
func (h *FilterRuleHandler) checkPriceRule(product *model.Product, rule *api.FilterRuleRespDTO, ctx *pipeline.TaskContext) bool {
	// 获取店铺配置的价格类型
	priceType := "special"
	if ctx != nil && ctx.StoreInfo != nil && ctx.StoreInfo.PriceType != "" {
		priceType = ctx.StoreInfo.PriceType
	}

	// 根据价格类型获取价格(包含运费)
	price := getProductPrice(product, priceType)

	// 最低价格检查
	if rule.PriceMin != nil && price < *rule.PriceMin {
		h.logger.Debugf("价格 %.2f 低于最低价格 %.2f", price, *rule.PriceMin)
		return false
	}

	// 最高价格检查
	if rule.PriceMax != nil && price > *rule.PriceMax {
		h.logger.Debugf("价格 %.2f 高于最高价格 %.2f", price, *rule.PriceMax)
		return false
	}

	return true
}

// checkRatingRule 检查评分规则
func (h *FilterRuleHandler) checkRatingRule(product *model.Product, rule *api.FilterRuleRespDTO) bool {
	if rule.RatingMin == nil {
		return true
	}

	rating := product.Rating
	if rating < *rule.RatingMin {
		h.logger.Debugf("评分 %.1f 低于最低评分 %.1f", rating, *rule.RatingMin)
		return false
	}

	return true
}

// checkReviewCountRule 检查评论数量规则
func (h *FilterRuleHandler) checkReviewCountRule(product *model.Product, rule *api.FilterRuleRespDTO) bool {
	if rule.ReviewCountMin == nil {
		return true
	}

	reviewCount := product.ReviewsCount
	if reviewCount < *rule.ReviewCountMin {
		h.logger.Debugf("评论数量 %d 低于最低评论数量 %d", reviewCount, *rule.ReviewCountMin)
		return false
	}

	return true
}

// checkStockRule 检查库存规则
func (h *FilterRuleHandler) checkStockRule(product *model.Product, rule *api.FilterRuleRespDTO) bool {
	if rule.StockMin == nil {
		return true
	}

	// 获取库存数量
	stock := h.getInventory(product)

	if stock < *rule.StockMin {
		h.logger.Debugf("库存 %d 低于最低库存 %d", stock, *rule.StockMin)
		return false
	}

	return true
}

// getInventory 获取库存数量（支持多语言）
func (h *FilterRuleHandler) getInventory(product *model.Product) int {

	// 优先：如果有明确的最大可用数量
	if product.MaxQuantityAvailable > 0 {
		h.logger.WithFields(logrus.Fields{
			"asin":  product.Asin,
			"stock": product.MaxQuantityAvailable,
		}).Debug("✅ 从 MaxQuantityAvailable 获取库存")
		return product.MaxQuantityAvailable
	}

	// 其次：尝试从 Availability 文本中提取具体数量
	availability := product.Availability

	// 支持多语言的库存数量提取
	patterns := []string{
		// 英语
		`(?i)only\s+(\d+)\s+left`,                 // "Only 13 left in stock"
		`(?i)(\d+)\s+left`,                        // "13 left in stock"
		`(?i)(\d+)\s+in\s+stock`,                  // "13 in stock"
		`(?i)(\d+)\s+available`,                   // "13 available"
		`(?i)(\d+)\s+remaining`,                   // "13 remaining"
		`(?i)stock:\s*(\d+)`,                      // "Stock: 13"
		`(?i)quantity:\s*(\d+)`,                   // "Quantity: 13"
		`(?i)(\d+)\s+units?\s+(?:left|available)`, // "13 units left"
		// 西班牙语
		`(?i)quedan\s+(\d+)`,       // "quedan 13"
		`(?i)(\d+)\s+disponibles?`, // "13 disponibles"
		`(?i)solo\s+(\d+)`,         // "solo 13"
		// 日语
		`(?i)残り\s*(\d+)`, // "残り13"
		`(?i)(\d+)\s*個`,  // "13個"
		`(?i)(\d+)\s*点`,  // "13点"
		// 德语
		`(?i)noch\s+(\d+)`,       // "noch 13"
		`(?i)(\d+)\s+verfügbar`,  // "13 verfügbar"
		`(?i)nur\s+noch\s+(\d+)`, // "nur noch 13"
		// 法语
		`(?i)reste\s+(\d+)`,        // "reste 13"
		`(?i)(\d+)\s+disponibles?`, // "13 disponible(s)"
		`(?i)seulement\s+(\d+)`,    // "seulement 13"
		// 意大利语
		`(?i)rimangono\s+(\d+)`,    // "rimangono 13"
		`(?i)(\d+)\s+disponibili?`, // "13 disponibili"
		`(?i)solo\s+(\d+)`,         // "solo 13"
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(availability); len(matches) > 1 {
			if stock, err := strconv.Atoi(matches[1]); err == nil {
				h.logger.WithFields(logrus.Fields{
					"asin":    product.Asin,
					"pattern": pattern,
					"stock":   stock,
				}).Debug("✅ 从 Availability 文本中提取到库存数量")
				return stock
			}
		}
	}

	// 再次：检查 IsAvailable 字段
	if !product.IsAvailable {
		h.logger.WithFields(logrus.Fields{
			"asin":         product.Asin,
			"availability": product.Availability,
		}).Warn("⚠️ 产品标记为不可用且无法从文本提取库存，返回库存 0")
		return 0
	}

	// 最后：如果产品可用但没有具体数量，返回一个较大的值（表示充足库存）
	// 根据用户说明，In Stock代表库存大于30
	h.logger.WithFields(logrus.Fields{
		"asin":         product.Asin,
		"availability": product.Availability,
		"stock":        31,
	}).Debug("✅ 产品可用但无具体数量，返回默认库存 31")
	return 31
}

// checkFulfillmentTypeRule 检查配送方式规则
func (h *FilterRuleHandler) checkFulfillmentTypeRule(product *model.Product, rule *api.FilterRuleRespDTO) bool {
	// 如果规则未设置配送方式或设置为ALL，则不进行筛选
	if rule.FulfillmentType == "" || rule.FulfillmentType == "ALL" {
		return true
	}

	// 判断产品的配送方式
	isFBA := h.isFBAFulfillment(product.ShipsFrom)
	isAMZ := h.isAMZSeller(product.SellerName)

	// 根据规则要求进行校验
	switch rule.FulfillmentType {
	case "FBA":
		if !isFBA {
			h.logger.Debugf("配送方式不符合：规则要求FBA配送，但产品为FBM配送 (ships_from: %s)", product.ShipsFrom)
			return false
		}
	case "FBM":
		if isFBA {
			h.logger.Debugf("配送方式不符合：规则要求FBM配送，但产品为FBA配送 (ships_from: %s)", product.ShipsFrom)
			return false
		}
	case "AMZ":
		if !isAMZ {
			h.logger.Debugf("配送方式不符合：规则要求亚马逊自营，但卖家为 %s", product.SellerName)
			return false
		}
	default:
		h.logger.Warnf("未知的配送方式类型: %s", rule.FulfillmentType)
	}

	return true
}

// isFBAFulfillment 判断是否为FBA配送
func (h *FilterRuleHandler) isFBAFulfillment(shipsFrom string) bool {
	if shipsFrom == "" {
		return false
	}

	// 支持多语言站点的 Amazon 关键词匹配
	amazonKeywords := []string{
		"Amazon",
		"amazon",
		"AMAZON",
	}

	for _, keyword := range amazonKeywords {
		if regexp.MustCompile(keyword).MatchString(shipsFrom) {
			return true
		}
	}

	return false
}

// isAMZSeller 判断是否为亚马逊自营
func (h *FilterRuleHandler) isAMZSeller(sellerName string) bool {
	if sellerName == "" {
		return false
	}

	// 支持多语言站点的 Amazon 卖家名称匹配
	amazonSellerKeywords := []string{
		"Amazon",
		"amazon",
		"AMAZON",
		"Amazon.com",
		"Amazon.co.jp",
		"Amazon.de",
		"Amazon.fr",
		"Amazon.co.uk",
		"Amazon.es",
		"Amazon.it",
		"Amazon.com.mx",
	}

	for _, keyword := range amazonSellerKeywords {
		if regexp.MustCompile(keyword).MatchString(sellerName) {
			return true
		}
	}

	return false
}

// GetFilterRuleStats 获取筛选规则统计信息（用于调试和监控）
func (h *FilterRuleHandler) GetFilterRuleStats(ctx *pipeline.TaskContext) (map[string]interface{}, error) {
	rules, err := h.getFilterRules(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_rules":    len(*rules),
		"enabled_rules":  0,
		"disabled_rules": 0,
		"rule_details":   make([]map[string]interface{}, 0),
	}

	for _, rule := range *rules {
		if rule.Status == 1 {
			stats["enabled_rules"] = stats["enabled_rules"].(int) + 1
		} else {
			stats["disabled_rules"] = stats["disabled_rules"].(int) + 1
		}

		ruleDetail := map[string]interface{}{
			"id":          rule.ID,
			"name":        rule.Name,
			"description": rule.Description,
			"status":      rule.Status,
		}
		stats["rule_details"] = append(stats["rule_details"].([]map[string]interface{}), ruleDetail)
	}

	return stats, nil
}
