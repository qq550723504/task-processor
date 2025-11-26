package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/common"
	management_api "task-processor/common/management/api"
	product "task-processor/common/shein/api/product"

	"github.com/sirupsen/logrus"
)

// PublishProductHandler 发布产品处理器
type PublishProductHandler struct {
}

// NewPublishProductHandler 创建新的发布产品处理器
func NewPublishProductHandler() *PublishProductHandler {
	return &PublishProductHandler{}
}

// Name 返回处理器名称
func (h *PublishProductHandler) Name() string {
	return "发布产品"
}

// Handle 执行发布产品处理
func (h *PublishProductHandler) Handle(ctx *TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	// 检查是否已获取店铺客户端
	if ctx.ShopClient == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("店铺客户端未获取，请先执行获取店铺API客户端步骤", nil)
	}

	// 检查产品是否已上架
	if err := h.checkProductExists(ctx); err != nil {
		logrus.Errorf("❌ 检查产品是否已上架失败: %v", err)
		// 检查失败可能是网络问题，可重试
		return NewRetryableError("检查产品是否已上架失败", err)
	}

	// 方案3：发布前预验证
	logrus.Info("🔍 开始发布前预验证...")
	if err := h.preValidateProductData(ctx); err != nil {
		logrus.Errorf("❌ 发布前预验证失败: %v", err)
		// 预验证失败通常是数据问题，可重试（可能通过重新处理解决）
		return NewRetryableError("发布前预验证失败", err)
	}
	logrus.Info("✅ 发布前预验证通过")

	// 发布产品
	response, err := h.publishProduct(ctx)
	if err != nil {
		// 发布失败可能是网络问题或临时性错误，可重试
		return NewRetryableError("发布产品失败", err)
	}

	return h.handlePublishResponse(ctx, response)
}

// checkProductExists 检查产品是否已上架
func (h *PublishProductHandler) checkProductExists(ctx *TaskContext) error {
	// 检查必要的上下文信息
	if ctx.ManagementClientMgr == nil {
		logrus.Warn("管理客户端管理器未初始化，跳过产品存在性检查")
		return nil
	}

	if ctx.Task == nil {
		logrus.Warn("任务信息未初始化，跳过产品存在性检查")
		return nil
	}

	// 获取产品导入映射客户端
	mappingClient := ctx.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		logrus.Warn("产品导入映射客户端未初始化，跳过产品存在性检查")
		return nil
	}

	// 检查主产品是否已上架
	if ctx.Task.ProductID != "" {
		req := &management_api.ProductImportMappingCheckReqDTO{
			StoreId:   ctx.Task.StoreID,
			Platform:  ctx.Task.Platform,
			Region:    ctx.Task.Region,
			ProductId: ctx.Task.ProductID,
		}

		exists, err := mappingClient.CheckProductExists(req)
		if err != nil {
			logrus.Errorf("检查产品 %s 是否已上架失败: %v", ctx.Task.ProductID, err)
			return err
		}

		if exists {
			logrus.Warnf("⚠️ 产品 %s 已经上架过，跳过本次上架", ctx.Task.ProductID)
			return NewNonRetryableError(fmt.Sprintf("产品 %s 已经上架过", ctx.Task.ProductID), nil)
		}

		logrus.Infof("✅ 产品 %s 未上架，可以继续上架流程", ctx.Task.ProductID)
	}

	// 检查所有变体是否已上架
	if ctx.Variants != nil && len(*ctx.Variants) > 0 {
		for _, variant := range *ctx.Variants {
			if variant.Asin == "" {
				continue
			}

			req := &management_api.ProductImportMappingCheckReqDTO{
				StoreId:   ctx.Task.StoreID,
				Platform:  ctx.Task.Platform,
				Region:    ctx.Task.Region,
				ProductId: variant.Asin,
			}

			exists, err := mappingClient.CheckProductExists(req)
			if err != nil {
				logrus.Errorf("检查变体 %s 是否已上架失败: %v", variant.Asin, err)
				// 单个变体检查失败不影响整体流程，继续检查下一个
				continue
			}

			if exists {
				logrus.Warnf("⚠️ 变体 %s 已经上架过", variant.Asin)
				// 标记该变体已被筛选掉
				ctx.SetVariantFiltered(variant.Asin, true, fmt.Sprintf("产品 %s 已经上架过", variant.Asin))
			} else {
				logrus.Debugf("✅ 变体 %s 未上架", variant.Asin)
			}
		}
	}

	return nil
}

