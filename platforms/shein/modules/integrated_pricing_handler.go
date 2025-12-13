// Package modules 提供SHEIN平台与通用核价系统的集成。
package modules

import (
	"context"
	"fmt"
	"time"

	"task-processor/common/management"
	managementapi "task-processor/common/management/api"
	"task-processor/common/pricing"
	"task-processor/common/pricing/model"
	"task-processor/common/pricing/service"
	shops "task-processor/platforms/shein/client"
	sheinapi "task-processor/platforms/shein/client/api"
	sheinpricing "task-processor/platforms/shein/client/api/pricing"

	"github.com/sirupsen/logrus"
)

// IntegratedAutoPricingHandler 集成通用核价系统的SHEIN自动核价处理器
type IntegratedAutoPricingHandler struct {
	shopClientMgr *shops.ClientManager
	storeClient   *management.ClientManager
	commonService service.CommonPricingService
	sheinAdapter  *pricing.SheinAdapter
	logger        *logrus.Entry
}

// NewIntegratedAutoPricingHandler 创建新的集成自动核价处理器
func NewIntegratedAutoPricingHandler(shopClientMgr *shops.ClientManager, storeClient *management.ClientManager) *IntegratedAutoPricingHandler {
	// 创建服务工厂
	factory := pricing.NewServiceFactory(storeClient)

	// 创建通用核价服务
	commonService := factory.CreateCommonPricingService()

	// 获取SHEIN适配器
	sheinAdapter := factory.CreateSheinAdapter()

	return &IntegratedAutoPricingHandler{
		shopClientMgr: shopClientMgr,
		storeClient:   storeClient,
		commonService: commonService,
		sheinAdapter:  sheinAdapter,
		logger:        logrus.WithField("component", "IntegratedAutoPricingHandler"),
	}
}

// Start 启动自动核价任务
func (h *IntegratedAutoPricingHandler) Start(ctx context.Context, interval time.Duration) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Errorf("自动核价任务panic: %v", r)
			}
		}()

		// 立即执行一次
		h.performAutoPricing(ctx)

		// 之后按指定间隔执行
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.performAutoPricing(ctx)
			case <-ctx.Done():
				h.logger.Info("自动核价任务收到停止信号，退出")
				return
			}
		}
	}()
}

// performAutoPricing 执行自动核价操作
func (h *IntegratedAutoPricingHandler) performAutoPricing(ctx context.Context) {
	h.logger.Info("开始执行SHEIN集成自动核价任务")

	tenantShops := h.shopClientMgr.GetTenantShopPairs()
	h.logger.Infof("获取到 %d 个租户店铺对", len(tenantShops))

	if len(tenantShops) == 0 {
		return
	}

	for _, tenantShop := range tenantShops {
		h.logger.Infof("处理租户 %d 的店铺 %d", tenantShop.TenantID, tenantShop.ShopID)

		if err := h.processStoreProducts(ctx, tenantShop.TenantID, tenantShop.ShopID); err != nil {
			h.logger.Errorf("处理店铺 %d 失败: %v", tenantShop.ShopID, err)
			continue
		}
	}

	h.logger.Info("SHEIN集成自动核价任务执行完成")
}

