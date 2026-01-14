// Package modules 提供SHEIN平台产品发布验证功能
package publish

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	shein_model "task-processor/internal/platforms/shein/model"
	"task-processor/internal/platforms/shein/service/validation"
	"time"

	"github.com/sirupsen/logrus"
)

// PublishProductValidator 产品发布验证器
type PublishProductValidator struct {
}

// NewPublishProductValidator 创建新的产品发布验证器
func NewPublishProductValidator() *PublishProductValidator {
	return &PublishProductValidator{}
}

// PreValidateProductData 发布前预验证产品数据
func (v *PublishProductValidator) PreValidateProductData(ctx *shein_model.TaskContext) error {
	logrus.Info("🔍 开始产品数据预验证...")

	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据为空")
	}

	// 生成详细的验证报告
	report := v.generateValidationReport(ctx)

	// 检查是否有严重问题
	if len(report.CriticalIssues) > 0 {
		// 验证失败时保存验证报告到JSON文件
		if ctx.Task != nil {
			taskID := fmt.Sprintf("%d", ctx.Task.ID)
			filename := fmt.Sprintf("%s_%s_validation_failed_report.json", ctx.Task.ProductID, taskID)
			if err := v.saveValidationReportToFile(filename, report); err != nil {
				logrus.Errorf("保存验证失败报告失败: %v", err)
			} else {
				logrus.Infof("📊 验证失败报告已保存: %s", filename)
			}
		}

		return fmt.Errorf("发现%d个严重问题，无法继续发布", len(report.CriticalIssues))
	}

	// 如果有自动修复，记录修复信息
	if report.AutoFixedIssues > 0 {
		logrus.Infof("🔧 自动修复了%d个问题，产品数据已优化", report.AutoFixedIssues)
	}

	// 计算验证成功率
	successRate := float64(report.PassedChecks) / float64(report.TotalChecks) * 100
	if successRate < 75 {
		// 验证成功率过低时也保存验证报告
		if ctx.Task != nil {
			taskID := fmt.Sprintf("%d", ctx.Task.ID)
			filename := fmt.Sprintf("%s_%s_validation_low_success_report.json", ctx.Task.ProductID, taskID)
			if err := v.saveValidationReportToFile(filename, report); err != nil {
				logrus.Errorf("保存低成功率验证报告失败: %v", err)
			} else {
				logrus.Infof("📊 低成功率验证报告已保存: %s", filename)
			}
		}

		return fmt.Errorf("验证成功率过低(%.1f%%)，建议检查产品数据", successRate)
	}

	logrus.Info("✅ 产品数据预验证全部通过")
	return nil
}