// preValidateProductData 发布前预验证产品数据
func (h *PublishProductHandler) preValidateProductData(ctx *TaskContext) error {
	logrus.Info("🔍 开始产品数据预验证...")

	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据为空")
	}

	// 生成详细的验证报告
	report := h.generateValidationReport(ctx)

	// 检查是否有严重问题
	if len(report.CriticalIssues) > 0 {
		return fmt.Errorf("发现%d个严重问题，无法继续发布", len(report.CriticalIssues))
	}

	// 如果有自动修复，记录修复信息
	if report.AutoFixedIssues > 0 {
		logrus.Infof("🔧 自动修复了%d个问题，产品数据已优化", report.AutoFixedIssues)
	}

	// 计算验证成功率
	successRate := float64(report.PassedChecks) / float64(report.TotalChecks) * 100
	if successRate < 75 {
		return fmt.Errorf("验证成功率过低(%.1f%%)，建议检查产品数据", successRate)
	}

	logrus.Info("✅ 产品数据预验证全部通过")
	return nil
}

// validateBasicProductInfo 验证基本产品信息
func (h *PublishProductHandler) validateBasicProductInfo(ctx *TaskContext) error {
	product := ctx.ProductData

	// 检查必要字段
	if len(product.MultiLanguageNameList) == 0 {
		return fmt.Errorf("缺少产品名称")
	}

	if len(product.MultiLanguageDescList) == 0 {
		return fmt.Errorf("缺少产品描述")
	}

	if product.CategoryID == 0 {
		return fmt.Errorf("缺少分类ID")
	}

	logrus.Debug("✅ 基本产品信息验证通过")
	return nil
}

// validateSKCAndSKUData 验证SKC和SKU数据完整性
func (h *PublishProductHandler) validateSKCAndSKUData(ctx *TaskContext) error {
	product := ctx.ProductData

	if len(product.SKCList) == 0 {
		return fmt.Errorf("缺少SKC数据")
	}

	totalSKUs := 0
	issues := []string{}

	for skcIndex, skc := range product.SKCList {
		if len(skc.SKUS) == 0 {
			issue := fmt.Sprintf("SKC[%d]缺少SKU数据", skcIndex)
			issues = append(issues, issue)
			continue
		}

		for skuIndex, sku := range skc.SKUS {
			totalSKUs++

			// 检查必要字段
			if sku.SupplierSKU == "" {
				issue := fmt.Sprintf("SKC[%d] SKU[%d]缺少SupplierSKU", skcIndex, skuIndex)
				issues = append(issues, issue)
			}

			if sku.CostInfo == nil || sku.CostInfo.CostPrice == "" {
				issue := fmt.Sprintf("SKC[%d] SKU[%d]缺少成本价格信息", skcIndex, skuIndex)
				issues = append(issues, issue)
			}

			if len(sku.PriceInfoList) == 0 {
				issue := fmt.Sprintf("SKC[%d] SKU[%d]缺少价格信息", skcIndex, skuIndex)
				issues = append(issues, issue)
			}

			if len(sku.StockInfoList) == 0 {
				issue := fmt.Sprintf("SKC[%d] SKU[%d]缺少库存信息", skcIndex, skuIndex)
				issues = append(issues, issue)
			}
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("发现%d个SKC/SKU数据问题: %s",
			len(issues), strings.Join(issues, "; "))
	}

	logrus.Debugf("✅ SKC和SKU数据验证通过，共%d个SKC，%d个SKU", len(product.SKCList), totalSKUs)
	return nil
}

// ValidationReport 验证报告
type ValidationReport struct {
	TotalChecks     int      `json:"total_checks"`
	PassedChecks    int      `json:"passed_checks"`
	FailedChecks    int      `json:"failed_checks"`
	AutoFixedIssues int      `json:"auto_fixed_issues"`
	CriticalIssues  []string `json:"critical_issues"`
	WarningIssues   []string `json:"warning_issues"`
	FixedIssues     []string `json:"fixed_issues"`
	ValidationTime  int64    `json:"validation_time_ms"`
}

// generateValidationReport 生成验证报告
func (h *PublishProductHandler) generateValidationReport(ctx *TaskContext) *ValidationReport {
	startTime := time.Now()

	report := &ValidationReport{
		TotalChecks:     4, // 基本信息、主图、多件商品SKU、SKC/SKU数据
		PassedChecks:    0,
		FailedChecks:    0,
		AutoFixedIssues: 0,
		CriticalIssues:  []string{},
		WarningIssues:   []string{},
		FixedIssues:     []string{},
	}

	// 1. 验证基本产品信息
	if err := h.validateBasicProductInfo(ctx); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("基本信息: %v", err))
	} else {
		report.PassedChecks++
	}

	// 3. 验证多件商品SKU图片（带自动修复）
	beforeSKUValidation := len(report.FixedIssues)
	if err := h.validateMultiPieceSKUImagesWithReport(ctx, report); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("多件商品SKU图片: %v", err))
	} else {
		report.PassedChecks++
	}
	report.AutoFixedIssues += len(report.FixedIssues) - beforeSKUValidation

	// 4. 验证SKC和SKU数据完整性
	if err := h.validateSKCAndSKUData(ctx); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("SKC/SKU数据: %v", err))
	} else {
		report.PassedChecks++
	}

	report.ValidationTime = time.Since(startTime).Milliseconds()

	// 记录验证报告
	h.logValidationReport(report)

	return report
}