// processStoreProducts 处理店铺商品
func (h *IntegratedAutoPricingHandler) processStoreProducts(ctx context.Context, tenantID, shopID int64) error {
	// 获取店铺信息
	storeInfo, err := h.storeClient.GetStoreClient().GetStore(shopID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	h.logger.Infof("店铺 %s (ID: %d) 自动核价状态: %v", storeInfo.Name, storeInfo.ID,
		storeInfo.EnableAutoPrice != nil && *storeInfo.EnableAutoPrice)

	// 检查是否启用自动核价
	if storeInfo.EnableAutoPrice == nil || !*storeInfo.EnableAutoPrice {
		h.logger.Infof("店铺 %s 未启用自动核价功能，跳过", storeInfo.Name)
		return nil
	}

	// 获取待核价的产品列表
	products, err := h.getPendingPricingProducts(ctx, tenantID, shopID, storeInfo)
	if err != nil {
		return fmt.Errorf("获取待核价产品失败: %w", err)
	}

	if len(products) == 0 {
		h.logger.Infof("店铺 %s 没有待核价的产品", storeInfo.Name)
		return nil
	}

	h.logger.Infof("店铺 %s 获取到 %d 个待核价产品", storeInfo.Name, len(products))

	// 使用通用核价服务批量处理
	return h.processBatchPricing(ctx, tenantID, shopID, storeInfo, products)
}

// processBatchPricing 批量处理核价
func (h *IntegratedAutoPricingHandler) processBatchPricing(ctx context.Context, tenantID, shopID int64, storeInfo *managementapi.StoreRespDTO, products []sheinpricing.BargainPageData) error {
	// 转换为通用核价上下文
	var pricingContexts []*model.PricingContext

	for _, product := range products {
		contexts := h.sheinAdapter.ConvertFromSheinBargain(&product, shopID, tenantID)
		for _, pricingCtx := range contexts {
			pricingCtx.Ctx = ctx

			// 加载店铺配置和核价规则
			if err := h.loadContextData(ctx, pricingCtx, storeInfo); err != nil {
				h.logger.Errorf("加载上下文数据失败: %v", err)
				continue
			}

			pricingContexts = append(pricingContexts, pricingCtx)
		}
	}

	if len(pricingContexts) == 0 {
		h.logger.Warn("没有有效的核价上下文")
		return nil
	}

	// 使用通用核价服务批量处理
	batchResult, err := h.commonService.ProcessBatchProducts(ctx, pricingContexts)
	if err != nil {
		return fmt.Errorf("批量核价处理失败: %w", err)
	}

	h.logger.Infof("批量核价完成: 总数=%d, 成功=%d, 失败=%d, 跳过=%d, 耗时=%v",
		batchResult.TotalCount, batchResult.SuccessCount,
		batchResult.FailCount, batchResult.SkipCount, batchResult.Duration)

	// 执行批量决策
	return h.executeBatchDecisions(ctx, tenantID, shopID, storeInfo, batchResult.Results)
}

// executeBatchDecisions 执行批量决策
func (h *IntegratedAutoPricingHandler) executeBatchDecisions(ctx context.Context, tenantID, shopID int64, storeInfo *managementapi.StoreRespDTO, results []*model.PricingResult) error {
	// 获取SHEIN客户端
	client, err := h.shopClientMgr.GetClient(tenantID, shopID, storeInfo)
	if err != nil {
		return fmt.Errorf("获取SHEIN客户端失败: %w", err)
	}

	// 创建批量请求
	batchReq := h.sheinAdapter.CreateBatchRequest(results)
	if batchReq == nil {
		h.logger.Info("没有需要处理的决策")
		return nil
	}

	// 执行批量处理
	return h.handleCostDiscuss(client, batchReq)
}

// loadContextData 加载核价上下文数据
func (h *IntegratedAutoPricingHandler) loadContextData(ctx context.Context, pricingCtx *model.PricingContext, storeInfo *managementapi.StoreRespDTO) error {
	// 转换店铺配置
	pricingCtx.StoreConfig = &model.StoreConfig{
		ID:                       storeInfo.ID,
		Name:                     storeInfo.Name,
		EnableAutoPrice:          storeInfo.EnableAutoPrice,
		SheinPriceRejectStrategy: "TAKE_OFFLINE", // SHEIN默认下架策略
	}

	// 加载核价规则
	pricingRules, err := h.loadPricingRules(ctx, pricingCtx.StoreID)
	if err != nil {
		h.logger.Warnf("加载核价规则失败: %v，将使用默认规则", err)
		// 使用默认规则
		pricingRules = []model.PricingRule{
			{
				Name:      "SHEIN默认规则",
				RuleType:  model.RuleTypePercent,
				RuleValue: floatPtr(0.3), // 30%利润率
				Status:    0,
			},
		}
	}
	pricingCtx.PricingRules = pricingRules

	return nil
}

// loadPricingRules 加载核价规则
func (h *IntegratedAutoPricingHandler) loadPricingRules(ctx context.Context, storeID int64) ([]model.PricingRule, error) {
	pricingRuleAPI := h.storeClient.GetPricingRuleClient()
	reqDto := &managementapi.PricingRuleReqDTO{StoreID: &storeID}

	apiRule, err := pricingRuleAPI.GetPricingRule(reqDto)
	if err != nil {
		return nil, fmt.Errorf("获取定价规则失败: %w", err)
	}

	if apiRule == nil {
		return nil, fmt.Errorf("未找到定价规则")
	}

	// 转换为统一模型
	rule := model.PricingRule{}
	rule.FromManagementAPI(apiRule)

	return []model.PricingRule{rule}, nil
}

// getPendingPricingProducts 获取待核价的产品列表
func (h *IntegratedAutoPricingHandler) getPendingPricingProducts(ctx context.Context, tenantID, shopID int64, storeInfo *managementapi.StoreRespDTO) ([]sheinpricing.BargainPageData, error) {
	var allProducts []sheinpricing.BargainPageData

	// 获取店铺API客户端
	shopClient, err := h.shopClientMgr.GetClient(tenantID, shopID, storeInfo)
	if err != nil {
		return nil, fmt.Errorf("获取租户%d店铺%d的API客户端失败: %w", tenantID, shopID, err)
	}

	h.logger.Infof("成功获取租户%d店铺%d的API客户端", tenantID, shopID)

	// 分页获取所有待处理的议价数据
	pageNum := 1
	const pageSize = 100

	for {
		select {
		case <-ctx.Done():
			return allProducts, ctx.Err()
		default:
		}

		req := &sheinpricing.PageRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		}

		response, err := shopClient.BargainPage(req, 1)
		if err != nil {
			h.logger.Errorf("获取租户%d店铺%d的议价页面数据失败(页面%d): %v", tenantID, shopID, pageNum, err)
			break
		}

		h.logger.Debugf("租户%d店铺%d页面%d获取到%d个议价数据", tenantID, shopID, pageNum, len(response.Info.Data))

		allProducts = append(allProducts, response.Info.Data...)

		// 如果当前页数据少于页面大小，说明已经到最后一页
		if len(response.Info.Data) < pageSize {
			break
		}
		pageNum++
	}

	return allProducts, nil
}

// handleCostDiscuss 处理成本讨论
func (h *IntegratedAutoPricingHandler) handleCostDiscuss(client sheinapi.APIClient, req *sheinpricing.BatchHandleCostDiscussRequest) error {
	response, err := client.BatchHandleCostDiscuss(req)
	if err != nil {
		return fmt.Errorf("调用批量处理成本讨论接口失败: %w", err)
	}

	if response.Code != "0" {
		return fmt.Errorf("批量处理成本讨论接口返回错误: %s", response.Msg)
	}

	h.logger.Infof("成功处理成本讨论，成功处理数量: %d", response.Info.SuccessCount)
	return nil
}

// 辅助函数
func floatPtr(f float64) *float64 {
	return &f
}
