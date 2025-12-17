package handlers

import (
	"fmt"
	"strings"

	"task-processor/internal/platforms/temu/types"
)

// validateAndFixAIResponse 验证和修复AI响应，处理临时spec_id
func (sb *SkuBuilder) validateAndFixAIResponse(aiResponse *AISkuMappingResponse, temuSpecProperties []GoodsSpecProperty) {
	// 构建parent_spec_id到可用spec_id的映射
	parentSpecToValidSpecIDs := make(map[string]map[string]PropertyValue)
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
				sb.logger.Debugf("识别到用户输入规格: %s (parent_spec_id: %s)，将接受任意spec_id",
					specProp.Name, specProp.ParentSpecID)
				continue
			}

			if parentSpecToValidSpecIDs[specProp.ParentSpecID] == nil {
				parentSpecToValidSpecIDs[specProp.ParentSpecID] = make(map[string]PropertyValue)
			}

			// 添加所有可用的spec_id
			for _, value := range specProp.Values {
				if value.SpecID != "" {
					parentSpecToValidSpecIDs[specProp.ParentSpecID][value.SpecID] = value
				}
			}
		}
	}

	sb.logger.Infof("构建了%d个parent_spec_id的有效spec_id映射，其中%d个为用户输入规格",
		len(parentSpecToValidSpecIDs), len(userInputSpecs))

	// 验证和修复每个SKU的spec
	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		validSpecs := []types.SpecInfo{}

		for _, spec := range sku.Spec {
			// 检查parent_spec_id是否存在于模板中
			if !parentSpecExists[spec.ParentSpecID] {
				sb.logger.Warnf("SKU[%d] parent_spec_id %s 不存在于模板中，跳过该规格", i, spec.ParentSpecID)
				continue
			}

			// 如果是用户输入规格，直接接受AI提供的spec_id和spec_name
			if userInputSpecs[spec.ParentSpecID] {
				validSpecs = append(validSpecs, types.SpecInfo{
					SpecID:         spec.SpecID,
					SpecName:       spec.SpecName,
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: parentSpecNames[spec.ParentSpecID],
				})
				sb.logger.Debugf("✅ SKU[%d] 用户输入规格 parent_spec_id=%s, spec_id=%s, spec_name=%s",
					i, spec.ParentSpecID, spec.SpecID, spec.SpecName)
				continue
			}

			if validSpecIDs, exists := parentSpecToValidSpecIDs[spec.ParentSpecID]; exists {
				if _, specExists := validSpecIDs[spec.SpecID]; specExists {
					// spec_id有效，保留（使用AI提供的spec_name，不是模板中的值）
					validSpecs = append(validSpecs, types.SpecInfo{
						SpecID:         spec.SpecID,
						SpecName:       spec.SpecName, // 使用AI提供的具体值，不是模板中的规格维度名称
						ParentSpecID:   spec.ParentSpecID,
						ParentSpecName: parentSpecNames[spec.ParentSpecID],
					})
					sb.logger.Debugf("✅ SKU[%d] spec_id %s 验证通过，spec_name=%s", i, spec.SpecID, spec.SpecName)
				} else {
					// spec_id无效，尝试通过spec_name匹配
					matched := false
					for _, validValue := range validSpecIDs {
						if strings.EqualFold(validValue.Value, spec.SpecName) {
							validSpecs = append(validSpecs, types.SpecInfo{
								SpecID:         validValue.SpecID,
								SpecName:       validValue.Value,
								ParentSpecID:   spec.ParentSpecID,
								ParentSpecName: parentSpecNames[spec.ParentSpecID],
							})
							sb.logger.Infof("🔧 SKU[%d] 通过名称匹配修复: %s -> %s", i, spec.SpecID, validValue.SpecID)
							matched = true
							break
						}
					}
					if !matched {
						// 没有匹配的预定义值，创建临时ID，后续通过API解析
						tempSpecID := fmt.Sprintf("TEMP_%s", spec.SpecName)
						validSpecs = append(validSpecs, types.SpecInfo{
							SpecID:         tempSpecID,
							SpecName:       spec.SpecName,
							ParentSpecID:   spec.ParentSpecID,
							ParentSpecName: parentSpecNames[spec.ParentSpecID],
						})
						sb.logger.Infof("🔧 SKU[%d] 创建临时spec_id: %s -> %s", i, spec.SpecName, tempSpecID)
					}
				}
			} else {
				// parent_spec_id存在但没有预定义值，创建临时ID
				tempSpecID := fmt.Sprintf("TEMP_%s", spec.SpecName)
				validSpecs = append(validSpecs, types.SpecInfo{
					SpecID:         tempSpecID,
					SpecName:       spec.SpecName,
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: parentSpecNames[spec.ParentSpecID],
				})
				sb.logger.Infof("🔧 SKU[%d] parent_spec_id %s 无预定义值，创建临时spec_id: %s", i, spec.ParentSpecID, tempSpecID)
			}
		}

		// 更新SKU的spec列表
		sku.Spec = validSpecs

		// 暂时不更新ColorSpecID和SpecID，等临时ID解析完成后再处理
	}

	sb.logger.Infof("AI响应验证完成，临时spec_id将在后续步骤中解析")
}

