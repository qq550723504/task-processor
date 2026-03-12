package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"task-processor/internal/application/state"
	commontypes "task-processor/internal/domain/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/management/api"
	pkgproduct "task-processor/internal/pkg/product"
	"task-processor/internal/pkg/ptrutil"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// SavePublishResultHandler 保存发品成功后返回信息处理器（参考SHEIN实现）
type SavePublishResultHandler struct {
	mappingClient api.ProductImportMappingAPI
	memoryManager *state.MemoryManager
	logger        *logrus.Entry
}

// NewSavePublishResultHandler 创建新的保存发品成功后返回信息处理器
func NewSavePublishResultHandler(mappingClient api.ProductImportMappingAPI, memoryManager *state.MemoryManager) *SavePublishResultHandler {
	return &SavePublishResultHandler{
		mappingClient: mappingClient,
		memoryManager: memoryManager,
		logger:        logrus.WithField("handler", "SavePublishResultHandler"),
	}
}

// Name 返回处理器名称
func (h *SavePublishResultHandler) Name() string {
	return "保存发品成功后返回的信息"
}

// Handle 执行保存发品成功后返回信息处理（兼容pipeline.Handler接口）
func (h *SavePublishResultHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 执行保存发品成功后返回信息处理（强类型上下文）
func (h *SavePublishResultHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始保存发品成功后的信息")

	// 检查是否有提交响应数据（兼容两个字段）
	submitResponse := temuCtx.SubmitResponse
	if submitResponse == nil {
		submitResponse = temuCtx.SubmitResult
	}

	if submitResponse == nil {
		h.logger.Warn("TEMU提交响应数据为空，跳过保存")
		return nil
	}

	// 记录响应数据到日志
	if err := h.logSubmitResponse(temuCtx, submitResponse); err != nil {
		h.logger.Warnf("记录响应数据失败: %v", err)
		// 不阻断流程，继续执行
	}

	// 创建产品导入映射关系
	if err := h.createProductImportMapping(temuCtx); err != nil {
		h.logger.Warnf("创建产品导入映射关系失败: %v", err)
		// 不阻断流程，继续执行
	}

	// 记录每日上架成功数量并检查限额
	if err := h.recordDailyListingCount(temuCtx); err != nil {
		h.logger.Warnf("记录每日上架计数失败: %v", err)
		// 不阻断流程，继续执行
	}

	h.logger.Info("发品成功后返回信息保存完成")
	return nil
}

// createProductImportMapping 创建产品导入映射关系
func (h *SavePublishResultHandler) createProductImportMapping(temuCtx *temucontext.TemuTaskContext) error {
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息未初始化")
	}

	// 检查提交响应数据（兼容两个字段）
	submitResponse := temuCtx.SubmitResponse
	if submitResponse == nil {
		submitResponse = temuCtx.SubmitResult
	}

	if submitResponse == nil {
		h.logger.Warn("提交响应数据不存在")
		return nil
	}

	// 解析任务ID（任务ID已经是int64类型）
	taskID := task.ID

	// 检查产品数据
	var temuProduct *models.Product
	if temuCtx.ProductData != nil {
		if product, ok := temuCtx.ProductData.(*models.Product); ok {
			temuProduct = product
		}
	}

	// 如果ProductData为空，尝试使用TemuProduct
	if temuProduct == nil {
		temuProduct = temuCtx.TemuProduct
	}

	if temuProduct == nil {
		h.logger.Warn("产品数据不存在，无法创建映射关系")
		return nil
	}

	createdCount := 0

	// 遍历SKC和SKU列表创建映射关系
	if len(temuProduct.SkcList) > 0 {
		for _, skc := range temuProduct.SkcList {
			for _, sku := range skc.SkuList {
				createReq := &api.ProductImportMappingCreateReqDTO{
					ImportTaskId: taskID,
					TenantID:     task.TenantID,
					StoreId:      task.StoreID,
					Platform:     "TEMU",
					Region:       task.Region,
					Sku:          &sku.OutSkuSN,
					ProductId:    "",                  // 将在下面设置
					Status:       ptrutil.Int16Ptr(1), // 1表示导入成功
				}

				// 从AsinSkuMap中查找对应的ASIN
				if temuCtx.AsinSkuMap != nil {
					// 映射关系是 SKU -> ASIN
					if asin, exists := temuCtx.AsinSkuMap[sku.OutSkuSN]; exists {
						createReq.ProductId = asin
					}
				}

				// 设置父产品ASIN和成本价
				amazonProduct := temuCtx.GetAmazonProduct()
				if amazonProduct != nil && amazonProduct.ParentAsin != "" {
					createReq.ParentProductId = &amazonProduct.ParentAsin

					// 根据店铺设置获取成本价（原价或特价）- 使用公共函数
					if temuCtx.StoreInfo != nil && temuCtx.StoreInfo.PriceType != "" {
						costPrice := pkgproduct.GetProductPrice(amazonProduct, temuCtx.StoreInfo.PriceType)
						if costPrice > 0 {
							createReq.CostPrice = &costPrice
						}
					}
				}

				// 设置筛选规则信息
				if temuCtx.FilterRule != nil {
					createReq.FilterRuleId = &temuCtx.FilterRule.ID
					// 设置筛选规则范围，格式为 "价格范围:最小价格-最大价格"
					filterRuleRange := h.buildFilterRuleRange(temuCtx.FilterRule)
					if filterRuleRange != "" {
						createReq.FilterRuleRange = &filterRuleRange
					}
				}

				// 设置利润规则信息
				if temuCtx.ProfitRule != nil {
					createReq.ProfitRuleId = &temuCtx.ProfitRule.ID
					// 将float64转换为string指针
					if temuCtx.ProfitRule.SalePriceMultiplier > 0 {
						salePriceMultiplierStr := fmt.Sprintf("%.4f", temuCtx.ProfitRule.SalePriceMultiplier)
						createReq.SalePriceMultiplier = &salePriceMultiplierStr
					}
					if temuCtx.ProfitRule.DiscountPriceMultiplier > 0 {
						discountPriceMultiplierStr := fmt.Sprintf("%.4f", temuCtx.ProfitRule.DiscountPriceMultiplier)
						createReq.DiscountPriceMultiplier = &discountPriceMultiplierStr
					}
				}

				// 调用API创建映射关系
				_, err := h.mappingClient.CreateProductImportMapping(createReq)
				if err != nil {
					h.logger.Errorf("创建产品导入映射关系失败: OutSkuSn=%s, Error=%v", sku.OutSkuSN, err)
					continue
				}

				createdCount++
				h.logger.Debugf("成功创建产品导入映射关系: OutSkuSn=%s", sku.OutSkuSN)
			}
		}
	}

	h.logger.Infof("产品导入映射关系创建完成: 成功=%d", createdCount)
	return nil
}

