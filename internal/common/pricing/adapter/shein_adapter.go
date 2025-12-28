// Package adapter 提供SHEIN平台的核价适配器实现。
package adapter

import (
	"context"
	"fmt"
	"task-processor/internal/common/pricing/model"
	"task-processor/internal/common/pricing/service"
	sheinpricing "task-processor/internal/common/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// SheinAdapter SHEIN平台适配器
type SheinAdapter struct {
	costProvider service.CostPriceProvider
	logger       *logrus.Entry
}

// NewSheinAdapter 创建SHEIN适配器
func NewSheinAdapter(costProvider service.CostPriceProvider) *SheinAdapter {
	return &SheinAdapter{
		costProvider: costProvider,
		logger:       logrus.WithField("component", "SheinAdapter"),
	}
}

// GetPlatformName 获取平台名称
func (a *SheinAdapter) GetPlatformName() string {
	return "shein"
}

// PreprocessContext 预处理核价上下文
func (a *SheinAdapter) PreprocessContext(ctx context.Context, pricingCtx *model.PricingContext) error {
	// SHEIN特有的预处理逻辑

	// 如果没有原始成本价，尝试通过成本价提供者获取
	if !pricingCtx.HasValidCostPrice() {
		originCostPrice, err := a.costProvider.GetOriginCostPrice(
			ctx, pricingCtx.ProductID, pricingCtx.StoreID)
		if err != nil {
			a.logger.Warnf("获取成本价失败: %v", err)
		} else {
			pricingCtx.OriginCostPrice = originCostPrice
			a.logger.Infof("SHEIN商品 %s 获取成本价: %.2f",
				pricingCtx.ProductName, originCostPrice)
		}
	}

	// 处理SHEIN特有的平台数据
	if platformData, ok := pricingCtx.PlatformData.(map[string]interface{}); ok {
		// 处理议价数据
		if bargainData, exists := platformData["bargain_data"]; exists {
			a.logger.Infof("SHEIN商品 %s 为议价场景", pricingCtx.ProductName)

			// 从议价数据中提取更多信息
			if data, ok := bargainData.(*sheinpricing.BargainPageData); ok {
				// 可以从议价数据中提取更多上下文信息
				a.logger.Debugf("议价单号: %s, 文档号: %s", data.BargainSn, data.DocumentSn)
			}
		}
	}

	return nil
}

// PostprocessResult 后处理核价结果
func (a *SheinAdapter) PostprocessResult(ctx context.Context, result *model.PricingResult) error {
	// SHEIN特有的后处理逻辑

	// 为SHEIN平台添加特定的结果数据
	sheinData := map[string]interface{}{
		"platform":     "shein",
		"processed_at": result.Timestamp,
	}

	// SHEIN不支持重新申诉，将重新申诉转换为拒绝
	if result.Action == model.ActionReappeal {
		result.Action = model.ActionReject
		result.Reason = "SHEIN平台不支持重新申诉，转为拒绝处理"
		sheinData["converted_from_reappeal"] = true
	}

	result.PlatformData = sheinData

	a.logger.Debugf("SHEIN商品 %s 后处理完成", result.ProductName)
	return nil
}

// ExecuteDecision 执行核价决策
func (a *SheinAdapter) ExecuteDecision(ctx context.Context, result *model.PricingResult) error {
	// SHEIN平台特有的决策执行逻辑

	switch result.Action {
	case model.ActionAccept:
		return a.executeAccept(ctx, result)
	case model.ActionReject:
		return a.executeReject(ctx, result)
	case model.ActionSkip:
		a.logger.Infof("SHEIN商品 %s 跳过处理: %s", result.ProductName, result.Reason)
		return nil
	default:
		return fmt.Errorf("SHEIN平台不支持的决策动作: %s", result.Action)
	}
}

// executeAccept 执行接受决策
func (a *SheinAdapter) executeAccept(ctx context.Context, result *model.PricingResult) error {
	a.logger.Infof("SHEIN商品 %s 接受价格: %.2f", result.ProductName, result.SuggestPrice)

	// 这里应该调用SHEIN的批量处理成本讨论API
	// 创建接受请求
	// req := &pricing.BatchHandleCostDiscussRequest{
	//     ConfirmInfos: &[]pricing.ConfirmInfo{
	//         {
	//             DiscussAuditType: 1, // 1表示接受
	//             DiscussSn:        bargainSn,
	//             DocumentSn:       documentSn,
	//         },
	//     },
	// }
	// sheinClient.BatchHandleCostDiscuss(req)

	return nil
}

// executeReject 执行拒绝决策
func (a *SheinAdapter) executeReject(ctx context.Context, result *model.PricingResult) error {
	a.logger.Infof("SHEIN商品 %s 拒绝价格: %.2f", result.ProductName, result.SuggestPrice)

	// 这里应该调用SHEIN的批量处理成本讨论API
	// 创建拒绝请求
	// req := &pricing.BatchHandleCostDiscussRequest{
	//     ConfirmInfos: &[]pricing.ConfirmInfo{
	//         {
	//             DiscussAuditType: 2, // 2表示拒绝
	//             DiscussSn:        bargainSn,
	//             DocumentSn:       documentSn,
	//         },
	//     },
	// }
	// sheinClient.BatchHandleCostDiscuss(req)

	return nil
}

// ConvertFromSheinBargain 从SHEIN议价数据转换为通用核价上下文
func (a *SheinAdapter) ConvertFromSheinBargain(bargainData *sheinpricing.BargainPageData, storeID int64, tenantID int64) []*model.PricingContext {
	var contexts []*model.PricingContext

	// SHEIN的议价数据包含多个SKU
	for _, sku := range bargainData.SkuCostPrices {
		if len(sku.CostPriceHistories) == 0 {
			continue
		}

		// 获取当前成本价和建议成本价
		currentPrice := sku.CostPriceHistories[0].CostPrice
		suggestPrice := sku.SuggestCostPrice

		context := &model.PricingContext{
			TenantID:     tenantID,
			StoreID:      storeID,
			ProductID:    sku.SkuCode,
			SkuID:        sku.SkuCode,
			ProductName:  fmt.Sprintf("议价商品-%s", bargainData.BargainSn),
			CurrentPrice: currentPrice,
			SuggestPrice: suggestPrice,
			PlatformData: map[string]interface{}{
				"platform":     "shein",
				"bargain_data": bargainData,
				"sku_data":     sku,
				"bargain_sn":   bargainData.BargainSn,
				"document_sn":  bargainData.DocumentSn,
			},
		}

		contexts = append(contexts, context)
	}

	return contexts
}

// CreateBatchRequest 创建SHEIN批量处理请求
func (a *SheinAdapter) CreateBatchRequest(results []*model.PricingResult) *sheinpricing.BatchHandleCostDiscussRequest {
	var confirmInfos []sheinpricing.ConfirmInfo

	for _, result := range results {
		if result.Action == model.ActionSkip {
			continue
		}

		// 从平台数据中提取必要信息
		platformData, ok := result.PlatformData.(map[string]interface{})
		if !ok {
			continue
		}

		bargainSn, _ := platformData["bargain_sn"].(string)
		documentSn, _ := platformData["document_sn"].(string)

		if bargainSn == "" || documentSn == "" {
			continue
		}

		var auditType int
		switch result.Action {
		case model.ActionAccept:
			auditType = 1 // 接受
		case model.ActionReject:
			auditType = 2 // 拒绝
		default:
			continue
		}

		confirmInfo := sheinpricing.ConfirmInfo{
			DiscussAuditType: auditType,
			DiscussSn:        bargainSn,
			DocumentSn:       documentSn,
		}

		confirmInfos = append(confirmInfos, confirmInfo)
	}

	if len(confirmInfos) == 0 {
		return nil
	}

	return &sheinpricing.BatchHandleCostDiscussRequest{
		ConfirmInfos: &confirmInfos,
	}
}
