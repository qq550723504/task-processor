package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// ProductSubmitErrorAnalyzer 产品提交错误分析器
type ProductSubmitErrorAnalyzer struct {
	logger *logrus.Entry
}

// NewProductSubmitErrorAnalyzer 创建新的错误分析器
func NewProductSubmitErrorAnalyzer(logger *logrus.Entry) *ProductSubmitErrorAnalyzer {
	return &ProductSubmitErrorAnalyzer{
		logger: logger,
	}
}

// AnalyzeError 分析错误并保存调试信息
func (ea *ProductSubmitErrorAnalyzer) AnalyzeError(temuCtx *temucontext.TemuTaskContext, errorCode int, errorMessage string) {
	// 检查是否为模板相关错误
	isTemplateError := strings.Contains(errorMessage, "reset the variants template") ||
		strings.Contains(errorMessage, "required variants") ||
		strings.Contains(errorMessage, "keyword attribute") ||
		strings.Contains(errorMessage, "template")

	// 检查是否为属性验证错误
	isAttributeError := strings.Contains(errorMessage, "verification error") ||
		strings.Contains(errorMessage, "is not valid") ||
		strings.Contains(errorMessage, "Keyword attribute")

	// 检查是否为多件套包装错误
	isMultiplePackageError := strings.Contains(errorMessage, "Multi-piece set") ||
		strings.Contains(errorMessage, "Total packaging quantity") ||
		strings.Contains(errorMessage, "needs to be greater than 1")

	if isTemplateError || isAttributeError || isMultiplePackageError {
		ea.logger.Warnf("🔍 检测到模板/属性相关错误，开始保存详细信息用于分析")
		if err := ea.saveDetailedTemplateDataForError(temuCtx, errorCode, errorMessage); err != nil {
			ea.logger.Errorf("保存详细模板数据失败: %v", err)
		}
	}

	// 解析错误消息中的必填属性（保持原有逻辑）
	if strings.Contains(errorMessage, "keyword attribute") && strings.Contains(errorMessage, "required") {
		start := strings.Index(errorMessage, "[")
		end := strings.Index(errorMessage, "]")
		if start != -1 && end != -1 && end > start {
			requiredProps := errorMessage[start+1 : end]
			ea.logger.Warnf("🔍 TEMU API要求的必填属性: %s", requiredProps)

			// 保存模板数据用于分析
			if err := ea.saveTemplateDataForError(temuCtx, errorCode, errorMessage, requiredProps); err != nil {
				ea.logger.Errorf("保存模板数据失败: %v", err)
			}
		}
	}

	// 解析属性验证错误
	if strings.Contains(errorMessage, "verification error") {
		ea.parseAttributeValidationError(errorMessage)
	}
}

