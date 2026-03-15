package product

import (
	"fmt"
	"strings"
	temucontext "task-processor/internal/platforms/temu/context"
	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// ProductSubmitValidator 产品提交验证器
type ProductSubmitValidator struct {
	logger *logrus.Entry
}

// NewProductSubmitValidator 创建产品提交验证器
func NewProductSubmitValidator(logger *logrus.Entry) *ProductSubmitValidator {
	return &ProductSubmitValidator{
		logger: logger,
	}
}

// ValidateSpecCompleteness 验证规格完整性
func (v *ProductSubmitValidator) ValidateSpecCompleteness(temuCtx *temucontext.TemuTaskContext) (map[string]map[string]bool, error) {
	temuProduct := temuCtx.TemuProduct

	// 收集所有SKU使用的规格维度
	specDimensions := make(map[string]map[string]bool) // parent_spec_id -> spec_id -> exists

	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				if specDimensions[spec.ParentSpecID] == nil {
					specDimensions[spec.ParentSpecID] = make(map[string]bool)
				}
				specDimensions[spec.ParentSpecID][spec.SpecID] = true
			}
		}
	}

	// 验证规格属性配置是否包含所有使用的规格，并检查关键字段
	configuredSpecs := make(map[string]map[string]bool)   // parent_spec_id -> spec_id -> exists
	incompleteSpecs := make(map[string]map[string]string) // parent_spec_id -> spec_id -> missing_field

	for _, specProp := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties {
		if configuredSpecs[specProp.ParentSpecID] == nil {
			configuredSpecs[specProp.ParentSpecID] = make(map[string]bool)
			incompleteSpecs[specProp.ParentSpecID] = make(map[string]string)
		}
		configuredSpecs[specProp.ParentSpecID][specProp.SpecID] = true

		// 检查关键字段是否缺失
		if specProp.Vid == 0 {
			incompleteSpecs[specProp.ParentSpecID][specProp.SpecID] = "missing_vid"
			v.logger.WithFields(logrus.Fields{
				"parent_spec_name": specProp.ParentSpecName,
				"parent_spec_id":   specProp.ParentSpecID,
				"spec_value":       specProp.Value,
				"spec_id":          specProp.SpecID,
			}).Warn("规格配置缺少vid字段")
		}
	}

	// 检查是否有缺失的规格配置或不完整的规格
	var missingSpecs []string
	var incompleteSpecsList []string

	for parentSpecID, specIDs := range specDimensions {
		for specID := range specIDs {
			if configuredSpecs[parentSpecID] == nil || !configuredSpecs[parentSpecID][specID] {
				// 查找规格名称
				specName := v.findSpecName(temuCtx, parentSpecID, specID)
				missingSpecs = append(missingSpecs, fmt.Sprintf("%s(%s)", specName, specID))
			} else if incompleteSpecs[parentSpecID] != nil && incompleteSpecs[parentSpecID][specID] != "" {
				// 规格存在但不完整
				specName := v.findSpecName(temuCtx, parentSpecID, specID)
				incompleteSpecsList = append(incompleteSpecsList, fmt.Sprintf("%s(%s)-%s", specName, specID, incompleteSpecs[parentSpecID][specID]))
			}
		}
	}

	if len(missingSpecs) > 0 || len(incompleteSpecsList) > 0 {
		if len(missingSpecs) > 0 {
			v.logger.WithFields(logrus.Fields{
				"missing_specs": missingSpecs,
			}).Error("检测到缺失的规格配置")
		}
		if len(incompleteSpecsList) > 0 {
			v.logger.WithFields(logrus.Fields{
				"incomplete_specs": incompleteSpecsList,
			}).Error("检测到不完整的规格配置")
		}
		v.logger.Error("这可能导致TEMU API返回'reset the variants template'错误")

		return specDimensions, fmt.Errorf("缺失或不完整的规格配置: missing=%v, incomplete=%v", missingSpecs, incompleteSpecsList)
	}

	return specDimensions, nil
}

