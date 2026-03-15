// Package sku 提供TEMU平台的AI SKU映射验证功能
package sku

import (
	"fmt"
	"strings"

	temucontext "task-processor/internal/platforms/temu/context"
	temutemplate "task-processor/internal/platforms/temu/api/template"
)

// validateAsinAttributeMapping 验证ASIN与属性映射的正确性
func (vp *SkuVariantProcessor) validateAsinAttributeMapping(asinToAttributes map[string]map[string]any, aiVariants []temucontext.AmazonVariantForAI) {
	vp.logger.Info("🔍 开始验证ASIN与属性映射的正确性")

	validCount := 0
	invalidCount := 0
	missingCount := 0

	for i, aiVariant := range aiVariants {
		asin := aiVariant.Asin

		// 检查映射是否存在
		if attributes, exists := asinToAttributes[asin]; exists {
			// 验证映射的属性与变体的属性是否一致
			if len(attributes) > 0 {
				validCount++
				vp.logger.Infof("✅ ASIN[%d] %s 映射验证通过: %+v", i, asin, attributes)

				// 详细对比每个属性值
				for key, mappedValue := range attributes {
					if variantValue, ok := aiVariant.Attributes[key]; ok {
						if fmt.Sprintf("%v", mappedValue) != fmt.Sprintf("%v", variantValue) {
							vp.logger.Errorf("❌ ASIN %s 属性值不匹配: key=%s, 映射值=%v, 变体值=%v",
								asin, key, mappedValue, variantValue)
							invalidCount++
						}
					}
				}
			} else {
				vp.logger.Warnf("⚠️ ASIN[%d] %s 映射存在但属性为空", i, asin)
				invalidCount++
			}
		} else {
			vp.logger.Warnf("⚠️ ASIN[%d] %s 未找到属性映射", i, asin)
			missingCount++
		}
	}

	vp.logger.Infof("🔍 ASIN属性映射验证完成: 有效=%d, 无效=%d, 缺失=%d",
		validCount, invalidCount, missingCount)
}

