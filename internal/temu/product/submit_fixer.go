package product

import (
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/template"

	"github.com/sirupsen/logrus"
)

// ProductSubmitFixer 产品提交修复器
type ProductSubmitFixer struct {
	logger    *logrus.Entry
	validator *ProductSubmitValidator
}

// NewProductSubmitFixer 创建产品提交修复器
func NewProductSubmitFixer(logger *logrus.Entry, validator *ProductSubmitValidator) *ProductSubmitFixer {
	return &ProductSubmitFixer{
		logger:    logger,
		validator: validator,
	}
}

// AutoFixSpecConfiguration 自动修复规格配置
func (f *ProductSubmitFixer) AutoFixSpecConfiguration(temuCtx *temucontext.TemuTaskContext, specDimensions map[string]map[string]bool) error {
	temuProduct := temuCtx.TemuProduct

	// 获取模板信息用于验证
	templateInfo, hasTemplate := template.GetTemplateInfoFromContext(temuCtx)

	// 首先修复现有规格的缺失字段
	for i := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties {
		specProp := &temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties[i]

		// 如果vid缺失，尝试从模板中获取
		if specProp.Vid == 0 && hasTemplate {
			if vid := f.validator.FindVidFromTemplate(templateInfo, specProp.ParentSpecID, specProp.SpecID); vid > 0 {
				specProp.Vid = vid
				f.logger.WithFields(logrus.Fields{
					"parent_spec_name": specProp.ParentSpecName,
					"parent_spec_id":   specProp.ParentSpecID,
					"spec_value":       specProp.Value,
					"spec_id":          specProp.SpecID,
					"vid":              vid,
				}).Info("已为规格配置添加vid")
			}
		}

		// 如果template相关字段缺失，尝试从模板中获取
		if specProp.TemplatePid == 0 && hasTemplate {
			if templatePid := f.validator.FindTemplatePidFromTemplate(templateInfo, specProp.ParentSpecID); templatePid > 0 {
				specProp.TemplatePid = templatePid
			}
		}

		if specProp.TemplateModuleID == 0 && hasTemplate {
			if moduleId := f.validator.FindTemplateModuleIdFromTemplate(templateInfo, specProp.ParentSpecID); moduleId > 0 {
				specProp.TemplateModuleID = moduleId
			}
		}
	}

	// 然后添加缺失的规格配置
	for parentSpecID, specIDs := range specDimensions {
		for specID := range specIDs {
			// 检查是否已配置
			found := false
			for _, existing := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties {
				if existing.ParentSpecID == parentSpecID && existing.SpecID == specID {
					found = true
					break
				}
			}

			if !found {
				// 查找规格信息
				specName := f.validator.findSpecName(temuCtx, parentSpecID, specID)
				parentSpecName := f.validator.FindParentSpecName(temuCtx, parentSpecID)

				// 创建规格属性配置
				specProp := models.GoodSpecProperty{
					Value:          specName,
					SpecID:         specID,
					ParentSpecID:   parentSpecID,
					ParentSpecName: parentSpecName,
					Checked:        true,
					ControlType:    0,
					Disabled:       false,
					Name:           parentSpecName,
					IsCustomized:   1,
				}

				// 如果有模板信息，尝试获取vid和template相关字段
				if hasTemplate {
					if vid := f.validator.FindVidFromTemplate(templateInfo, parentSpecID, specID); vid > 0 {
						specProp.Vid = vid
					}
					if templatePid := f.validator.FindTemplatePidFromTemplate(templateInfo, parentSpecID); templatePid > 0 {
						specProp.TemplatePid = templatePid
					}
					if moduleId := f.validator.FindTemplateModuleIdFromTemplate(templateInfo, parentSpecID); moduleId > 0 {
						specProp.TemplateModuleID = moduleId
					}
				}

				// 添加到规格属性列表
				temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties = append(
					temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties,
					specProp,
				)

				f.logger.WithFields(logrus.Fields{
					"parent_spec_name": parentSpecName,
					"parent_spec_id":   parentSpecID,
					"spec_name":        specName,
					"spec_id":          specID,
				}).Info("已添加缺失的规格配置")
			}
		}
	}

	return nil
}

// FixMultiplePackageConfiguration 修复多件套包装配置
func (f *ProductSubmitFixer) FixMultiplePackageConfiguration(temuCtx *temucontext.TemuTaskContext, multipleSetsPropIndex int) error {
	temuProduct := temuCtx.TemuProduct

	f.logger.Warn("多件套配置不正确，将产品属性修改为单件")

	// 直接删除"Multiple Sets"属性
	if multipleSetsPropIndex >= 0 {
		temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties = append(
			temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties[:multipleSetsPropIndex],
			temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties[multipleSetsPropIndex+1:]...)
		f.logger.Info("已删除'Multiple Sets'属性")
	}

	// 确保所有SKU的包装配置为单件
	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			sku.MultiplePackage.SkuClassification = 1 // 单品
			sku.MultiplePackage.NumberOfPieces = 1
			sku.MultiplePackage.NumberOfPiecesNew = "1"
			sku.MultiplePackage.IndividuallyPacked = 1
		}
	}

	f.logger.Info("已将产品配置修改为单件")
	return nil
}