// validateMultiPieceSKUImagesWithReport 带报告的多件商品SKU图片验证
func (h *PublishProductHandler) validateMultiPieceSKUImagesWithReport(ctx *TaskContext, report *ValidationReport) error {
	product := ctx.ProductData

	if len(product.SKCList) == 0 {
		report.WarningIssues = append(report.WarningIssues, "没有SKC数据")
		return nil
	}

	multiPieceIssues := []string{}
	fixedCount := 0

	for skcIndex, skc := range product.SKCList {
		if len(skc.SKUS) == 0 {
			continue
		}

		for skuIndex, sku := range skc.SKUS {
			// 检查是否为多件商品
			isMultiPiece := sku.QuantityInfo != nil &&
				sku.QuantityInfo.QuantityType != nil &&
				*sku.QuantityInfo.QuantityType == 2

			if isMultiPiece {
				// 多件商品必须有SKU图片
				if sku.ImageInfo == nil || len(sku.ImageInfo.ImageInfoList) == 0 {
					issue := fmt.Sprintf("多件商品SKU缺少图片 (SKC[%d] SKU[%d] SupplierSKU: %s)",
						skcIndex, skuIndex, sku.SupplierSKU)
					multiPieceIssues = append(multiPieceIssues, issue)
				} else {
					// 多件商品SKU只能有一张图片
					if len(sku.ImageInfo.ImageInfoList) > 1 {
						fixMsg := fmt.Sprintf("修复多件商品SKU图片数量: SKU %s 从%d张减少到1张",
							sku.SupplierSKU, len(sku.ImageInfo.ImageInfoList))
						report.FixedIssues = append(report.FixedIssues, fixMsg)
						// 只保留第一张图片
						sku.ImageInfo.ImageInfoList = sku.ImageInfo.ImageInfoList[:1]
						fixedCount++
					}

					// 多件商品SKU的主图排序必须是1
					if len(sku.ImageInfo.ImageInfoList) > 0 {
						if sku.ImageInfo.ImageInfoList[0].ImageSort != 1 {
							fixMsg := fmt.Sprintf("修复多件商品SKU主图排序: SKU %s 从%d修复为1",
								sku.SupplierSKU, sku.ImageInfo.ImageInfoList[0].ImageSort)
							report.FixedIssues = append(report.FixedIssues, fixMsg)
							sku.ImageInfo.ImageInfoList[0].ImageSort = 1
							fixedCount++
						}
					}
				}
			}
		}
	}

	if len(multiPieceIssues) > 0 {
		return fmt.Errorf("发现%d个多件商品SKU图片问题", len(multiPieceIssues))
	}

	if fixedCount > 0 {
		report.WarningIssues = append(report.WarningIssues, fmt.Sprintf("自动修复了%d个SKU图片排序问题", fixedCount))
	}

	return nil
}