// getStringValue 安全获取字符串指针的值
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// recordDailyListingCount 记录每日上架成功数量并检查限额（参考SHEIN实现）
func (h *SavePublishResultHandler) recordDailyListingCount(temuCtx *temucontext.TemuTaskContext) error {
	// 检查必要的上下文信息
	if h.memoryManager == nil {
		h.logger.Debug("内存管理器未初始化，跳过每日上架计数")
		return nil
	}

	task := temuCtx.GetTask()
	if task == nil {
		h.logger.Debug("任务信息未初始化，跳过每日上架计数")
		return nil
	}

	// 从强类型上下文获取店铺信息
	if temuCtx.StoreInfo == nil {
		h.logger.Debug("店铺信息未初始化，跳过每日上架计数")
		return nil
	}

	// 检查店铺是否有每日上架限额
	if temuCtx.StoreInfo.DailyLimit == nil || *temuCtx.StoreInfo.DailyLimit <= 0 {
		h.logger.Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", task.StoreID)
		return nil
	}

	dailyLimit := *temuCtx.StoreInfo.DailyLimit
	dailyLimitType := "SPU" // 默认值
	if temuCtx.StoreInfo.DailyLimitType != "" {
		dailyLimitType = temuCtx.StoreInfo.DailyLimitType
	}

	h.logger.Debugf("店铺 %d 的每日上架限额为: %d，限制类型: %s", task.StoreID, dailyLimit, dailyLimitType)

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := time.Now().Format("2006-01-02")

	// 根据店铺配置的限制类型计算增加的数量
	increment := h.calculateIncrementFromContext(temuCtx, dailyLimitType)
	if increment <= 0 {
		h.logger.Warnf("计算增量失败，跳过计数更新")
		return nil
	}

	// 增加每日上架计数
	count := h.memoryManager.DailyCountManager.IncrementCount(
		task.TenantID,
		task.StoreID,
		currentDate,
		increment,
	)

	h.logger.Infof("店铺 %d 在 %s 的上架计数: %d (本次增加: %d, 类型: %s)",
		task.StoreID, currentDate, count, increment, dailyLimitType)

	// 检查是否超过限额
	if count > int64(dailyLimit) {
		h.logger.Warnf("店铺 %d 在 %s 的上架数量(%d)已超过限额(%d)，将暂停上架", task.StoreID, currentDate, count, dailyLimit)

		// 暂停店铺上架到当日结束
		if err := h.pauseShopUntilEndOfDay(
			temuCtx,
			fmt.Sprintf("超过每日上架限额(%d/%d)", count, dailyLimit),
		); err != nil {
			h.logger.Errorf("暂停店铺上架失败: %v", err)
		}

		h.logger.Infof("已暂停店铺 %d 上架到当日结束，因为已超过每日限额 %d", task.StoreID, dailyLimit)
	} else {
		h.logger.Infof("店铺 %d 在 %s 的上架数量(%d)未超过限额(%d)", task.StoreID, currentDate, count, dailyLimit)
	}

	return nil
}