// validateBasicProductInfo 验证基本产品信息
func (v *PublishProductValidator) validateBasicProductInfo(ctx *shein_model.TaskContext) error {
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
func (v *PublishProductValidator) validateSKCAndSKUData(ctx *shein_model.TaskContext) error {
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

			// 验证数量类型和数量值的匹配性
			if quantityIssue := v.validateQuantityTypeAndValue(sku, skcIndex, skuIndex); quantityIssue != "" {
				issues = append(issues, quantityIssue)
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

// validateQuantityTypeAndValue 验证数量类型和数量值的匹配性
func (v *PublishProductValidator) validateQuantityTypeAndValue(sku interface{}, skcIndex, skuIndex int) string {
	// 由于SKU可能是不同的类型，我们需要通过反射或类型断言来获取数量信息
	// 这里假设sku有QuantityInfo字段

	// 尝试获取数量信息（这里需要根据实际的SKU结构来调整）
	var quantityType, quantity *int

	// 如果是map类型（JSON反序列化后）
	if skuMap, ok := sku.(map[string]interface{}); ok {
		if quantityInfo, exists := skuMap["quantity_info"]; exists {
			if qiMap, ok := quantityInfo.(map[string]interface{}); ok {
				if qt, exists := qiMap["quantity_type"]; exists {
					if qtInt, ok := qt.(float64); ok {
						qtIntVal := int(qtInt)
						quantityType = &qtIntVal
					}
				}
				if q, exists := qiMap["quantity"]; exists {
					if qInt, ok := q.(float64); ok {
						qIntVal := int(qInt)
						quantity = &qIntVal
					}
				}
			}
		}
	}

	// 如果无法获取数量信息，跳过验证
	if quantityType == nil || quantity == nil {
		return ""
	}

	// 使用数量验证器进行验证
	validator := validation.NewQuantityValidator()
	if err := validator.ValidateQuantity(*quantity, *quantityType); err != nil {
		return fmt.Sprintf("SKC[%d] SKU[%d]数量配置错误: %v (quantityType=%d, quantity=%d)",
			skcIndex, skuIndex, err, *quantityType, *quantity)
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
	DetailedIssues  map[string][]string `json:"detailed_issues"` // 按类别分组的详细问题
	ValidationTime  int64               `json:"validation_time_ms"`
}

// generateValidationReport 生成验证报告
func (v *PublishProductValidator) generateValidationReport(ctx *shein_model.TaskContext) *ValidationReport {
	startTime := time.Now()

	report := &ValidationReport{
		TotalChecks:     4, // 基本信息、主图、多件商品SKU、SKC/SKU数据
		PassedChecks:    0,
		FailedChecks:    0,
		AutoFixedIssues: 0,
		CriticalIssues:  []string{},
		WarningIssues:   []string{},
		FixedIssues:     []string{},
		DetailedIssues:  make(map[string][]string),
	}

	// 1. 验证基本产品信息
	if err := v.validateBasicProductInfo(ctx); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("基本信息: %v", err))
	} else {
		report.PassedChecks++
	}

	// 3. 验证多件商品SKU图片（带自动修复）
	beforeSKUValidation := len(report.FixedIssues)
	if err := v.validateMultiPieceSKUImagesWithReport(ctx, report); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("多件商品SKU图片: %v", err))
	} else {
		report.PassedChecks++
	}
	report.AutoFixedIssues += len(report.FixedIssues) - beforeSKUValidation

	// 4. 验证SKC和SKU数据完整性
	if err := v.validateSKCAndSKUData(ctx); err != nil {
		report.FailedChecks++
		report.CriticalIssues = append(report.CriticalIssues, fmt.Sprintf("SKC/SKU数据: %v", err))
	} else {
		report.PassedChecks++
	}

	report.ValidationTime = time.Since(startTime).Milliseconds()

	return report
}

// validateMultiPieceSKUImagesWithReport 带报告的多件商品SKU图片验证
func (v *PublishProductValidator) validateMultiPieceSKUImagesWithReport(ctx *shein_model.TaskContext, report *ValidationReport) error {
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
					// 这种情况应该在SKC构建阶段已经修复了，如果还出现说明有问题
					issue := fmt.Sprintf("多件商品SKU缺少图片 (SKC[%d] SKU[%d] SupplierSKU: %s) - 应该在构建阶段已修复",
						skcIndex, skuIndex, sku.SupplierSKU)
					multiPieceIssues = append(multiPieceIssues, issue)
				} else {
					// 多件商品SKU已有图片，检查并修复图片排序
					fixedInThisSKU := v.fixSKUImageSorting(sku, sku.SupplierSKU, report)
					fixedCount += fixedInThisSKU
				}
			}
		}
	}

	if len(multiPieceIssues) > 0 {
		// 将详细问题添加到报告中
		report.DetailedIssues["多件商品SKU图片问题"] = multiPieceIssues
		// 返回包含具体问题详情的错误信息
		return fmt.Errorf("发现%d个多件商品SKU图片问题: %s",
			len(multiPieceIssues), strings.Join(multiPieceIssues, "; "))
	}

	if fixedCount > 0 {
		report.WarningIssues = append(report.WarningIssues, fmt.Sprintf("验证阶段修复了%d个SKU图片排序问题", fixedCount))
	}

	return nil
}

// saveValidationReportToFile 保存验证报告到JSON文件
func (v *PublishProductValidator) saveValidationReportToFile(filename string, report *ValidationReport) error {
	// 确保目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 序列化验证报告
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化验证报告失败: %w", err)
	}

	// 写入文件
	filePath := filepath.Join("logs", filename)
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入验证报告文件失败: %w", err)
	}

	return nil
}