// validateAndFixAIResponse 验证和修复AI响应
func (vp *SkuVariantProcessor) validateAndFixAIResponse(response *temucontext.AISkuMappingResponse, temuSpecProperties []temutemplate.TemplateRespGoodsSpecProperty) {
	vp.logger.Info("🔧 开始验证和修复AI响应")

	// 构建parent_spec_id到可用spec_id的映射
	parentSpecToValidSpecIDs := make(map[string]map[string]temutemplate.PropertyValue)
	parentSpecExists := make(map[string]bool)
	parentSpecNames := make(map[string]string) // parent_spec_id -> parent_spec_name
	userInputSpecs := make(map[string]bool)    // 标记哪些是用户输入规格（无预定义值）

	for _, specProp := range temuSpecProperties {
		if specProp.ParentSpecID != "" {
			parentSpecExists[specProp.ParentSpecID] = true
			parentSpecNames[specProp.ParentSpecID] = specProp.Name // 保存parent_spec_name

			// 如果Values为空，说明这是用户输入规格，不需要验证spec_id
			if len(specProp.Values) == 0 {
				userInputSpecs[specProp.ParentSpecID] = true
				vp.logger.Debugf("识别到用户输入规格: %s (parent_spec_id: %s)，将接受任意spec_id",
					specProp.Name, specProp.ParentSpecID)
				continue
			}

			if parentSpecToValidSpecIDs[specProp.ParentSpecID] == nil {
				parentSpecToValidSpecIDs[specProp.ParentSpecID] = make(map[string]temutemplate.PropertyValue)
			}

			// 添加所有可用的spec_id
			for _, value := range specProp.Values {
				if value.SpecID != "" {
					parentSpecToValidSpecIDs[specProp.ParentSpecID][value.SpecID] = value
				}
			}
		}
	}

	vp.logger.Infof("构建了%d个parent_spec_id的有效spec_id映射，其中%d个为用户输入规格",
		len(parentSpecToValidSpecIDs), len(userInputSpecs))

	// 验证和修复每个SKU的spec
	fixedCount := 0
	for i := range response.SkuList {
		sku := &response.SkuList[i]
		validSpecs := make([]temucontext.SpecInfo, 0, len(sku.Spec))

		for _, spec := range sku.Spec {
			// 检查parent_spec_id是否存在于模板中
			if !parentSpecExists[spec.ParentSpecID] {
				vp.logger.Warnf("SKU[%d] parent_spec_id %s 不存在于模板中，跳过该规格", i, spec.ParentSpecID)
				continue
			}

			// 创建新的spec，确保包含parent_spec_name
			newSpec := temucontext.SpecInfo{
				SpecID:         spec.SpecID,
				SpecName:       spec.SpecName,
				ParentSpecID:   spec.ParentSpecID,
				ParentSpecName: parentSpecNames[spec.ParentSpecID], // 从模板中获取parent_spec_name
			}

			// 如果是用户输入规格，直接接受AI提供的spec_id和spec_name，但确保使用TEMP_格式
			if userInputSpecs[spec.ParentSpecID] {
				// 对于用户输入规格，如果spec_id不是TEMP_格式，强制转换
				if !strings.HasPrefix(spec.SpecID, "TEMP_") {
					tempSpecID := fmt.Sprintf("TEMP_%s", spec.SpecName)
					newSpec.SpecID = tempSpecID
					vp.logger.Infof("🔧 SKU[%d] 用户输入规格强制转换为TEMP_格式: %s -> %s",
						i, spec.SpecID, tempSpecID)
					fixedCount++
				}
				validSpecs = append(validSpecs, newSpec)
				vp.logger.Debugf("✅ SKU[%d] 用户输入规格 parent_spec_id=%s, spec_id=%s, spec_name=%s",
					i, spec.ParentSpecID, newSpec.SpecID, spec.SpecName)
				continue
			}

			// 验证预定义规格的spec_id
			if validSpecIDs, exists := parentSpecToValidSpecIDs[spec.ParentSpecID]; exists {
				if _, specExists := validSpecIDs[spec.SpecID]; specExists {
					// spec_id有效，保留
					validSpecs = append(validSpecs, newSpec)
					vp.logger.Debugf("✅ SKU[%d] spec_id %s 验证通过，spec_name=%s", i, spec.SpecID, spec.SpecName)
				} else {
					// spec_id无效，尝试通过spec_name匹配
					matched := false

					// 首先尝试精确匹配
					for _, validValue := range validSpecIDs {
						if strings.EqualFold(validValue.Value, spec.SpecName) {
							newSpec.SpecID = validValue.SpecID
							newSpec.SpecName = validValue.Value
							validSpecs = append(validSpecs, newSpec)
							vp.logger.Infof("🔧 SKU[%d] 通过精确名称匹配修复: %s -> %s (名称: %s -> %s)",
								i, spec.SpecID, validValue.SpecID, spec.SpecName, validValue.Value)
							matched = true
							fixedCount++
							break
						}
					}

					// 如果精确匹配失败，尝试包含匹配
					if !matched {
						for _, validValue := range validSpecIDs {
							if strings.Contains(strings.ToLower(validValue.Value), strings.ToLower(spec.SpecName)) ||
								strings.Contains(strings.ToLower(spec.SpecName), strings.ToLower(validValue.Value)) {
								newSpec.SpecID = validValue.SpecID
								newSpec.SpecName = validValue.Value
								validSpecs = append(validSpecs, newSpec)
								vp.logger.Infof("🔧 SKU[%d] 通过模糊名称匹配修复: %s -> %s (名称: %s -> %s)",
									i, spec.SpecID, validValue.SpecID, spec.SpecName, validValue.Value)
								matched = true
								fixedCount++
								break
							}
						}
					}

					if !matched {
						// 没有匹配的预定义值，创建临时ID
						tempSpecID := fmt.Sprintf("TEMP_%s", spec.SpecName)
						newSpec.SpecID = tempSpecID
						validSpecs = append(validSpecs, newSpec)
						vp.logger.Warnf("⚠️ SKU[%d] 无效spec_id '%s' 且无法匹配规格名称 '%s'，强制转换为临时ID: %s",
							i, spec.SpecID, spec.SpecName, tempSpecID)
						vp.logger.Warnf("⚠️ 可用的规格值: %v", func() []string {
							var values []string
							for _, v := range validSpecIDs {
								values = append(values, v.Value)
							}
							return values
						}())
						fixedCount++
					}
				}
			} else {
				// parent_spec_id存在但没有预定义值，创建临时ID
				tempSpecID := fmt.Sprintf("TEMP_%s", spec.SpecName)
				newSpec.SpecID = tempSpecID
				validSpecs = append(validSpecs, newSpec)
				vp.logger.Infof("🔧 SKU[%d] parent_spec_id %s 无预定义值，创建临时spec_id: %s", i, spec.ParentSpecID, tempSpecID)
				fixedCount++
			}
		}

		// 更新SKU的spec列表
		sku.Spec = validSpecs
	}

	if fixedCount > 0 {
		vp.logger.Infof("🔧 修复了 %d 个规格问题，并为所有规格补充了parent_spec_name", fixedCount)

		// 再次验证是否还有无效的spec_id
		invalidSpecCount := 0
		for _, sku := range response.SkuList {
			for _, spec := range sku.Spec {
				// 检查是否有非TEMP_格式且不在模板中的spec_id
				if !strings.HasPrefix(spec.SpecID, "TEMP_") {
					if validSpecIDs, exists := parentSpecToValidSpecIDs[spec.ParentSpecID]; exists {
						if _, specExists := validSpecIDs[spec.SpecID]; !specExists {
							vp.logger.Errorf("❌ 发现未修复的无效spec_id: %s (parent_spec_id: %s)",
								spec.SpecID, spec.ParentSpecID)
							invalidSpecCount++
						}
					}
				}
			}
		}

		if invalidSpecCount > 0 {
			vp.logger.Errorf("❌ 仍有 %d 个无效spec_id未被修复", invalidSpecCount)
		}
	} else {
		vp.logger.Info("✅ AI响应验证通过，已为所有规格补充parent_spec_name")
	}
}