// calculateIncrementFromContext 根据店铺配置的限制类型计算增量
func (h *SavePublishResultHandler) calculateIncrementFromContext(temuCtx *temucontext.TemuTaskContext, dailyLimitType string) int64 {
	// 检查TEMU产品数据是否存在
	if temuCtx.TemuProduct == nil {
		h.logger.Warn("TEMU产品数据为空，无法计算增量")
		return 0
	}

	temuProduct := temuCtx.TemuProduct

	switch dailyLimitType {
	case "SPU":
		// SPU级别：每个产品算1个
		return 1
	case "SKC":
		// SKC级别：按SKC数量计算
		skcCount := int64(len(temuProduct.SkcList))
		h.logger.Debugf("SKC计数: %d", skcCount)
		return skcCount
	case "SKU":
		// SKU级别：按所有SKU数量计算
		var skuCount int64
		for _, skc := range temuProduct.SkcList {
			skuCount += int64(len(skc.SkuList))
		}
		h.logger.Debugf("SKU计数: %d", skuCount)
		return skuCount
	default:
		// 默认按SPU计算
		h.logger.Warnf("未知的限制类型: %s，默认按SPU计算", dailyLimitType)
		return 1
	}
}

// pauseShopUntilEndOfDay 暂停店铺到当日结束
func (h *SavePublishResultHandler) pauseShopUntilEndOfDay(temuCtx *temucontext.TemuTaskContext, reason string) error {
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息未初始化")
	}

	// 暂停店铺到当日结束（23:59:59）
	h.memoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		task.TenantID,
		task.StoreID,
		reason,
	)

	h.logger.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s", task.TenantID, task.StoreID, reason)

	return nil
}

// logSubmitResponse 记录提交响应数据到日志
func (h *SavePublishResultHandler) logSubmitResponse(temuCtx *temucontext.TemuTaskContext, submitResponse interface{}) error {
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息未初始化")
	}

	// 序列化响应数据
	responseJSON, err := h.marshalWithoutHTMLEscape(submitResponse)
	if err != nil {
		h.logger.Errorf("序列化响应数据失败: %v", err)
		return fmt.Errorf("序列化响应数据失败: %w", err)
	}

	// 记录到结构化日志
	h.logger.WithFields(logrus.Fields{
		"task_id":    task.ID,
		"tenant_id":  task.TenantID,
		"store_id":   task.StoreID,
		"platform":   task.Platform,
		"product_id": task.ProductID,
		"response":   string(responseJSON),
	}).Info("TEMU产品提交响应数据")

	// // 保存响应数据到文件（用于调试和审计）
	// if err := h.saveResponseToFile(task.ID, responseJSON); err != nil {
	// 	h.logger.Warnf("保存响应数据到文件失败: %v", err)
	// 	// 不返回错误，因为这不是关键功能
	// }

	// 提取关键信息进行详细记录
	//h.logResponseDetails(submitResponse, task)

	return nil
}

