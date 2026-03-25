// Package publish 提供SHEIN平台产品发布验证功能
package publish

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"task-processor/internal/core/logger"
	shein "task-processor/internal/shein"
	apiproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/validation"

	"github.com/sirupsen/logrus"
)

// PublishProductValidator 产品发布验证器
type PublishProductValidator struct {
	logger *logrus.Entry
}

// NewPublishProductValidator 创建新的产品发布验证器
func NewPublishProductValidator() *PublishProductValidator {
	return &PublishProductValidator{
		logger: logger.GetGlobalLogger("publish_validator"),
	}
}

// PreValidateProductData 发布前预验证产品数据
func (v *PublishProductValidator) PreValidateProductData(ctx *shein.TaskContext, input *ValidationInput) error {
	v.logger.Info("🔍 开始产品数据预验证...")

	if input == nil || input.ProductData == nil {
		return fmt.Errorf("产品数据为空")
	}

	report := v.generateValidationReport(ctx, input)

	if len(report.CriticalIssues) > 0 {
		v.trySaveReport(ctx, report, "validation_failed_report", "保存验证失败报告失败")
		return fmt.Errorf("发现%d个严重问题，无法继续发布", len(report.CriticalIssues))
	}

	if report.AutoFixedIssues > 0 {
		v.logger.Infof("🔧 自动修复了%d个问题，产品数据已优化", report.AutoFixedIssues)
	}

	successRate := float64(report.PassedChecks) / float64(report.TotalChecks) * 100
	if successRate < 75 {
		v.trySaveReport(ctx, report, "validation_low_success_report", "保存低成功率验证报告失败")
		return fmt.Errorf("验证成功率过低(%.1f%%)，建议检查产品数据", successRate)
	}

	v.logger.Info("✅ 产品数据预验证全部通过")
	return nil
}

// validateBasicProductInfo 验证基本产品信息
func (v *PublishProductValidator) validateBasicProductInfo(input *ValidationInput) error {
	product := input.ProductData

	if len(product.MultiLanguageNameList) == 0 {
		return fmt.Errorf("缺少产品名称")
	}
	if len(product.MultiLanguageDescList) == 0 {
		return fmt.Errorf("缺少产品描述")
	}
	if product.CategoryID == 0 {
		return fmt.Errorf("缺少分类ID")
	}

	v.logger.Debug("✅ 基本产品信息验证通过")
	return nil
}

// validateSKCAndSKUData 验证SKC和SKU数据完整性
func (v *PublishProductValidator) validateSKCAndSKUData(input *ValidationInput) error {
	product := input.ProductData

	if len(product.SKCList) == 0 {
		return fmt.Errorf("缺少SKC数据")
	}

	totalSKUs := 0
	issues := []string{}

	for skcIndex, skc := range product.SKCList {
		if len(skc.SKUS) == 0 {
			issues = append(issues, fmt.Sprintf("SKC[%d]缺少SKU数据", skcIndex))
			continue
		}

		for skuIndex, sku := range skc.SKUS {
			totalSKUs++

			if sku.SupplierSKU == "" {
				issues = append(issues, fmt.Sprintf("SKC[%d] SKU[%d]缺少SupplierSKU", skcIndex, skuIndex))
			}
			if sku.CostInfo == nil || sku.CostInfo.CostPrice == "" {
				issues = append(issues, fmt.Sprintf("SKC[%d] SKU[%d]缺少成本价格信息", skcIndex, skuIndex))
			}
			if len(sku.PriceInfoList) == 0 {
				issues = append(issues, fmt.Sprintf("SKC[%d] SKU[%d]缺少价格信息", skcIndex, skuIndex))
			}
			if len(sku.StockInfoList) == 0 {
				issues = append(issues, fmt.Sprintf("SKC[%d] SKU[%d]缺少库存信息", skcIndex, skuIndex))
			}
			if issue := v.validateQuantityTypeAndValue(sku, skcIndex, skuIndex); issue != "" {
				issues = append(issues, issue)
			}
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("发现%d个SKC/SKU数据问题: %s", len(issues), strings.Join(issues, "; "))
	}

	v.logger.Debugf("✅ SKC和SKU数据验证通过，共%d个SKC，%d个SKU", len(product.SKCList), totalSKUs)
	return nil
}

// validateQuantityTypeAndValue 验证数量类型和数量值的匹配性
func (v *PublishProductValidator) validateQuantityTypeAndValue(sku apiproduct.SKU, skcIndex, skuIndex int) string {
	if sku.QuantityInfo == nil || sku.QuantityInfo.QuantityType == nil || sku.QuantityInfo.Quantity == nil {
		return ""
	}
	quantityType := *sku.QuantityInfo.QuantityType
	quantity := *sku.QuantityInfo.Quantity

	validator := validation.NewQuantityValidator()
	if err := validator.ValidateQuantity(quantity, quantityType); err != nil {
		return fmt.Sprintf("SKC[%d] SKU[%d]数量配置错误: %v (quantityType=%d, quantity=%d)",
			skcIndex, skuIndex, err, quantityType, quantity)
	}
	return ""
}

// ValidationReport 验证报告
type ValidationReport struct {
	TotalChecks     int                 `json:"total_checks"`
	PassedChecks    int                 `json:"passed_checks"`
	FailedChecks    int                 `json:"failed_checks"`
	AutoFixedIssues int                 `json:"auto_fixed_issues"`
	CriticalIssues  []string            `json:"critical_issues"`
	WarningIssues   []string            `json:"warning_issues"`
	FixedIssues     []string            `json:"fixed_issues"`
	DetailedIssues  map[string][]string `json:"detailed_issues"`
	ValidationTime  int64               `json:"validation_time_ms"`
}

// generateValidationReport 生成验证报告
func (v *PublishProductValidator) generateValidationReport(ctx *shein.TaskContext, input *ValidationInput) *ValidationReport {
	startTime := time.Now()

	report := &ValidationReport{
		TotalChecks:    4,
		CriticalIssues: []string{},
		WarningIssues:  []string{},
		FixedIssues:    []string{},
		DetailedIssues: make(map[string][]string),
	}

	// 1. 验证基本产品信息
	if err := v.validateBasicProductInfo(input); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("基本信息: %v", err))
	} else {
		report.PassedChecks++
	}

	// 2. 验证多件商品SKU图片（带自动修复）
	beforeSKUValidation := len(report.FixedIssues)
	if err := v.validateMultiPieceSKUImagesWithReport(ctx, report); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("多件商品SKU图片: %v", err))
	} else {
		report.PassedChecks++
	}
	report.AutoFixedIssues += len(report.FixedIssues) - beforeSKUValidation

	// 3. 验证SKC和SKU数据完整性
	if err := v.validateSKCAndSKUData(input); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("SKC/SKU数据: %v", err))
	} else {
		report.PassedChecks++
	}

	report.ValidationTime = time.Since(startTime).Milliseconds()
	return report
}

