package product

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/skugen"
	temuapi "task-processor/internal/platforms/temu/api"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// OutGoodsSnCheckHandler SKU编码批量检查处理器
type OutGoodsSnCheckHandler struct {
	logger *logrus.Entry
}

// NewOutGoodsSnCheckHandler 创建新的SKU编码检查处理器
func NewOutGoodsSnCheckHandler() *OutGoodsSnCheckHandler {
	return &OutGoodsSnCheckHandler{
		logger: logger.GetGlobalLogger("temu.handlers.out_goods_sn_check").WithField("handler", "OutGoodsSnCheckHandler"),
	}
}

// Name 返回处理器名称
func (h *OutGoodsSnCheckHandler) Name() string {
	return "SKU编码批量检查处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *OutGoodsSnCheckHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *OutGoodsSnCheckHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始检查SKU编码")

	// 获取任务信息
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 从Amazon数据生成OutSkuSN进行检查
	h.logger.Debug("从Amazon数据生成SKU编码进行检查")
	outSkuSnList := h.generateOutSkuSnFromAmazon(temuCtx)

	if len(outSkuSnList) == 0 {
		h.logger.Info("没有找到SKU编码，跳过检查")
		return nil
	}

	h.logger.WithField("sku_count", len(outSkuSnList)).Info("收集到SKU编码，开始检查")

	// 执行SKU编码检查
	err := h.checkOutSkuSn(temuCtx, outSkuSnList)
	if err != nil {
		h.logger.WithError(err).Error("SKU编码检查失败")
		return fmt.Errorf("SKU编码检查失败: %w", err)
	}

	h.logger.Info("SKU编码检查完成")
	return nil
}

// checkOutSkuSn 执行SKU编码检查
func (h *OutGoodsSnCheckHandler) checkOutSkuSn(temuCtx *temucontext.TemuTaskContext, outSkuSnList []temuapi.OutSkuSnItem) error {
	// 获取API客户端
	if temuCtx.APIClient == nil {
		h.logger.Error("API客户端未初始化，无法执行SKU编码检查")
		return fmt.Errorf("API客户端未初始化，无法执行SKU编码检查")
	}

	// 从强类型上下文获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 创建QueryAPI实例
	queryAPI := temuapi.NewQueryAPI(temuCtx.APIClient, h.logger)

	// 构造请求体
	request := &temuapi.SkuSnCheckRequest{
		GoodsID:   temuProduct.GoodsBasic.GoodsID,
		OutSnList: outSkuSnList,
	}

	h.logger.WithFields(logrus.Fields{
		"goodsID":    request.GoodsID,
		"outSnCount": len(request.OutSnList),
	}).Info("发送SKU编码检查请求")

	// 发送API请求
	response, err := queryAPI.CheckSkuSn(request)
	if err != nil {
		h.logger.WithError(err).Error("SKU编码检查API调用失败")
		return fmt.Errorf("SKU编码检查API调用失败: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"success":   response.Success,
		"errorCode": response.ErrorCode,
	}).Info("SKU编码检查API响应")

	// 处理检查结果
	if response.Result != nil {
		err := h.handleCheckResult(temuCtx, response.Result)
		if err != nil {
			return err
		}
	}

	h.logger.Info("SKU编码检查成功")
	return nil
}

// handleCheckResult 处理检查结果
func (h *OutGoodsSnCheckHandler) handleCheckResult(temuCtx *temucontext.TemuTaskContext, result *temuapi.OutGoodsSnCheckResult) error {
	// 将检查结果存储到强类型上下文中，供其他处理器使用
	// 这里可以添加一个字段到TemuTaskContext来存储检查结果
	// 暂时先不存储，直接处理

	if len(result.FailList) == 0 {
		h.logger.Info("所有SKU编码检查通过，没有重复")
		return nil
	}

	// 记录失败的SKU编码
	h.logger.WithField("fail_count", len(result.FailList)).Error("发现重复的SKU编码，任务失败")

	var duplicateErrors []string
	for _, failItem := range result.FailList {
		errorMsg := fmt.Sprintf("SKU编码 '%s' 已被商品 %s 使用 (SKU ID: %s): %s",
			failItem.OutSkuSn, failItem.UsedGoodsID, failItem.UsedSkuID, failItem.FailReason)

		h.logger.WithFields(map[string]any{
			"out_sku_sn":    failItem.OutSkuSn,
			"used_goods_id": failItem.UsedGoodsID,
			"used_sku_id":   failItem.UsedSkuID,
			"fail_reason":   failItem.FailReason,
		}).Error("SKU编码重复")

		duplicateErrors = append(duplicateErrors, errorMsg)
	}

	// 直接返回错误，中断流程
	return fmt.Errorf("发现SKU编码重复，请检查并修改重复的编码: %v", duplicateErrors)
}

// generateOutSkuSnFromAmazon 从Amazon数据生成OutSkuSN列表进行检查
func (h *OutGoodsSnCheckHandler) generateOutSkuSnFromAmazon(temuCtx *temucontext.TemuTaskContext) []temuapi.OutSkuSnItem {
	var outSkuSnList []temuapi.OutSkuSnItem

	// 获取店铺配置（前缀、后缀、策略）
	prefix := ""
	suffix := ""
	strategy := skugen.StrategyASINOnly // 默认策略：仅使用ASIN

	// 从强类型上下文获取店铺信息
	if temuCtx.StoreInfo != nil {
		if temuCtx.StoreInfo.Prefix != "" {
			prefix = temuCtx.StoreInfo.Prefix
		}
		if temuCtx.StoreInfo.Suffix != "" {
			suffix = temuCtx.StoreInfo.Suffix
		}
	}

	h.logger.Debugf("使用SKU生成策略: strategy=%d, prefix=%s, suffix=%s", strategy, prefix, suffix)

	// 使用 map 去重
	outSkuSnMap := make(map[string]bool)

	// 1. 为主产品生成OutSkuSN
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil && amazonProduct.Asin != "" {
		outSkuSN := skugen.Generate(amazonProduct.Asin, strategy, prefix, suffix)
		if !outSkuSnMap[outSkuSN] {
			outSkuSnMap[outSkuSN] = true
			outSkuSnList = append(outSkuSnList, temuapi.OutSkuSnItem{
				OutSkuSn: outSkuSN,
			})
			h.logger.WithFields(map[string]any{
				"asin":       amazonProduct.Asin,
				"out_sku_sn": outSkuSN,
			}).Debug("主产品SKU生成")
		}
	}

	// 2. 为所有变体生成OutSkuSN
	variants := temuCtx.GetVariants()
	if len(variants) > 0 {
		h.logger.WithField("variant_count", len(variants)).Debug("为变体生成OutSkuSN")
		for _, variant := range variants {
			if variant.Asin != "" {
				outSkuSN := skugen.Generate(variant.Asin, strategy, prefix, suffix)
				if !outSkuSnMap[outSkuSN] {
					outSkuSnMap[outSkuSN] = true
					outSkuSnList = append(outSkuSnList, temuapi.OutSkuSnItem{
						OutSkuSn: outSkuSN,
					})
					h.logger.WithFields(map[string]any{
						"asin":       variant.Asin,
						"out_sku_sn": outSkuSN,
					}).Debug("变体SKU生成")
				} else {
					h.logger.WithFields(map[string]any{
						"asin":       variant.Asin,
						"out_sku_sn": outSkuSN,
					}).Debug("跳过重复的SKU")
				}
			}
		}
	}

	h.logger.WithField("unique_count", len(outSkuSnList)).Info("从Amazon数据生成唯一的OutSkuSN")
	return outSkuSnList
}
