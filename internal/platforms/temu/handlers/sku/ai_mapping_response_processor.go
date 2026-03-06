// Package sku 提供TEMU平台的AI SKU映射响应处理功能
package sku

import (
	"fmt"
	"strings"

	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/types"
)

// enforceSpecCountLimit 强制执行规格数量限制（最多2个）
func (vp *SkuVariantProcessor) enforceSpecCountLimit(aiResponse *types.AISkuMappingResponse) {
	// 1. 统计所有SKU使用的parent_spec_id
	parentSpecUsage := make(map[string]int)
	parentSpecNames := make(map[string]string)

	for _, sku := range aiResponse.SkuList {
		for _, spec := range sku.Spec {
			parentSpecUsage[spec.ParentSpecID]++
			parentSpecNames[spec.ParentSpecID] = spec.ParentSpecName
		}
	}

	vp.logger.Infof("📊 规格维度使用统计: 总维度数=%d", len(parentSpecUsage))
	for parentSpecID, usage := range parentSpecUsage {
		vp.logger.Infof("   - %s (%s): 使用次数=%d", parentSpecNames[parentSpecID], parentSpecID, usage)
	}

	// 2. 如果超过2个维度，选择使用频率最高的2个
	if len(parentSpecUsage) <= 2 {
		vp.logger.Info("✅ 规格维度数量符合要求（≤2个），无需调整")
		return
	}

	vp.logger.Warnf("⚠️ 规格维度数量超限（%d > 2），开始选择最重要的2个维度", len(parentSpecUsage))

	// 3. 按优先级和使用频率排序选择规格
	type specPriority struct {
		parentSpecID   string
		parentSpecName string
		priority       int
		usage          int
	}

	var specs []specPriority
	for parentSpecID, usage := range parentSpecUsage {
		priority := vp.getSpecPriority(parentSpecNames[parentSpecID])
		specs = append(specs, specPriority{
			parentSpecID:   parentSpecID,
			parentSpecName: parentSpecNames[parentSpecID],
			priority:       priority,
			usage:          usage,
		})
	}

	// 排序：优先级高的在前，优先级相同时使用频率高的在前
	for i := 0; i < len(specs)-1; i++ {
		for j := i + 1; j < len(specs); j++ {
			if specs[i].priority > specs[j].priority ||
				(specs[i].priority == specs[j].priority && specs[i].usage < specs[j].usage) {
				specs[i], specs[j] = specs[j], specs[i]
			}
		}
	}

	// 4. 选择前2个规格维度
	selectedSpecs := make(map[string]bool)
	for i := 0; i < 2 && i < len(specs); i++ {
		selectedSpecs[specs[i].parentSpecID] = true
		vp.logger.Infof("✅ 选择规格维度[%d]: %s (parent_spec_id=%s, 使用次数=%d)",
			i+1, specs[i].parentSpecName, specs[i].parentSpecID, specs[i].usage)
	}

	// 记录被忽略的规格维度
	for i := 2; i < len(specs); i++ {
		vp.logger.Warnf("⚠️ 忽略规格维度: %s (parent_spec_id=%s, 使用次数=%d)",
			specs[i].parentSpecName, specs[i].parentSpecID, specs[i].usage)
	}

	// 5. 过滤每个SKU的规格，只保留选中的维度
	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		filteredSpecs := make([]models.SpecInfo, 0, 2)

		for _, spec := range sku.Spec {
			if selectedSpecs[spec.ParentSpecID] {
				filteredSpecs = append(filteredSpecs, spec)
			}
		}

		sku.Spec = filteredSpecs

		// 重新生成unique_id
		if len(filteredSpecs) >= 2 {
			sku.UniqueID = fmt.Sprintf("%s_%s", filteredSpecs[0].SpecID, filteredSpecs[1].SpecID)
		} else if len(filteredSpecs) == 1 {
			sku.UniqueID = filteredSpecs[0].SpecID
		}
	}

	vp.logger.Infof("🔧 规格维度限制执行完成，保留了2个最重要的维度")
}

// getSpecPriority 获取规格的优先级（数值越小优先级越高）
func (vp *SkuVariantProcessor) getSpecPriority(specName string) int {
	specNameLower := strings.ToLower(specName)

	// 颜色相关规格优先级最高
	if strings.Contains(specNameLower, "color") || strings.Contains(specNameLower, "颜色") {
		return 1
	}
	// 尺寸相关规格优先级第二
	if strings.Contains(specNameLower, "size") || strings.Contains(specNameLower, "尺寸") ||
		strings.Contains(specNameLower, "尺码") {
		return 2
	}
	// 其他规格优先级较低
	return 3
}

// cleanJSONContent 清理JSON内容
func (vp *SkuVariantProcessor) cleanJSONContent(content string) string {
	// 移除markdown代码块标记
	if after, found := strings.CutPrefix(content, "```json"); found {
		content = after
	}
	if before, found := strings.CutSuffix(content, "```"); found {
		content = before
	}
	return strings.TrimSpace(content)
}
