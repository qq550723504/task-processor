// Package scheduler 提供SHEIN平台限时折扣活动服务
package scheduler

import (
	"context"
	"fmt"
	"time"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

// queryPromotionGoods 查询促销活动商品列表（私有方法）
func (s *activityRegistrationServiceImpl) queryPromotionGoods(
	req *marketing.QueryPromotionGoodsRequest,
) (*marketing.QueryPromotionGoodsResponse, error) {
	s.logger.Debug("开始查询促销活动商品列表")

	response, err := s.marketingAPI.QueryPromotionGoods(req)
	if err != nil {
		s.logger.Errorf("查询促销活动商品列表失败: %v", err)
		return nil, fmt.Errorf("查询促销活动商品列表失败: %w", err)
	}

	if response.Info != nil {
		s.logger.Infof("查询到 %d 个促销商品", response.Info.Meta.Count)
	}

	return response, nil
}

// calculateSupplyPrice 计算供货价格和利润（私有方法）
func (s *activityRegistrationServiceImpl) calculateSupplyPrice(
	req *marketing.CalculateSupplyPriceRequest,
) (*marketing.CalculateSupplyPriceResponse, error) {
	s.logger.Info("开始计算供货价格")

	response, err := s.marketingAPI.CalculateSupplyPrice(req)
	if err != nil {
		s.logger.Errorf("计算供货价格失败: %v", err)
		return nil, fmt.Errorf("计算供货价格失败: %w", err)
	}

	s.logger.Infof("成功计算 %d 个SKC的价格", len(response.Info))
	return response, nil
}

// createTimeLimitedDiscount 创建限时折扣活动（私有方法）
func (s *activityRegistrationServiceImpl) createTimeLimitedDiscount(
	req *marketing.CreateActivityRequest,
) (*marketing.CreateActivityResponse, error) {
	s.logger.WithField("activity_name", req.ActivityBaseInfoRequest.ActName).Debug("开始创建限时折扣活动")

	response, err := s.marketingAPI.CreateActivity(req)
	if err != nil {
		s.logger.Errorf("创建限时折扣活动失败: %v", err)
		return nil, fmt.Errorf("创建限时折扣活动失败: %w", err)
	}

	if response.Info != nil {
		s.logger.Infof("限时折扣活动创建成功，活动ID: %d", response.Info.ActivityID)
	}

	return response, nil
}

// queryAllPromotionGoods 分页查询所有促销活动商品
func (s *activityRegistrationServiceImpl) queryAllPromotionGoods(
	config TimeLimitedDiscountConfig,
) ([]marketing.PromotionGoodsData, error) {
	allGoods := make([]marketing.PromotionGoodsData, 0)
	pageNum := 1

	for {
		// 构建查询请求
		queryReq := s.buildQueryRequest(config)
		queryReq.PageNum = pageNum

		// 查询当前页
		queryResp, err := s.queryPromotionGoods(queryReq)
		if err != nil {
			return nil, fmt.Errorf("查询第 %d 页商品失败: %w", pageNum, err)
		}

		// 检查响应
		if queryResp.Info == nil || len(queryResp.Info.Data) == 0 {
			break
		}

		// 追加当前页数据
		allGoods = append(allGoods, queryResp.Info.Data...)
		s.logger.Infof("已查询第 %d 页，获取 %d 个商品，累计 %d 个", pageNum, len(queryResp.Info.Data), len(allGoods))

		// 检查是否还有更多数据
		if len(allGoods) >= queryResp.Info.Meta.Count {
			break
		}

		// 继续下一页
		pageNum++
	}

	return allGoods, nil
}

// buildQueryRequest 构建查询请求
func (s *activityRegistrationServiceImpl) buildQueryRequest(
	config TimeLimitedDiscountConfig,
) *marketing.QueryPromotionGoodsRequest {
	return &marketing.QueryPromotionGoodsRequest{
		ActivityBaseInfoRequest: marketing.ActivityBaseInfoRequest{
			ActName:       config.ActivityName,
			RefToolID:     config.RefToolID,
			TimeZone:      config.TimeZone,
			ZoneStartTime: config.StartTime.Format("2006-01-02 15:04:05"),
			ZoneEndTime:   config.EndTime.Format("2006-01-02 15:04:05"),
			SubTypeID:     config.SubTypeID,
		},
		EffectiveCenterList: config.EffectiveCenterList,
		IsShelf:             config.IsShelf,
		PageNum:             1,
		PageSize:            config.PageSize,
	}
}

// validatePriceRisk 验证价格风险
func (s *activityRegistrationServiceImpl) validatePriceRisk(
	calcResp *marketing.CalculateSupplyPriceResponse,
	config TimeLimitedDiscountConfig,
) error {
	for _, skcResult := range calcResp.Info {
		for _, skuInfo := range skcResult.SkuInfoList {
			// 检查风险标签
			if skuInfo.RiskTag != 0 && !config.AllowRiskProducts {
				s.logger.Warnf("SKU %s 存在风险，风险标签: %d", skuInfo.SkuCode, skuInfo.RiskTag)
				return ErrProductPriceRisk
			}

			// 检查警告值
			if skuInfo.WarningValue > config.MaxWarningValue {
				s.logger.Warnf("SKU %s 警告值过高: %.2f", skuInfo.SkuCode, skuInfo.WarningValue)
				return ErrProductPriceRisk
			}
		}
	}

	return nil
}

// buildCreateActivityRequest 构建活动创建请求
func (s *activityRegistrationServiceImpl) buildCreateActivityRequest(
	config TimeLimitedDiscountConfig,
	goods []marketing.PromotionGoodsData,
	calcResp *marketing.CalculateSupplyPriceResponse,
) *marketing.CreateActivityRequest {
	// 构建SKC到计算结果的映射,方便快速查找
	skcPriceMap := make(map[string]*marketing.SkcCalculationResult)
	for i := range calcResp.Info {
		skcPriceMap[calcResp.Info[i].SkcName] = &calcResp.Info[i]
	}

	costAndStockList := make([]marketing.CostAndStockInfo, 0, len(goods))

	// 【调试代码】记录已添加的商品数量
	addedCount := 0
	maxProductsPerActivity := 500 // SHEIN平台限制:一次活动最多500个商品

	// 统计各种过滤原因的商品数量
	var (
		totalGoods             = len(goods)
		skippedByActivity      = 0
		skippedByStock         = 0
		skippedByPlatform      = 0
		skippedByPriceCalc     = 0
		skippedByActivityStock = 0
		skippedByPriceInfo     = 0
		skippedByDiscount      = 0
	)

	s.logger.Infof("开始筛选商品，总共 %d 个商品", totalGoods)

	for _, g := range goods {
		// 检查是否已参加其他活动
		if g.ErrorCode != "" {
			s.logger.Warnf("商品 %s 已参加其他活动(活动: %v),跳过该商品", g.Skc, g.ErrorCode)
			skippedByActivity++
			continue
		}

		// 检查库存是否满足最低要求(至少15个)
		minStockRequired := 15
		if g.InventoryNum < minStockRequired {
			s.logger.Warnf("商品 %s 库存不足(%d < %d),跳过该商品", g.Skc, g.InventoryNum, minStockRequired)
			skippedByStock++
			continue
		}

		// 额外检查平台的库存要求
		if g.CheckStock != nil && g.InventoryNum < g.CheckStock.MinStock {
			s.logger.Warnf("商品 %s 库存不足(%d < 平台要求%d),跳过该商品", g.Skc, g.InventoryNum, g.CheckStock.MinStock)
			skippedByPlatform++
			continue
		}

		// 从第5步的计算结果中获取该SKC的价格信息
		skcCalcResult, exists := skcPriceMap[g.Skc]
		if !exists {
			s.logger.Warnf("商品 %s 未找到价格计算结果,跳过", g.Skc)
			skippedByPriceCalc++
			continue
		}

		// 构建SKU列表
		addSkuList := make([]marketing.SkuCostInfo, 0, len(g.SkuInfoList))
		for _, sku := range g.SkuInfoList {
			addSkuList = append(addSkuList, marketing.SkuCostInfo{
				Sku:                sku.Sku,
				CostPrice:          0,
				MaxProductActPrice: 0,
				ProductActPrice:    0,
			})
		}

		// 确定库存数量
		var stockNum int

		// 如果启用了库存限量，按百分比计算
		if config.StockLimit && g.InventoryNum > 0 {
			stockNum = int(float64(g.InventoryNum) * float64(config.StockPercent) / 100.0)
			if stockNum < 1 {
				stockNum = 1 // 至少1个
			}
		} else {
			// 如果不限量，使用实际库存
			stockNum = g.InventoryNum
		}

		// 检查活动库存是否满足平台要求
		if g.CheckStock != nil {
			if stockNum < g.CheckStock.MinStock {
				s.logger.Warnf("商品 %s 活动库存(%d)低于平台最低要求(%d),跳过该商品", g.Skc, stockNum, g.CheckStock.MinStock)
				skippedByActivityStock++
				continue
			}
			if stockNum > g.CheckStock.MaxStock {
				s.logger.Warnf("商品 %s 活动库存(%d)超过平台最大限制(%d),调整为最大值", g.Skc, stockNum, g.CheckStock.MaxStock)
				stockNum = g.CheckStock.MaxStock
			}
		}

		// 使用第5步已经计算并验证过的活动价格
		// 从SKU列表中获取第一个SKU的促销金额作为活动价格
		var activityPrice float64
		if len(skcCalcResult.SkuInfoList) > 0 {
			// 活动价格 = 商品金额 - 促销金额
			priceInfo := skcCalcResult.SkuInfoList[0].PriceInfo
			activityPrice = priceInfo.ProductAmount - priceInfo.PromotionAmount
		} else {
			s.logger.Warnf("商品 %s 没有SKU价格信息,跳过", g.Skc)
			skippedByPriceInfo++
			continue
		}

		// 检查折扣率是否满足要求(活动价必须小于销售价的95%)
		maxDiscountRate := 0.95 // 最大折扣率95%,即至少5%折扣
		if activityPrice >= g.USSupplyPrice*maxDiscountRate {
			actualDiscountRate := activityPrice / g.USSupplyPrice
			s.logger.Warnf("商品 %s 折扣不足(活动价:%.2f, 原价:%.2f, 折扣率:%.2f%%, 要求<%.2f%%),跳过该商品",
				g.Skc, activityPrice, g.USSupplyPrice, actualDiscountRate*100, maxDiscountRate*100)
			skippedByDiscount++
			continue
		}

		costAndStockList = append(costAndStockList, marketing.CostAndStockInfo{
			Skc:                g.Skc,
			AttendNum:          stockNum, // 活动库存
			StockNum:           stockNum, // 也设为活动库存
			CenterList:         config.EffectiveCenterList,
			IsSaleAttribute:    g.IsSaleAttribute,
			PromotionIDList:    nil,
			CostPrice:          g.USSupplyPrice,
			MaxProductActPrice: g.MaxUSSupplyPrice,
			ProductActPrice:    activityPrice,
			AddSkuList:         addSkuList,
		})

		addedCount++

		// 检查是否已达到商品数量上限
		if addedCount >= maxProductsPerActivity {
			s.logger.Warnf("已达到单次活动商品数量上限(%d),停止添加商品", maxProductsPerActivity)
			break
		}

	}

	// 输出详细的筛选统计信息
	s.logger.WithFields(logrus.Fields{
		"total_goods":               totalGoods,
		"skipped_by_activity":       skippedByActivity,
		"skipped_by_stock":          skippedByStock,
		"skipped_by_platform":       skippedByPlatform,
		"skipped_by_price_calc":     skippedByPriceCalc,
		"skipped_by_activity_stock": skippedByActivityStock,
		"skipped_by_price_info":     skippedByPriceInfo,
		"skipped_by_discount":       skippedByDiscount,
		"final_selected":            len(costAndStockList),
	}).Info("商品筛选统计")

	s.logger.Infof("成功筛选出 %d 个符合条件的商品用于活动", len(costAndStockList))

	return &marketing.CreateActivityRequest{
		ActivityBaseInfoRequest: marketing.ActivityBaseInfo{
			ActName:       config.ActivityName,
			TimeZone:      config.TimeZone,
			ZoneStartTime: config.StartTime.Format("2006-01-02 15:04:05"),
			ZoneEndTime:   config.EndTime.Format("2006-01-02 15:04:05"),
			RefToolID:     config.RefToolID,
			NotifyFlag:    1,
			SubTypeID:     config.SubTypeID,
			ActivityRule: marketing.ActivityRule{
				GoodsLimit:    config.GoodsLimit,
				GoodsLimitNum: config.GoodsLimitNum,
			},
		},
		AddCostAndStockInfoList: costAndStockList,
		PricingType:             config.PricingType,
	}
}

// checkCreateResult 检查创建结果
func (s *activityRegistrationServiceImpl) checkCreateResult(
	resp *marketing.CreateActivityResponse,
) error {
	if resp.Info == nil {
		return ErrActivityCreationFailed
	}

	// 检查错误信息
	if resp.Info.ErrorInfo != nil {
		s.logger.Warnf("活动创建有错误信息: %v", resp.Info.ErrorInfo)
	}

	if resp.Info.SkcErrorInfo != nil {
		s.logger.Warnf("SKC错误信息: %v", resp.Info.SkcErrorInfo)
	}

	if resp.Info.SkuErrorInfo != nil {
		s.logger.Warnf("SKU错误信息: %v", resp.Info.SkuErrorInfo)
	}

	return nil
}

// GenerateActivityName 生成活动名称
func GenerateActivityName(username string, sequence int) string {
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	return fmt.Sprintf("#%s#限时折扣#%s#%d", username, dateStr, sequence)
}

// CreateTimeLimitedDiscountActivity 根据运营策略创建限时折扣活动（完整流程）
func (s *activityRegistrationServiceImpl) CreateTimeLimitedDiscountActivity(
	ctx context.Context,
	strategy *managementapi.OperationStrategyDTO,
) (int, error) {
	s.logger.WithFields(logrus.Fields{
		"store_id":      strategy.StoreID,
		"discount_rate": strategy.ActivityDiscountRate,
		"stock_ratio":   strategy.ActivityStockRatio,
	}).Info("开始根据运营策略创建限时折扣活动")

	// 1. 获取店铺信息
	storeClient := s.managementClient.GetStoreClient()
	storeInfo, err := storeClient.GetStore(strategy.StoreID)
	if err != nil {
		s.logger.WithError(err).Error("获取店铺信息失败")
		return 0, fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 2. 构建限时折扣配置
	config := s.buildTimeLimitedDiscountConfig(storeInfo, strategy)

	// 3. 验证配置
	if err := config.Validate(); err != nil {
		return 0, fmt.Errorf("配置验证失败: %w", err)
	}

	// 4. 分页查询所有可参加活动的商品
	allGoods, err := s.queryAllPromotionGoods(config)
	if err != nil {
		return 0, fmt.Errorf("查询商品失败: %w", err)
	}

	if len(allGoods) == 0 {
		s.logger.Warn("没有可参加活动的商品")
		return 0, ErrNoAvailableProducts
	}

	s.logger.Infof("共查询到 %d 个可参加活动的商品", len(allGoods))

	// 5. 计算商品价格和利润
	calcReq := s.buildCalculateRequestWithPriceMode(config, allGoods, strategy.StoreID)
	calcResp, err := s.calculateSupplyPrice(calcReq)
	if err != nil {
		return 0, fmt.Errorf("计算价格失败: %w", err)
	}

	// 6. 检查价格风险
	if err := s.validatePriceRisk(calcResp, config); err != nil {
		return 0, fmt.Errorf("价格风险检查失败: %w", err)
	}

	// 7. 构建活动创建请求
	createReq := s.buildCreateActivityRequest(config, allGoods, calcResp)

	// 检查是否有符合条件的商品
	if len(createReq.AddCostAndStockInfoList) == 0 {
		s.logger.Warn("没有符合条件的商品可以参加活动")
		return 0, ErrNoAvailableProducts
	}

	s.logger.Infof("准备创建活动，包含 %d 个商品", len(createReq.AddCostAndStockInfoList))

	// 8. 创建限时折扣活动
	createResp, err := s.createTimeLimitedDiscount(createReq)
	if err != nil {
		return 0, fmt.Errorf("创建活动失败: %w", err)
	}

	// 9. 检查创建结果
	if err := s.checkCreateResult(createResp); err != nil {
		return 0, fmt.Errorf("活动创建结果异常: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"activity_id":   createResp.Info.ActivityID,
		"product_count": len(allGoods),
	}).Info("限时折扣活动创建成功")

	return len(allGoods), nil
}