// logResponseDetails 记录响应详细信息
func (h *SavePublishResultHandler) logResponseDetails(submitResponse interface{}, task *commontypes.Task) {
	// 尝试解析为ProductSubmitResponse结构
	if responseMap, ok := submitResponse.(map[string]interface{}); ok {
		// 记录基本响应信息
		success, _ := responseMap["success"].(bool)
		errorCode, _ := responseMap["error_code"].(float64)
		message, _ := responseMap["error_msg"].(string)

		h.logger.WithFields(logrus.Fields{
			"task_id":    task.ID,
			"success":    success,
			"error_code": int(errorCode),
			"message":    message,
		}).Info("TEMU提交响应基本信息")

		// 记录结果详情
		if result, ok := responseMap["result"].(map[string]interface{}); ok {
			submitSuccess, _ := result["submit_success"].(bool)
			editAlert, _ := result["edit_customized_info_alert"].(bool)

			h.logger.WithFields(logrus.Fields{
				"task_id":                    task.ID,
				"submit_success":             submitSuccess,
				"edit_customized_info_alert": editAlert,
			}).Info("TEMU提交结果详情")
		}
	}
}

// saveResponseToFile 保存响应数据到文件
func (h *SavePublishResultHandler) saveResponseToFile(taskID int64, responseData []byte) error {
	// 创建文件名
	filename := fmt.Sprintf("submit_response_%d_%s.json", taskID, time.Now().Format("20060102_150405"))

	// 确保目录存在
	logDir := "logs/responses"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建响应日志目录失败: %w", err)
	}

	// 写入文件
	filePath := filepath.Join(logDir, filename)
	if err := os.WriteFile(filePath, responseData, 0644); err != nil {
		return fmt.Errorf("写入响应文件失败: %w", err)
	}

	h.logger.Infof("响应数据已保存到文件: %s", filePath)
	return nil
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *SavePublishResultHandler) marshalWithoutHTMLEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // 关闭HTML转义

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// 移除最后的换行符
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}

// buildFilterRuleRange 构建筛选规则范围字符串
func (h *SavePublishResultHandler) buildFilterRuleRange(filterRule *api.FilterRuleRespDTO) string {
	if filterRule == nil {
		return ""
	}

	var rangeParts []string

	// 价格范围
	if filterRule.PriceMin != nil || filterRule.PriceMax != nil {
		var priceRange string
		if filterRule.PriceMin != nil && filterRule.PriceMax != nil {
			priceRange = fmt.Sprintf("价格:%.2f-%.2f", *filterRule.PriceMin, *filterRule.PriceMax)
		} else if filterRule.PriceMin != nil {
			priceRange = fmt.Sprintf("价格:>=%.2f", *filterRule.PriceMin)
		} else if filterRule.PriceMax != nil {
			priceRange = fmt.Sprintf("价格:<=%.2f", *filterRule.PriceMax)
		}
		if priceRange != "" {
			rangeParts = append(rangeParts, priceRange)
		}
	}

	// 库存范围
	if filterRule.StockMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("库存:>=%d", *filterRule.StockMin))
	}

	// 评分范围
	if filterRule.RatingMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("评分:>=%.1f", *filterRule.RatingMin))
	}

	// 评论数量范围
	if filterRule.ReviewCountMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("评论数:>=%d", *filterRule.ReviewCountMin))
	}

	// 发货时效
	if filterRule.DeliveryTimeMax != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("发货时效:<=%d天", *filterRule.DeliveryTimeMax))
	}

	// 配送方式
	if filterRule.FulfillmentType != "" && filterRule.FulfillmentType != "ALL" {
		rangeParts = append(rangeParts, fmt.Sprintf("配送:%s", filterRule.FulfillmentType))
	}

	if len(rangeParts) == 0 {
		return ""
	}

	return fmt.Sprintf("[%s]", strings.Join(rangeParts, ","))
}
