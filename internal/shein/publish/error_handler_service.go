// Package publish 提供SHEIN平台产品发布错误处理功能
package publish

import (
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/shein"
	product "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/content"
	skuutils "task-processor/internal/shein/product/sku"

	"github.com/sirupsen/logrus"
)

// PublishProductErrorHandler 产品发布错误处理器
type PublishProductErrorHandler struct {
}

// NewPublishProductErrorHandler 创建新的产品发布错误处理器
func NewPublishProductErrorHandler() *PublishProductErrorHandler {
	return &PublishProductErrorHandler{}
}

// HandlePublishResponse 处理发布响应
func (h *PublishProductErrorHandler) HandlePublishResponse(ctx *shein.TaskContext, response *product.SheinResponse) error {
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

				// 检查是否为数量类型错误，尝试自动修复
				if h.isQuantityTypeError(validResults) {
					logrus.Warnf("检测到数量类型错误，尝试自动修复并重新提交")
					if h.autoFixQuantityTypeAndResubmit(ctx, validResults) {
						logrus.Info("数量类型错误自动修复成功")
						return nil
					}
					// 如果自动修复失败，继续后续处理
				}

				// 检查是否为SKU重复错误
				if h.isDuplicateSKUError(validResults) {
					logrus.Errorf("检测到卖家SKU重复错误，标记为不可重试")
					// SKU重复错误不可重试
					return shein.NewNonRetryableError("产品发布失败: 卖家SKU重复", fmt.Errorf("%+v", response.Info.PreValidResult))
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

					// 保存到草稿箱
					handler := &PublishProductHandler{}
					handler.SaveDraftProduct(ctx)

					// 产品已保存到草稿箱，不需要再重试
					return shein.NewNonRetryableError("产品已保存到草稿箱，请手动处理", fmt.Errorf("%+v", response.Info.PreValidResult))
				}
			}
		}

		// 保存发布成功后的所有对应记录
		saver := NewPublishProductSaver()
		if err := saver.SavePublishResult(ctx, response); err != nil {
			// 保存结果失败可能是数据问题，不可重试
			return shein.NewNonRetryableError("保存发布结果失败", err)
		}
	} else {
		// 发布失败，根据错误信息判断是否可重试
		if response != nil {
			// 如果是数据验证错误等，不可重试
			if response.Code == "400" || response.Code == "403" {
				return shein.NewNonRetryableError("产品发布失败: "+response.Msg, nil)
			}
		}
		// 其他错误可重试
		return shein.NewRetryableError("产品发布失败", fmt.Errorf("%+v", response))
	}

	return nil
}

// autoReplaceSensitiveWordsAndResubmit 自动替换敏感词并重新提交
func (h *PublishProductErrorHandler) autoReplaceSensitiveWordsAndResubmit(ctx *shein.TaskContext, results []shein.PreValidResult) bool {
	logrus.Info("开始检查敏感词错误并尝试自动替换重试...")

	// 创建敏感词服务实例
	sensitiveWordService := h.getSensitiveWordService()
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
	handler := &PublishProductHandler{}
	response, err := handler.publishProduct(ctx)
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
	saver := NewPublishProductSaver()
	if err := saver.SavePublishResult(ctx, response); err != nil {
		logrus.Errorf("敏感词重试成功但保存结果失败: %v", err)
		return false
	}

	logrus.Info("敏感词重试成功 - 产品发布成功")
	return true
}

// getSensitiveWordService 获取敏感词服务实例
func (h *PublishProductErrorHandler) getSensitiveWordService() *content.SensitiveWordService {
	// 创建简化的敏感词服务实例
	service := content.NewSensitiveWordService()

	logrus.Info("创建简化敏感词服务实例")

	return service
}

// parsePreValidResult 解析预验证结果
func (h *PublishProductErrorHandler) parsePreValidResult(preValidResult any) ([]shein.PreValidResult, error) {
	if preValidResult == nil {
		return []shein.PreValidResult{}, nil
	}

	// 添加调试代码：打印实际的响应数据结构
	logrus.Infof("🔍 调试 - PreValidResult 原始数据类型: %T", preValidResult)

	// 将interface{}转换为JSON字符串，再解析为结构体
	jsonData, err := json.Marshal(preValidResult)
	if err != nil {
		logrus.Errorf("❌ 调试 - 序列化 PreValidResult 失败: %v", err)
		return nil, err
	}

	// 打印实际的JSON结构
	logrus.Infof("🔍 调试 - PreValidResult JSON 数据: %s", string(jsonData))

	var results []shein.PreValidResult
	if err := jsonx.UnmarshalBytes(jsonData, &results, "反序列化 PreValidResult 失败"); err != nil {
		logrus.Errorf("❌ 调试 - 反序列化 PreValidResult 失败: %v", err)
		logrus.Errorf("❌ 调试 - 尝试反序列化的JSON: %s", string(jsonData))
		return nil, err
	}

	logrus.Infof("✅ 调试 - 成功解析 PreValidResult，共 %d 项", len(results))
	return results, nil
}

