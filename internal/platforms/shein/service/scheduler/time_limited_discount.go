// Package scheduler 提供SHEIN平台限时折扣活动服务
package scheduler

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/platforms/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

// QueryPromotionGoods 查询促销活动商品列表
func (s *activityRegistrationServiceImpl) QueryPromotionGoods(
	ctx context.Context,
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

// CalculateSupplyPrice 计算供货价格和利润
func (s *activityRegistrationServiceImpl) CalculateSupplyPrice(
	ctx context.Context,
	req *marketing.CalculateSupplyPriceRequest,
) (*marketing.CalculateSupplyPriceResponse, error) {
	s.logger.Debug("开始计算供货价格")

	response, err := s.marketingAPI.CalculateSupplyPrice(req)
	if err != nil {
		s.logger.Errorf("计算供货价格失败: %v", err)
		return nil, fmt.Errorf("计算供货价格失败: %w", err)
	}

	s.logger.Infof("成功计算 %d 个SKC的价格", len(response.Info))
	return response, nil
}

// CreateTimeLimitedDiscount 创建限时折扣活动
func (s *activityRegistrationServiceImpl) CreateTimeLimitedDiscount(
	ctx context.Context,
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

// AutoCreateTimeLimitedDiscount 自动创建限时折扣活动（完整流程）
func (s *activityRegistrationServiceImpl) AutoCreateTimeLimitedDiscount(
	ctx context.Context,
	config TimeLimitedDiscountConfig,
) error {
	s.logger.WithField("activity_name", config.ActivityName).Info("开始自动创建限时折扣活动")

	// 1. 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 2. 查询可参加活动的商品
	queryReq := s.buildQueryRequest(config)
	queryResp, err := s.QueryPromotionGoods(ctx, queryReq)
	if err != nil {
		return fmt.Errorf("查询商品失败: %w", err)
	}

	if queryResp.Info == nil || len(queryResp.Info.Data) == 0 {
		s.logger.Warn("没有可参加活动的商品")
		return ErrNoAvailableProducts
	}

	s.logger.Infof("查询到 %d 个可参加活动的商品", len(queryResp.Info.Data))

	// 3. 计算商品价格和利润
	calcReq := s.buildCalculateRequest(config, queryResp.Info.Data)
	calcResp, err := s.CalculateSupplyPrice(ctx, calcReq)
	if err != nil {
		return fmt.Errorf("计算价格失败: %w", err)
	}

	// 4. 检查价格风险
	if err := s.validatePriceRisk(calcResp, config); err != nil {
		return fmt.Errorf("价格风险检查失败: %w", err)
	}

	// 5. 构建活动创建请求
	createReq := s.buildCreateActivityRequest(config, queryResp.Info.Data, calcResp)

	// 6. 创建限时折扣活动
	createResp, err := s.CreateTimeLimitedDiscount(ctx, createReq)
	if err != nil {
		return fmt.Errorf("创建活动失败: %w", err)
	}

	// 7. 检查创建结果
	if err := s.checkCreateResult(createResp); err != nil {
		return fmt.Errorf("活动创建结果异常: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"activity_id":   createResp.Info.ActivityID,
		"product_count": len(queryResp.Info.Data),
	}).Info("限时折扣活动创建成功")

	return nil
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

// buildCalculateRequest 构建价格计算请求
func (s *activityRegistrationServiceImpl) buildCalculateRequest(
	config TimeLimitedDiscountConfig,
	goods []marketing.PromotionGoodsData,
) *marketing.CalculateSupplyPriceRequest {
	skcInfoList := make([]marketing.SkcPriceInfo, 0, len(goods))

	for _, g := range goods {
		skuInfoList := make([]marketing.SkuPriceInfo, 0, len(g.SkuInfoList))
		for _, sku := range g.SkuInfoList {
			skuInfoList = append(skuInfoList, marketing.SkuPriceInfo{
				SkuCode:       sku.Sku,
				ProductPrice:  g.USSupplyPrice,
				DiscountValue: g.USSupplyPrice * 0.6, // 默认6折
			})
		}

		skcInfoList = append(skcInfoList, marketing.SkcPriceInfo{
			SkcName:     g.Skc,
			SkuInfoList: skuInfoList,
		})
	}

	return &marketing.CalculateSupplyPriceRequest{
		Currency:      config.Currency,
		RefToolID:     config.RefToolID,
		SceneID:       config.SceneID,
		SkcInfoList:   skcInfoList,
		TimeZone:      config.TimeZone,
		ZoneStartTime: config.StartTime.Format("2006-01-02 15:04:05"),
		ZoneEndTime:   config.EndTime.Format("2006-01-02 15:04:05"),
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
				s.logger.Warnf("SKU %s 警告值过高: %d", skuInfo.SkuCode, skuInfo.WarningValue)
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
	costAndStockList := make([]marketing.CostAndStockInfo, 0, len(goods))

	for _, g := range goods {
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
		stockNum := config.DefaultStockNum
		if g.InventoryNum > 0 && g.InventoryNum < stockNum {
			stockNum = g.InventoryNum
		}

		costAndStockList = append(costAndStockList, marketing.CostAndStockInfo{
			Skc:                g.Skc,
			AttendNum:          config.DefaultAttendNum,
			StockNum:           stockNum,
			CenterList:         config.EffectiveCenterList,
			IsSaleAttribute:    g.IsSaleAttribute,
			PromotionIDList:    nil,
			CostPrice:          g.USSupplyPrice,
			MaxProductActPrice: g.MaxUSSupplyPrice,
			ProductActPrice:    g.USSupplyPrice * 0.6, // 6折
			AddSkuList:         addSkuList,
		})
	}

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