// copySKCImageToSKU 从SKC复制图片到SKU
func (v *PublishProductValidator) copySKCImageToSKU(skc interface{}, sku interface{}) error {
	// 由于我们知道这些是product.SKC和product.SKU类型，我们需要进行类型转换
	// 但是由于它们在JSON序列化后可能是map[string]interface{}类型，我们需要处理这种情况

	// 尝试作为map处理（JSON反序列化后的情况）
	skcMap, skcOk := skc.(map[string]interface{})
	skuMap, skuOk := sku.(map[string]interface{})

	if !skcOk || !skuOk {
		return fmt.Errorf("SKC或SKU类型转换失败")
	}

	// 获取SKC的图片信息
	skcImageInfo, exists := skcMap["image_info"]
	if !exists {
		return fmt.Errorf("SKC没有图片信息")
	}

	skcImageMap, ok := skcImageInfo.(map[string]interface{})
	if !ok {
		return fmt.Errorf("SKC图片信息格式错误")
	}

	skcImageList, exists := skcImageMap["image_info_list"]
	if !exists {
		return fmt.Errorf("SKC没有图片列表")
	}

	skcImages, ok := skcImageList.([]interface{})
	if !ok || len(skcImages) == 0 {
		return fmt.Errorf("SKC图片列表为空")
	}

	// 获取第一张图片
	firstImage, ok := skcImages[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("SKC第一张图片格式错误")
	}

	// 创建SKU图片信息
	skuImageInfo := map[string]interface{}{
		"image_group_code": nil,
		"image_info_list": []interface{}{
			map[string]interface{}{
				"image_type":              firstImage["image_type"],
				"image_sort":              1, // SKU图片排序固定为1
				"image_url":               firstImage["image_url"],
				"image_item_id":           firstImage["image_item_id"],
				"size_img_flag":           firstImage["size_img_flag"],
				"transformCvSizeImage":    firstImage["transformCvSizeImage"],
				"ai_status":               firstImage["ai_status"],
				"ps_types":                firstImage["ps_types"],
				"marketing_main_image":    false, // SKU图片不是营销主图
				"commodity_category_flag": firstImage["commodity_category_flag"],
			},
		},
		"original_image_info_list": []interface{}{},
	}

	// 设置SKU的图片信息
	skuMap["image_info"] = skuImageInfo

	return nil
}

// fixSKUImageSorting 修复SKU图片排序
func (v *PublishProductValidator) fixSKUImageSorting(sku interface{}, supplierSKU string, report *ValidationReport) int {
	skuMap, ok := sku.(map[string]interface{})
	if !ok {
		return 0
	}

	imageInfo, exists := skuMap["image_info"]
	if !exists {
		return 0
	}

	imageInfoMap, ok := imageInfo.(map[string]interface{})
	if !ok {
		return 0
	}

	imageList, exists := imageInfoMap["image_info_list"]
	if !exists {
		return 0
	}

	images, ok := imageList.([]interface{})
	if !ok || len(images) == 0 {
		return 0
	}

	fixedCount := 0

	// 多件商品SKU只能有一张图片
	if len(images) > 1 {
		fixMsg := fmt.Sprintf("修复多件商品SKU图片数量: SKU %s 从%d张减少到1张",
			supplierSKU, len(images))
		report.FixedIssues = append(report.FixedIssues, fixMsg)
		// 只保留第一张图片
		imageInfoMap["image_info_list"] = []interface{}{images[0]}
		images = []interface{}{images[0]}
		fixedCount++
	}

	// 检查第一张图片的排序
	if len(images) > 0 {
		if imageMap, ok := images[0].(map[string]interface{}); ok {
			if imageSort, exists := imageMap["image_sort"]; exists {
				if sortValue, ok := imageSort.(float64); ok && int(sortValue) != 1 {
					fixMsg := fmt.Sprintf("修复多件商品SKU主图排序: SKU %s 从%d修复为1",
						supplierSKU, int(sortValue))
					report.FixedIssues = append(report.FixedIssues, fixMsg)
					imageMap["image_sort"] = 1
					fixedCount++
				}
			}
		}
	}

	return fixedCount
}