// enforceSpecCountLimit 强制执行规格数量限制（最多2个）
func (sb *SkuBuilder) enforceSpecCountLimit(aiResponse *AISkuMappingResponse) {
	// 1. 统计所有SKU使用的parent_spec_id
	parentSpecUsage := make(map[string]int)
	parentSpecNames := make(map[string]string)

	for _, sku := range aiResponse.SkuList {
		for _, spec := range sku.Spec {
			parentSpecUsage[spec.ParentSpecID]++
			parentSpecNames[spec.ParentSpecID] = spec.ParentSpecName
		}
	}

	// 2. 如果parent_spec_id数量<=2，无需处理
	if len(parentSpecUsage) <= 2 {
		sb.logger.Infof("✅ 规格维度数量符合要求: %d个", len(parentSpecUsage))
		return
	}

	// 3. 超过2个，需要选择最重要的2个
	sb.logger.Warnf("⚠️ 检测到%d个规格维度，超过TEMU限制(2个)，将自动选择最重要的2个", len(parentSpecUsage))

	// 按优先级排序：颜色 > 尺寸 > 其他
	type specPriority struct {
		parentSpecID   string
		parentSpecName string
		priority       int
		usage          int
	}

	var specs []specPriority
	for parentSpecID, usage := range parentSpecUsage {
		priority := sb.getSpecPriority(parentSpecNames[parentSpecID])
		specs = append(specs, specPriority{
			parentSpecID:   parentSpecID,
			parentSpecName: parentSpecNames[parentSpecID],
			priority:       priority,
			usage:          usage,
		})
	}

	// 排序：优先级高的在前，优先级相同时使用频率高的在前
	for i := 0; i < len(specs); i++ {
		for j := i + 1; j < len(specs); j++ {
			if specs[i].priority > specs[j].priority ||
				(specs[i].priority == specs[j].priority && specs[i].usage < specs[j].usage) {
				specs[i], specs[j] = specs[j], specs[i]
			}
		}
	}

	// 选择前2个
	selectedSpecs := make(map[string]bool)
	for i := 0; i < 2 && i < len(specs); i++ {
		selectedSpecs[specs[i].parentSpecID] = true
		sb.logger.Infof("✅ 选择规格维度[%d]: %s (parent_spec_id=%s, 使用次数=%d)",
			i+1, specs[i].parentSpecName, specs[i].parentSpecID, specs[i].usage)
	}

	// 记录被忽略的规格
	for i := 2; i < len(specs); i++ {
		sb.logger.Warnf("⚠️ 忽略规格维度: %s (parent_spec_id=%s, 使用次数=%d)",
			specs[i].parentSpecName, specs[i].parentSpecID, specs[i].usage)
	}

	// 4. 过滤每个SKU的spec，只保留选中的parent_spec_id
	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		filteredSpecs := []types.SpecInfo{}

		for _, spec := range sku.Spec {
			if selectedSpecs[spec.ParentSpecID] {
				filteredSpecs = append(filteredSpecs, spec)
			}
		}

		if len(filteredSpecs) != len(sku.Spec) {
			sb.logger.Infof("🔧 SKU[%d] 规格从%d个减少到%d个", i, len(sku.Spec), len(filteredSpecs))
		}

		sku.Spec = filteredSpecs
	}

	sb.logger.Infof("✅ 规格数量限制强制执行完成，所有SKU现在使用%d个规格维度", len(selectedSpecs))
}

// getSpecPriority 获取规格的优先级（数字越小优先级越高）
func (sb *SkuBuilder) getSpecPriority(specName string) int {
	specNameLower := strings.ToLower(specName)

	// 颜色相关：优先级1
	if sb.isColorSpec(specNameLower) {
		return 1
	}

	// 尺寸相关：优先级2
	if sb.isSizeSpec(specNameLower) {
		return 2
	}

	// 其他：优先级3
	return 3
}