// hasValidationError 检查是否存在验证错误
func (h *PublishProductErrorHandler) hasValidationError(results []shein.PreValidResult) bool {
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
func (h *PublishProductErrorHandler) formatValidationErrors(results []shein.PreValidResult) string {
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

// isSpecificationError 检查是否为规格配置错误
func (h *PublishProductErrorHandler) isSpecificationError(results []shein.PreValidResult) bool {
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
func (h *PublishProductErrorHandler) isDuplicateSKUError(results []shein.PreValidResult) bool {
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

// isQuantityTypeError 检查是否为数量类型错误
func (h *PublishProductErrorHandler) isQuantityTypeError(results []shein.PreValidResult) bool {
	quantityErrorPatterns := []string{
		"SKU件数类型为同款多件时，有效范围2-99999",
		"数量必须大于等于2",
		"quantityType",
		"quantity",
	}

	for _, result := range results {
		// 检查错误消息中是否包含数量相关关键词
		for _, message := range result.Messages {
			for _, pattern := range quantityErrorPatterns {
				if strings.Contains(message, pattern) {
					logrus.Infof("检测到数量类型错误: %s", message)
					return true
				}
			}
		}

		// 检查SKC错误消息中的数量错误
		for _, skcError := range result.SkcErrorMessageMap {
			for _, message := range skcError.Messages {
				for _, pattern := range quantityErrorPatterns {
					if strings.Contains(message, pattern) {
						logrus.Infof("检测到SKC数量类型错误: %s", message)
						return true
					}
				}
			}
		}
	}

	return false
}

// autoFixQuantityTypeAndResubmit 自动修复数量类型错误并重新提交
func (h *PublishProductErrorHandler) autoFixQuantityTypeAndResubmit(ctx *shein.TaskContext, _ []shein.PreValidResult) bool {
	logrus.Info("开始自动修复数量类型错误...")

	if ctx.ProductData == nil || len(ctx.ProductData.SKCList) == 0 {
		logrus.Error("产品数据为空，无法修复数量类型错误")
		return false
	}

	fixed := false

	// 遍历所有SKC和SKU，修复数量类型问题
	for skcIndex, skc := range ctx.ProductData.SKCList {
		for skuIndex, sku := range skc.SKUS {
			if sku.QuantityInfo != nil {
				originalQuantityType := 1
				originalQuantity := 1

				if sku.QuantityInfo.QuantityType != nil {
					originalQuantityType = *sku.QuantityInfo.QuantityType
				}
				if sku.QuantityInfo.Quantity != nil {
					originalQuantity = *sku.QuantityInfo.Quantity
				}

				// 使用SKUUtils的修正逻辑
				skuUtilsInstance := skuutils.NewSKUUtils()
				correctedQuantityType, correctedQuantity := skuUtilsInstance.CorrectQuantityTypeAndValue(
					originalQuantityType, originalQuantity, sku.SupplierSKU)

				// 如果有修正，应用修正
				if correctedQuantityType != originalQuantityType || correctedQuantity != originalQuantity {
					sku.QuantityInfo.QuantityType = &correctedQuantityType
					sku.QuantityInfo.Quantity = &correctedQuantity

					logrus.Infof("修复SKC[%d] SKU[%d] %s: quantityType %d->%d, quantity %d->%d",
						skcIndex, skuIndex, sku.SupplierSKU,
						originalQuantityType, correctedQuantityType,
						originalQuantity, correctedQuantity)
					fixed = true
				}
			}
		}
	}

	if !fixed {
		logrus.Warn("未发现需要修复的数量类型问题")
		return false
	}

	// 重新提交产品
	logrus.Info("开始执行数量类型修复后的产品重新提交...")
	handler := &PublishProductHandler{}
	response, err := handler.publishProduct(ctx)
	if err != nil {
		logrus.Errorf("数量类型修复重试失败 - 重新提交产品时发生错误: %v", err)
		return false
	}

	logrus.Info("数量类型修复重试 - 产品重新提交完成，正在检查结果...")

	// 检查重新提交的结果
	if response == nil || response.Code != "0" {
		logrus.Warnf("数量类型修复重试失败 - 产品发布失败，响应码: %s", response.Code)
		return false
	}

	// 检查是否还有验证错误
	validResults, parseErr := h.parsePreValidResult(response.Info.PreValidResult)
	if parseErr != nil {
		logrus.Warnf("解析重新提交的验证结果失败: %v", parseErr)
		return false
	}

	// 如果还有验证错误，说明修复没有完全解决问题
	if h.hasValidationError(validResults) {
		logrus.Warnf("数量类型修复重试后仍有验证错误，修复未完全解决问题")
		return false
	}

	// 保存发布成功后的结果
	saver := NewPublishProductSaver()
	if err := saver.SavePublishResult(ctx, response); err != nil {
		logrus.Errorf("数量类型修复重试成功但保存结果失败: %v", err)
		return false
	}

	logrus.Info("数量类型修复重试成功 - 产品发布成功")
	return true
}