// validateMultiPieceSKUImagesWithReport 带报告的多件商品SKU图片验证
func (v *PublishProductValidator) validateMultiPieceSKUImagesWithReport(ctx *shein.TaskContext, report *ValidationReport) error {
	product := ctx.ProductData

	if len(product.SKCList) == 0 {
		report.WarningIssues = append(report.WarningIssues, "没有SKC数据")
		return nil
	}

	multiPieceIssues := []string{}
	fixedCount := 0

	for skcIndex, skc := range product.SKCList {
		for skuIndex, sku := range skc.SKUS {
			isMultiPiece := sku.QuantityInfo != nil &&
				sku.QuantityInfo.QuantityType != nil &&
				*sku.QuantityInfo.QuantityType == 2

			if !isMultiPiece {
				continue
			}

			if sku.ImageInfo == nil || len(sku.ImageInfo.ImageInfoList) == 0 {
				multiPieceIssues = append(multiPieceIssues, fmt.Sprintf(
					"多件商品SKU缺少图片 (SKC[%d] SKU[%d] SupplierSKU: %s) - 应该在构建阶段已修复",
					skcIndex, skuIndex, sku.SupplierSKU))
			} else {
				fixedCount += v.fixSKUImageSorting(&skc.SKUS[skuIndex], sku.SupplierSKU, report)
			}
		}
	}

	if len(multiPieceIssues) > 0 {
		report.DetailedIssues["多件商品SKU图片问题"] = multiPieceIssues
		return fmt.Errorf("发现%d个多件商品SKU图片问题: %s",
			len(multiPieceIssues), strings.Join(multiPieceIssues, "; "))
	}

	if fixedCount > 0 {
		report.WarningIssues = append(report.WarningIssues, fmt.Sprintf("验证阶段修复了%d个SKU图片排序问题", fixedCount))
	}

	return nil
}

// fixSKUImageSorting 修复多件商品 SKU 图片数量和排序，返回修复次数。
func (v *PublishProductValidator) fixSKUImageSorting(sku *apiproduct.SKU, supplierSKU string, report *ValidationReport) int {
	if sku.ImageInfo == nil || len(sku.ImageInfo.ImageInfoList) == 0 {
		return 0
	}

	fixedCount := 0
	images := sku.ImageInfo.ImageInfoList

	// 多件商品 SKU 只能有一张图片
	if len(images) > 1 {
		report.FixedIssues = append(report.FixedIssues,
			fmt.Sprintf("修复多件商品SKU图片数量: SKU %s 从%d张减少到1张", supplierSKU, len(images)))
		sku.ImageInfo.ImageInfoList = images[:1]
		images = sku.ImageInfo.ImageInfoList
		fixedCount++
	}

	// 修复第一张图片的排序
	if len(images) > 0 && images[0].ImageSort != 1 {
		report.FixedIssues = append(report.FixedIssues,
			fmt.Sprintf("修复多件商品SKU主图排序: SKU %s 从%d修复为1", supplierSKU, images[0].ImageSort))
		sku.ImageInfo.ImageInfoList[0].ImageSort = 1
		fixedCount++
	}

	return fixedCount
}

// trySaveReport 尝试保存验证报告，失败时只记录日志
func (v *PublishProductValidator) trySaveReport(ctx *shein.TaskContext, report *ValidationReport, suffix, errMsg string) {
	if ctx.Task == nil {
		return
	}
	taskID := fmt.Sprintf("%d", ctx.Task.ID)
	filename := fmt.Sprintf("%s_%s_%s.json", ctx.Task.ProductID, taskID, suffix)
	if err := v.saveValidationReportToFile(filename, report); err != nil {
		v.logger.Errorf("%s: %v", errMsg, err)
	} else {
		v.logger.Infof("📊 验证报告已保存: %s", filename)
	}
}

// saveValidationReportToFile 保存验证报告到JSON文件
func (v *PublishProductValidator) saveValidationReportToFile(filename string, report *ValidationReport) error {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化验证报告失败: %w", err)
	}

	filePath := filepath.Join("logs", filename)
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入验证报告文件失败: %w", err)
	}

	return nil
}
