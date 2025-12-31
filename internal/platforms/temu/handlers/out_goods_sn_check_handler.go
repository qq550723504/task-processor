package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// OutGoodsSnCheckHandler SKU编码批量检查处理器
type OutGoodsSnCheckHandler struct {
	logger *logrus.Entry
}

// OutSkuSnItem SKU编码项
type OutSkuSnItem struct {
	OutSkuSn string `json:"out_sku_sn"`
}

// OutGoodsSnCheckRequest SKU编码批量检查请求结构体
type OutGoodsSnCheckRequest struct {
	GoodsID   string         `json:"goods_id"`
	OutSnList []OutSkuSnItem `json:"out_sn_list"`
}

// OutGoodsSnCheckResponse SKU编码批量检查响应结构体
type OutGoodsSnCheckResponse struct {
	Success   bool                   `json:"success"`
	ErrorCode int                    `json:"error_code"`
	Result    *OutGoodsSnCheckResult `json:"result,omitempty"`
	Message   string                 `json:"error_msg,omitempty"`
}

// OutGoodsSnCheckResult SKU编码检查结果
type OutGoodsSnCheckResult struct {
	FailList []OutSkuSnFailItem `json:"fail_list"`
}

// OutSkuSnFailItem 失败的SKU编码项
type OutSkuSnFailItem struct {
	OutSkuSn    string `json:"out_sku_sn"`
	UsedGoodsID string `json:"used_goods_id"`
	UsedSkuID   string `json:"used_sku_id"`
	FailReason  string `json:"fail_reason"`
}

// NewOutGoodsSnCheckHandler 创建新的SKU编码检查处理器
func NewOutGoodsSnCheckHandler() *OutGoodsSnCheckHandler {
	return &OutGoodsSnCheckHandler{
		logger: logrus.WithField("handler", "OutGoodsSnCheckHandler"),
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

	h.logger.Infof("收集到 %d 个SKU编码，开始检查", len(outSkuSnList))

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
func (h *OutGoodsSnCheckHandler) checkOutSkuSn(temuCtx *temucontext.TemuTaskContext, outSkuSnList []OutSkuSnItem) error {
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

	// 构造请求体
	requestBody := OutGoodsSnCheckRequest{
		GoodsID:   temuProduct.GoodsBasic.GoodsID,
		OutSnList: outSkuSnList,
	}

	h.logger.WithFields(logrus.Fields{
		"goodsID":    requestBody.GoodsID,
		"outSnCount": len(requestBody.OutSnList),
	}).Info("发送SKU编码检查请求")

	// 构造API请求
	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/out_sku_sn_batch_check",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": requestBody,
	}

	// 类型断言获取TEMU API客户端
	type TEMUAPIClient interface {
		SendTEMURequest(request map[string]interface{}, response interface{}) error
	}

	// 发送API请求
	response := &OutGoodsSnCheckResponse{}

	// 直接使用APIClient，假设它实现了SendTEMURequest方法
	if temuClient, ok := interface{}(temuCtx.APIClient).(TEMUAPIClient); ok {
		err := temuClient.SendTEMURequest(apiReq, response)
		if err != nil {
			h.logger.WithError(err).Error("SKU编码检查API调用失败")
			return fmt.Errorf("SKU编码检查API调用失败: %w", err)
		}
	} else {
		return fmt.Errorf("API客户端不支持TEMU请求")
	}

	h.logger.WithFields(logrus.Fields{
		"success":   response.Success,
		"errorCode": response.ErrorCode,
	}).Info("SKU编码检查API响应")

	if !response.Success {
		errorMsg := fmt.Sprintf("SKU编码检查失败，API返回失败状态 (错误码: %d)", response.ErrorCode)
		if response.Message != "" {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, response.Message)
		}
		return fmt.Errorf("%s", errorMsg)
	}

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
func (h *OutGoodsSnCheckHandler) handleCheckResult(temuCtx *temucontext.TemuTaskContext, result *OutGoodsSnCheckResult) error {
	// 将检查结果存储到强类型上下文中，供其他处理器使用
	// 这里可以添加一个字段到TemuTaskContext来存储检查结果
	// 暂时先不存储，直接处理

	if len(result.FailList) == 0 {
		h.logger.Info("所有SKU编码检查通过，没有重复")
		return nil
	}

	// 记录失败的SKU编码
	h.logger.Errorf("发现 %d 个重复的SKU编码，任务失败", len(result.FailList))

	var duplicateErrors []string
	for _, failItem := range result.FailList {
		errorMsg := fmt.Sprintf("SKU编码 '%s' 已被商品 %s 使用 (SKU ID: %s): %s",
			failItem.OutSkuSn, failItem.UsedGoodsID, failItem.UsedSkuID, failItem.FailReason)

		h.logger.WithFields(logrus.Fields{
			"outSkuSn":    failItem.OutSkuSn,
			"usedGoodsID": failItem.UsedGoodsID,
			"usedSkuID":   failItem.UsedSkuID,
			"failReason":  failItem.FailReason,
		}).Error("SKU编码重复")

		duplicateErrors = append(duplicateErrors, errorMsg)
	}

	// 直接返回错误，中断流程
	return fmt.Errorf("发现SKU编码重复，请检查并修改重复的编码: %v", duplicateErrors)
}

// generateOutSkuSnFromAmazon 从Amazon数据生成OutSkuSN列表进行检查
func (h *OutGoodsSnCheckHandler) generateOutSkuSnFromAmazon(temuCtx *temucontext.TemuTaskContext) []OutSkuSnItem {
	var outSkuSnList []OutSkuSnItem

	// 获取店铺配置（前缀、后缀、策略）
	prefix := ""
	suffix := ""
	strategy := utils.StrategyASINOnly // 默认策略：仅使用ASIN

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
		outSkuSN := utils.GenerateSKU(amazonProduct.Asin, strategy, prefix, suffix)
		if !outSkuSnMap[outSkuSN] {
			outSkuSnMap[outSkuSN] = true
			outSkuSnList = append(outSkuSnList, OutSkuSnItem{
				OutSkuSn: outSkuSN,
			})
			h.logger.Debugf("主产品 ASIN=%s -> OutSkuSN=%s", amazonProduct.Asin, outSkuSN)
		}
	}

	// 2. 为所有变体生成OutSkuSN
	variants := temuCtx.GetVariants()
	if len(variants) > 0 {
		h.logger.Debugf("为 %d 个变体生成OutSkuSN", len(variants))
		for _, variant := range variants {
			if variant.Asin != "" {
				outSkuSN := utils.GenerateSKU(variant.Asin, strategy, prefix, suffix)
				if !outSkuSnMap[outSkuSN] {
					outSkuSnMap[outSkuSN] = true
					outSkuSnList = append(outSkuSnList, OutSkuSnItem{
						OutSkuSn: outSkuSN,
					})
					h.logger.Debugf("变体 ASIN=%s -> OutSkuSN=%s", variant.Asin, outSkuSN)
				} else {
					h.logger.Debugf("跳过重复的SKU: ASIN=%s -> OutSkuSN=%s", variant.Asin, outSkuSN)
				}
			}
		}
	}

	h.logger.Infof("从Amazon数据生成了 %d 个唯一的OutSkuSN", len(outSkuSnList))
	return outSkuSnList
}