// logValidationReport 记录验证报告
func (h *PublishProductHandler) logValidationReport(report *ValidationReport) {
	logrus.Infof("📊 产品验证报告:")
	logrus.Infof("   总检查项: %d", report.TotalChecks)
	logrus.Infof("   通过检查: %d", report.PassedChecks)
	logrus.Infof("   失败检查: %d", report.FailedChecks)
	logrus.Infof("   自动修复: %d", report.AutoFixedIssues)
	logrus.Infof("   验证耗时: %dms", report.ValidationTime)

	if len(report.CriticalIssues) > 0 {
		logrus.Errorf("❌ 严重问题 (%d个):", len(report.CriticalIssues))
		for i, issue := range report.CriticalIssues {
			logrus.Errorf("   %d. %s", i+1, issue)
		}
	}

	if len(report.WarningIssues) > 0 {
		logrus.Warnf("⚠️ 警告问题 (%d个):", len(report.WarningIssues))
		for i, issue := range report.WarningIssues {
			logrus.Warnf("   %d. %s", i+1, issue)
		}
	}

	if len(report.FixedIssues) > 0 {
		logrus.Infof("🔧 自动修复 (%d个):", len(report.FixedIssues))
		for i, issue := range report.FixedIssues {
			logrus.Infof("   %d. %s", i+1, issue)
		}
	}

	// 计算成功率
	successRate := float64(report.PassedChecks) / float64(report.TotalChecks) * 100
	if successRate == 100 {
		logrus.Infof("🎉 验证成功率: %.1f%% - 完美通过!", successRate)
	} else if successRate >= 75 {
		logrus.Infof("✅ 验证成功率: %.1f%% - 良好", successRate)
	} else {
		logrus.Warnf("⚠️ 验证成功率: %.1f%% - 需要关注", successRate)
	}
}

// autoReplaceSensitiveWordsAndResubmit 自动替换敏感词并重新提交
func (h *PublishProductHandler) autoReplaceSensitiveWordsAndResubmit(ctx *TaskContext, results []PreValidResult) bool {
	logrus.Info("开始检查敏感词错误并尝试自动替换重试...")

	// 创建敏感词服务实例
	sensitiveWordService := h.getSensitiveWordService(ctx)
	if sensitiveWordService == nil {
		logrus.Error("无法创建敏感词服务，跳过敏感词处理")
		return false
	}

	// 使用敏感词服务处理验证错误
	if !sensitiveWordService.HandleValidationErrors(ctx, results) {
		return false
	}

	// 重新提交产品
	logrus.Info("开始执行敏感词替换后的产品重新提交...")
	response, err := h.publishProduct(ctx)
	if err != nil {
		logrus.Errorf("敏感词重试失败 - 重新提交产品时发生错误: %v", err)
		return false
	}

	logrus.Info("敏感词重试 - 产品重新提交完成，正在检查结果...")

	// 直接检查重新提交的结果，避免递归调用handlePublishResponse
	if response == nil || response.Code != "0" {
		logrus.Warnf("敏感词重试失败 - 产品发布失败，响应码: %s", response.Code)
		return false
	}

	// 检查是否还有验证错误
	validResults, parseErr := h.parsePreValidResult(response.Info.PreValidResult)
	if parseErr != nil {
		logrus.Warnf("解析重新提交的验证结果失败: %v", parseErr)
		return false
	}

	// 如果还有验证错误，说明敏感词替换没有完全解决问题
	if h.hasValidationError(validResults) {
		logrus.Warnf("敏感词重试后仍有验证错误，敏感词替换未完全解决问题")
		return false
	}

	// 保存发布成功后的结果
	if err := h.savePublishResult(ctx, response); err != nil {
		logrus.Errorf("敏感词重试成功但保存结果失败: %v", err)
		return false
	}

	logrus.Info("敏感词重试成功 - 产品发布成功")
	return true
}