// ValidateMultiplePackageConfiguration 验证多件套包装配置
func (v *ProductSubmitValidator) ValidateMultiplePackageConfiguration(temuCtx *temucontext.TemuTaskContext) (bool, int, error) {
	temuProduct := temuCtx.TemuProduct

	// 检查产品属性中是否设置为多件套
	isMultipleSets := false
	var multipleSetsPropIndex = -1
	for i, prop := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties {
		if strings.Contains(strings.ToLower(prop.Value), "multiple sets") {
			isMultipleSets = true
			multipleSetsPropIndex = i
			v.logger.WithFields(logrus.Fields{
				"property_value": prop.Value,
			}).Info("检测到产品属性设置为多件套")
			break
		}
	}

	if !isMultipleSets {
		// 不是多件套产品，无需验证
		return false, -1, nil
	}

	// 检查所有SKU的包装配置是否正确
	hasInvalidPackaging := false
	for skcIndex, skc := range temuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			mp := sku.MultiplePackage

			// 多件套产品的包装数量必须大于1
			if mp.NumberOfPieces <= 1 {
				v.logger.WithFields(logrus.Fields{
					"skc_index": skcIndex,
					"sku_index": skuIndex,
				}).Warn("检测到多件套产品但包装数量为1")
				hasInvalidPackaging = true
				break
			}
		}
		if hasInvalidPackaging {
			break
		}
	}

	return hasInvalidPackaging, multipleSetsPropIndex, nil
}

// findSpecName 查找规格名称
func (v *ProductSubmitValidator) findSpecName(temuCtx *temucontext.TemuTaskContext, parentSpecID, specID string) string {
	// 从AI映射中查找
	if temuCtx.AISkuMapping != nil {
		for _, aiSku := range temuCtx.AISkuMapping.SkuList {
			for _, spec := range aiSku.Spec {
				if spec.ParentSpecID == parentSpecID && spec.SpecID == specID {
					return spec.SpecName
				}
			}
		}
	}

	// 从SKU中查找
	for _, skc := range temuCtx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				if spec.ParentSpecID == parentSpecID && spec.SpecID == specID {
					return spec.SpecName
				}
			}
		}
	}

	return fmt.Sprintf("Unknown_%s", specID)
}

// FindParentSpecName 查找父规格名称
func (v *ProductSubmitValidator) FindParentSpecName(temuCtx *temucontext.TemuTaskContext, parentSpecID string) string {
	// 从AI映射中查找
	if temuCtx.AISkuMapping != nil {
		for _, aiSku := range temuCtx.AISkuMapping.SkuList {
			for _, spec := range aiSku.Spec {
				if spec.ParentSpecID == parentSpecID {
					return spec.ParentSpecName
				}
			}
		}
	}

	// 从SKU中查找
	for _, skc := range temuCtx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				if spec.ParentSpecID == parentSpecID {
					return spec.ParentSpecName
				}
			}
		}
	}

	return fmt.Sprintf("Unknown_%s", parentSpecID)
}

// FindVidFromTemplate 从模板中查找vid
func (v *ProductSubmitValidator) FindVidFromTemplate(templateInfo *temutemplate.TemplateInfo, parentSpecID, specID string) int {
	for _, specProp := range templateInfo.GoodsSpecProperties {
		if specProp.ParentSpecID == parentSpecID {
			for _, value := range specProp.Values {
				if value.SpecID == specID {
					return value.VID
				}
			}
		}
	}
	return 0
}

// FindTemplatePidFromTemplate 从模板中查找template_pid
func (v *ProductSubmitValidator) FindTemplatePidFromTemplate(templateInfo *temutemplate.TemplateInfo, parentSpecID string) int {
	for _, specProp := range templateInfo.GoodsSpecProperties {
		if specProp.ParentSpecID == parentSpecID {
			return specProp.TemplatePID
		}
	}
	return 0
}

// FindTemplateModuleIdFromTemplate 从模板中查找template_module_id
func (v *ProductSubmitValidator) FindTemplateModuleIdFromTemplate(templateInfo *temutemplate.TemplateInfo, parentSpecID string) int {
	for _, specProp := range templateInfo.GoodsSpecProperties {
		if specProp.ParentSpecID == parentSpecID {
			return specProp.TemplateModuleID
		}
	}
	return 0
}