// saveDetailedTemplateDataForError 保存详细的模板数据用于错误分析
func (ea *ProductSubmitErrorAnalyzer) saveDetailedTemplateDataForError(temuCtx *temucontext.TemuTaskContext, errorCode int, errorMessage string) error {
	// 构建详细的分析数据
	analysisData := map[string]any{
		"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		"error_code":    errorCode,
		"error_message": errorMessage,
		"product_info": map[string]any{
			"out_goods_sn": temuCtx.TemuProduct.GoodsBasic.OutGoodsSN,
			"goods_name":   temuCtx.TemuProduct.GoodsBasic.GoodsName,
			"cat_id":       temuCtx.TemuProduct.GoodsBasic.CatID,
		},
	}

	// 获取模板信息
	if templateInfo, exists := GetTemplateInfoFromContext(temuCtx); exists {
		analysisData["template_info"] = map[string]any{
			"template_id":           templateInfo.TemplateID,
			"goods_properties":      templateInfo.GoodsProperties,
			"goods_spec_properties": templateInfo.GoodsSpecProperties,
		}

		// 分析模板中的规格选项
		specAnalysis := make(map[string]any)
		for _, specProp := range templateInfo.GoodsSpecProperties {
			specInfo := map[string]any{
				"parent_spec_id":      specProp.ParentSpecID,
				"name":                specProp.Name,
				"property_value_type": specProp.PropertyValueType,
				"required":            specProp.Required,
				"values_count":        len(specProp.Values),
				"available_values":    make([]map[string]any, 0),
			}

			// 收集可用的规格值
			for _, value := range specProp.Values {
				specInfo["available_values"] = append(specInfo["available_values"].([]map[string]any), map[string]any{
					"spec_id": value.SpecID,
					"value":   value.Value,
				})
			}

			specAnalysis[specProp.ParentSpecID] = specInfo
		}
		analysisData["template_spec_analysis"] = specAnalysis
	}

	// 获取当前产品的规格属性
	if temuCtx.TemuProduct != nil {
		analysisData["current_product_data"] = map[string]any{
			"goods_properties":      temuCtx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties,
			"goods_spec_properties": temuCtx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties,
			"skc_list":              temuCtx.TemuProduct.SkcList,
		}

		// 分析当前产品的规格使用情况
		currentSpecAnalysis := make(map[string]any)
		for _, skc := range temuCtx.TemuProduct.SkcList {
			for i, sku := range skc.SkuList {
				skuKey := fmt.Sprintf("sku_%d", i)
				skuSpecs := make([]map[string]any, 0)
				for _, spec := range sku.Spec {
					skuSpecs = append(skuSpecs, map[string]any{
						"spec_id":          spec.SpecID,
						"spec_name":        spec.SpecName,
						"parent_spec_id":   spec.ParentSpecID,
						"parent_spec_name": spec.ParentSpecName,
					})
				}
				currentSpecAnalysis[skuKey] = skuSpecs
			}
		}
		analysisData["current_spec_usage"] = currentSpecAnalysis
	}

	// 获取AI映射信息
	if temuCtx.AISkuMapping != nil {
		aiMappingAnalysis := map[string]any{
			"sku_count": len(temuCtx.AISkuMapping.SkuList),
			"sku_specs": make([]map[string]any, 0),
		}

		for i, aiSku := range temuCtx.AISkuMapping.SkuList {
			skuSpecs := make([]map[string]any, 0)
			for _, spec := range aiSku.Spec {
				skuSpecs = append(skuSpecs, map[string]any{
					"spec_id":          spec.SpecID,
					"spec_name":        spec.SpecName,
					"parent_spec_id":   spec.ParentSpecID,
					"parent_spec_name": spec.ParentSpecName,
				})
			}
			aiMappingAnalysis["sku_specs"] = append(aiMappingAnalysis["sku_specs"].([]map[string]any), map[string]any{
				"sku_index": i,
				"asin":      aiSku.Asin,
				"specs":     skuSpecs,
			})
		}
		analysisData["ai_mapping_analysis"] = aiMappingAnalysis
	}

	// 获取用户输入规格信息
	if temuCtx.UserInputParentSpecList != nil {
		analysisData["user_input_specs"] = temuCtx.UserInputParentSpecList
	}

	// 添加问题诊断建议
	diagnostics := []string{}
	if strings.Contains(errorMessage, "reset the variants template") {
		diagnostics = append(diagnostics, "检查AI生成的规格值是否在TEMU模板的预定义值列表中")
		diagnostics = append(diagnostics, "验证parent_spec_id和spec_id的对应关系是否正确")
		diagnostics = append(diagnostics, "确认规格名称和值的映射是否符合TEMU要求")
		diagnostics = append(diagnostics, "🔧 建议：启用规格完整性验证，自动修复缺失的规格配置")
	}
	if strings.Contains(errorMessage, "required variants") {
		diagnostics = append(diagnostics, "检查必需的规格维度是否缺失")
		diagnostics = append(diagnostics, "验证规格组合是否完整")
		diagnostics = append(diagnostics, "🔧 建议：确保goods_spec_properties包含所有SKU使用的规格")
	}
	if strings.Contains(errorMessage, "Multi-piece set") && strings.Contains(errorMessage, "Total packaging quantity") {
		diagnostics = append(diagnostics, "检查产品属性是否设置为'Multiple Sets'但包装数量为1")
		diagnostics = append(diagnostics, "验证multiple_package.number_of_pieces是否大于1")
		diagnostics = append(diagnostics, "🔧 建议：将产品属性改为单件，避免多件套配置复杂性")
		diagnostics = append(diagnostics, "🔧 建议：自动将SKU包装配置修改为单品模式")
	}
	if strings.Contains(errorMessage, "verification error") {
		diagnostics = append(diagnostics, "检查属性值是否在TEMU模板的预定义值列表中")
		diagnostics = append(diagnostics, "验证属性的vid字段是否为有效值（非0）")
		diagnostics = append(diagnostics, "确认AI生成的属性值与模板要求匹配")

		// 解析具体的属性和值
		if attributeName, invalidValue := ea.parseAttributeValidationError(errorMessage); attributeName != "" {
			diagnostics = append(diagnostics, fmt.Sprintf("问题属性: %s, 无效值: %s", attributeName, invalidValue))
		}
	}
	analysisData["diagnostic_suggestions"] = diagnostics

	// 序列化数据
	jsonData, err := ea.marshalWithoutHTMLEscape(analysisData)
	if err != nil {
		return fmt.Errorf("序列化详细模板分析数据失败: %w", err)
	}

	// 保存到文件
	filename := fmt.Sprintf("template_debug_%s_%s.json",
		temuCtx.TemuProduct.GoodsBasic.OutGoodsSN,
		time.Now().Format("20060102_150405"))
	filePath := filepath.Join("logs", filename)

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入详细模板分析文件失败: %w", err)
	}

	ea.logger.Infof("🔍 详细模板错误分析数据已保存到文件: %s", filePath)
	ea.logger.Infof("🔍 文件包含以下分析信息:")
	ea.logger.Infof("   - 模板规格属性和可用值列表")
	ea.logger.Infof("   - 当前产品的规格使用情况")
	ea.logger.Infof("   - AI映射的规格数据")
	ea.logger.Infof("   - 问题诊断建议")
	return nil
}