// getSensitiveWordService 获取敏感词服务实例
func (h *PublishProductHandler) getSensitiveWordService(ctx *TaskContext) *SensitiveWordService {
	// 创建简化的敏感词服务实例
	service := NewSensitiveWordService()

	logrus.Info("创建简化敏感词服务实例")

	return service
}

// parsePreValidResult 解析预验证结果
func (h *PublishProductHandler) parsePreValidResult(preValidResult interface{}) ([]PreValidResult, error) {
	if preValidResult == nil {
		return []PreValidResult{}, nil
	}

	// 将interface{}转换为JSON字符串，再解析为结构体
	jsonData, err := json.Marshal(preValidResult)
	if err != nil {
		return nil, err
	}

	var results []PreValidResult
	if err := json.Unmarshal(jsonData, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// hasValidationError 检查是否存在验证错误
func (h *PublishProductHandler) hasValidationError(results []PreValidResult) bool {
	for _, result := range results {
		// 检查普通消息错误
		if len(result.Messages) > 0 {
			return true
		}

		// 检查多语言消息错误
		if len(result.OtherLanguageMessageMap) > 0 {
			for _, messages := range result.OtherLanguageMessageMap {
				if len(messages) > 0 {
					return true
				}
			}
		}

		// 检查SKC错误消息
		if len(result.SkcErrorMessageMap) > 0 {
			for _, skcError := range result.SkcErrorMessageMap {
				if len(skcError.Messages) > 0 {
					return true
				}
				// 检查SKC多语言消息错误
				if len(skcError.OtherLanguageMessageMap) > 0 {
					for _, messages := range skcError.OtherLanguageMessageMap {
						if len(messages) > 0 {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// formatValidationErrors 格式化验证错误信息
func (h *PublishProductHandler) formatValidationErrors(results []PreValidResult) string {
	var errorMsgs []string

	for _, result := range results {
		// 处理普通消息错误
		if len(result.Messages) > 0 {
			errorMsgs = append(errorMsgs, fmt.Sprintf("[%s - %s]: %s",
				result.Module, result.FormName, strings.Join(result.Messages, "; ")))
		}

		// 处理多语言消息错误
		if len(result.OtherLanguageMessageMap) > 0 {
			for lang, messages := range result.OtherLanguageMessageMap {
				if len(messages) > 0 {
					errorMsgs = append(errorMsgs, fmt.Sprintf("[%s - %s - %s]: %s",
						result.Module, result.FormName, lang, strings.Join(messages, "; ")))
				}
			}
		}

		// 处理SKC错误消息
		if len(result.SkcErrorMessageMap) > 0 {
			for skcIndex, skcError := range result.SkcErrorMessageMap {
				if len(skcError.Messages) > 0 {
					errorMsgs = append(errorMsgs, fmt.Sprintf("[%s - %s - SKC%s]: %s",
						result.Module, result.FormName, skcIndex, strings.Join(skcError.Messages, "; ")))
				}
				// 处理SKC多语言消息错误
				if len(skcError.OtherLanguageMessageMap) > 0 {
					for lang, messages := range skcError.OtherLanguageMessageMap {
						if len(messages) > 0 {
							errorMsgs = append(errorMsgs, fmt.Sprintf("[%s - %s - SKC%s - %s]: %s",
								result.Module, result.FormName, skcIndex, lang, strings.Join(messages, "; ")))
						}
					}
				}
			}
		}
	}

	return strings.Join(errorMsgs, "\n")
}

// handlePublishResponse 处理发布响应
func (h *PublishProductHandler) handlePublishResponse(ctx *TaskContext, response *product.SheinResponse) error {

	// 检查发布结果
	if response != nil && response.Code == "0" {
		// 解析验证结果
		validResults, parseErr := h.parsePreValidResult(response.Info.PreValidResult)
		if parseErr != nil {
			logrus.Warnf("解析验证结果失败: %v", parseErr)
		} else {
			// 检查是否有验证错误
			if h.hasValidationError(validResults) {
				// 检查是否为规格配置错误，需要提交任务限制
				if h.isSpecificationError(validResults) {
					logrus.Warnf("检测到规格配置错误，提交任务限制到管理系统")
					// 将规格配置错误信息记录到上下文中
					ctx.SpecificationErrors = validResults
					// 规格配置错误通常需要人工处理，但仍然继续执行后续处理器
					logrus.Info("规格配置错误已记录，继续执行后续处理器")
					return nil
				}

				// 检查是否为SKU重复错误
				if h.isDuplicateSKUError(validResults) {
					logrus.Errorf("检测到卖家SKU重复错误，标记为不可重试")
					// SKU重复错误不可重试
					return NewNonRetryableError("产品发布失败: 卖家SKU重复", fmt.Errorf("%+v", response.Info.PreValidResult))
				}

				// 尝试自动替换敏感词并重新提交
				if h.autoReplaceSensitiveWordsAndResubmit(ctx, validResults) {
					// 重新提交成功，继续处理
					logrus.Info("自动替换敏感词并重新提交成功")
					return nil
				} else {
					for _, validResult := range validResults {
						if len(validResult.Messages) > 0 {
							// 输出错误信息
							logrus.Warnf("验证失败: %s", strings.Join(validResult.Messages, "\n"))
						}
					}

					h.SaveDraftProduct(ctx)

					// 产品已保存到草稿箱，不需要再重试
					return NewNonRetryableError("产品已保存到草稿箱，请手动处理", fmt.Errorf("%+v", response.Info.PreValidResult))
				}
			}
		}

		// 保存发布成功后的所有对应记录
		if err := h.savePublishResult(ctx, response); err != nil {
			// 保存结果失败可能是数据问题，不可重试
			return NewNonRetryableError("保存发布结果失败", err)
		}
	} else {
		// 发布失败，根据错误信息判断是否可重试
		if response != nil {
			// 如果是数据验证错误等，不可重试
			if response.Code == "400" || response.Code == "403" {
				return NewNonRetryableError("产品发布失败: "+response.Msg, nil)
			}
		}
		// 其他错误可重试
		return NewRetryableError("产品发布失败", fmt.Errorf("%+v", response))
	}

	return nil
}

// savePublishResult 保存发布成功后的所有对应记录
func (h *PublishProductHandler) savePublishResult(ctx *TaskContext, response *product.SheinResponse) error {
	// 保存SPU名称
	if response.Info.SPUName != "" {
		ctx.ProductData.SPUName = response.Info.SPUName
	}

	// 保存版本信息
	// ...

	// 保存SKC和SKU的对应关系
	if ctx.SupplierSkuMap == nil {
		ctx.SupplierSkuMap = make(map[string]string)
	}

	// 遍历返回的SKC列表，建立ASIN和SKU的对应关系
	for _, skc := range response.Info.SKCList {
		// 遍历每个SKC中的SKU列表
		for _, sku := range skc.SKUList {
			// 保存对应关系到AsinSkuMap中
			ctx.SupplierSkuMap[sku.SKUCode] = sku.SupplierSKU
		}
	}

	return nil
}

// PreValidResult 预验证结果
type PreValidResult struct {
	Form                    string                     `json:"form"`
	FormName                string                     `json:"form_name"`
	Messages                []string                   `json:"messages"`
	Module                  string                     `json:"module"`
	OtherLanguageMessageMap map[string][]string        `json:"other_language_message_map"`
	SkcErrorMessageMap      map[string]SkcErrorMessage `json:"skc_error_message_map"`
}

// SkcErrorMessage SKC错误信息
type SkcErrorMessage struct {
	Messages                []string            `json:"messages"`
	OtherLanguageMessageMap map[string][]string `json:"otherLanguageMessageMap"`
}

// isSpecificationError 检查是否为规格配置错误
func (h *PublishProductHandler) isSpecificationError(results []PreValidResult) bool {
	specificationErrorPatterns := []string{
		"不可以作为主规格",
		"主规格",
	}

	for _, result := range results {

		// 检查错误消息中是否包含规格相关关键词
		for _, message := range result.Messages {
			for _, pattern := range specificationErrorPatterns {
				if strings.Contains(message, pattern) {
					logrus.Infof("检测到规格配置错误: %s", message)
					return true
				}
			}
		}

		// 检查SKC错误消息中的规格错误
		for _, skcError := range result.SkcErrorMessageMap {
			for _, message := range skcError.Messages {
				for _, pattern := range specificationErrorPatterns {
					if strings.Contains(message, pattern) {
						logrus.Infof("检测到SKC规格配置错误: %s", message)
						return true
					}
				}
			}
		}
	}

	return false
}

// isDuplicateSKUError 检查是否为SKU重复错误
func (h *PublishProductHandler) isDuplicateSKUError(results []PreValidResult) bool {
	for _, result := range results {
		// 检查错误消息中是否包含"卖家SKU重复"
		for _, message := range result.Messages {
			if strings.Contains(message, "卖家SKU重复") {
				logrus.Infof("检测到卖家SKU重复错误: %s", message)
				return true
			}
		}

		// 检查多语言消息错误中的SKU重复错误
		for _, messages := range result.OtherLanguageMessageMap {
			for _, message := range messages {
				if strings.Contains(message, "卖家SKU重复") {
					logrus.Infof("检测到多语言消息中的卖家SKU重复错误: %s", message)
					return true
				}
			}
		}

		// 检查SKC错误消息中的SKU重复错误
		for _, skcError := range result.SkcErrorMessageMap {
			for _, message := range skcError.Messages {
				if strings.Contains(message, "卖家SKU重复") {
					logrus.Infof("检测到SKC中的卖家SKU重复错误: %s", message)
					return true
				}
			}
			// 检查SKC多语言消息错误中的SKU重复错误
			for _, messages := range skcError.OtherLanguageMessageMap {
				for _, message := range messages {
					if strings.Contains(message, "卖家SKU重复") {
						logrus.Infof("检测到SKC多语言消息中的卖家SKU重复错误: %s", message)
						return true
					}
				}
			}
		}
	}

	return false
}

// publishProduct 统一的产品发布方法
func (h *PublishProductHandler) publishProduct(ctx *TaskContext) (*product.SheinResponse, error) {
	response, _, err := ctx.ShopClient.PublishProduct(ctx.ProductData)

	// 保存产品发布结果
	ctx.SheinResponse = response

	return response, err
}

func (h *PublishProductHandler) SaveDraftProduct(ctx *TaskContext) (*product.SheinResponse, error) {
	response, _, err := ctx.ShopClient.SaveDraftProduct(ctx.ProductData)
	if err != nil {
		return nil, err
	}

	// 保存到草稿箱成功后，更新任务状态为草稿箱
	h.updateTaskStatusToDraft(ctx)

	return response, nil
}

// updateTaskStatusToDraft 更新任务状态为草稿箱
func (h *PublishProductHandler) updateTaskStatusToDraft(ctx *TaskContext) {
	// 检查必要的上下文信息
	if ctx.ManagementClientMgr == nil {
		logrus.Warn("管理客户端管理器未初始化，跳过状态更新")
		return
	}

	if ctx.Task == nil {
		logrus.Warn("任务信息未初始化，跳过状态更新")
		return
	}

	// 获取导入任务客户端
	importTaskClient := ctx.ManagementClientMgr.GetImportTaskClient()
	if importTaskClient == nil {
		logrus.Warn("导入任务客户端未初始化，跳过状态更新")
		return
	}

	// 解析任务ID
	var taskID int64
	if _, err := fmt.Sscanf(ctx.Task.ID, "%d", &taskID); err != nil {
		logrus.Errorf("解析任务ID失败: %v", err)
		return
	}

	// 构建更新请求
	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:     taskID,
		Status: common.TaskStatusDraft.Int16(),
	}

	// 异步更新状态
	go func() {
		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			logrus.Errorf("更新任务状态为草稿箱失败 (TaskID: %s): %v", ctx.Task.ID, err)
		} else {
			logrus.Infof("✅ 任务状态已更新为草稿箱 (TaskID: %s)", ctx.Task.ID)
		}
	}()
}
