// Package sku 提供TEMU平台的SKU工具方法
package sku

import (
	"task-processor/internal/pkg/jsonx"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
)

// convertSpecInfos 将 temucontext.SpecInfo 切片转换为 models.SpecInfo 切片
func convertSpecInfos(src []temucontext.SpecInfo) []models.SpecInfo {
	result := make([]models.SpecInfo, len(src))
	for i, s := range src {
		result[i] = models.SpecInfo{
			SpecID:         s.SpecID,
			SpecName:       s.SpecName,
			ParentSpecID:   s.ParentSpecID,
			ParentSpecName: s.ParentSpecName,
			ParentID:       s.ParentID,
		}
	}
	return result
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (sb *SkuBuilder) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	return jsonx.MarshalIndentWithoutHTMLEscape(v, "", "  ")
}