// saveTemplateDataForError 保存模板数据用于错误分析
func (ea *ProductSubmitErrorAnalyzer) saveTemplateDataForError(temuCtx *temucontext.TemuTaskContext, errorCode int, errorMessage string, requiredProps string) error {
	// 获取模板信息
	templateInfo, exists := GetTemplateInfoFromContext(temuCtx)
	if !exists {
		ea.logger.Warn("未找到模板信息，无法保存模板数据")
		return nil
	}

	// 构建分析数据
	analysisData := map[string]any{
		"timestamp":       time.Now().Format("2006-01-02 15:04:05"),
		"error_code":      errorCode,
		"error_message":   errorMessage,
		"required_by_api": strings.Split(requiredProps, ", "),
		"product_info": map[string]any{
			"out_goods_sn": temuCtx.TemuProduct.GoodsBasic.OutGoodsSN,
			"goods_name":   temuCtx.TemuProduct.GoodsBasic.GoodsName,
			"cat_id":       temuCtx.TemuProduct.GoodsBasic.CatID,
		},
		"template_info": map[string]any{
			"template_id":      templateInfo.TemplateID,
			"goods_properties": templateInfo.GoodsProperties,
		},
		"current_properties": temuCtx.TemuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties,
	}

	// 序列化数据
	jsonData, err := ea.marshalWithoutHTMLEscape(analysisData)
	if err != nil {
		return fmt.Errorf("序列化模板分析数据失败: %w", err)
	}

	// 保存到文件
	filename := fmt.Sprintf("template_error_%s_%s.json",
		temuCtx.TemuProduct.GoodsBasic.OutGoodsSN,
		time.Now().Format("20060102_150405"))
	filePath := filepath.Join("logs", filename)

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入模板分析文件失败: %w", err)
	}

	ea.logger.Infof("🔍 模板错误分析数据已保存到文件: %s", filePath)
	return nil
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (ea *ProductSubmitErrorAnalyzer) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // 关闭HTML转义，避免&被转义为\u0026

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

// parseAttributeValidationError 解析属性验证错误消息
func (ea *ProductSubmitErrorAnalyzer) parseAttributeValidationError(errorMessage string) (attributeName, invalidValue string) {
	// 解析错误消息格式: "Keyword attribute [Material] verification error: the value Cotton, Polyester is not valid."

	// 提取属性名称
	if start := strings.Index(errorMessage, "["); start != -1 {
		if end := strings.Index(errorMessage[start:], "]"); end != -1 {
			attributeName = errorMessage[start+1 : start+end]
		}
	}

	// 提取无效值
	if start := strings.Index(errorMessage, "the value "); start != -1 {
		start += len("the value ")
		if end := strings.Index(errorMessage[start:], " is not valid"); end != -1 {
			invalidValue = errorMessage[start : start+end]
		}
	}

	if attributeName != "" && invalidValue != "" {
		ea.logger.Warnf("🔍 属性验证错误详情: 属性名=%s, 无效值=%s", attributeName, invalidValue)
	}

	return attributeName, invalidValue
}
