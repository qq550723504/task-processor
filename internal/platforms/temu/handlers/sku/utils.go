// Package sku 提供TEMU平台的SKU工具方法
package sku

import "task-processor/internal/pkg/jsonutil"

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (sb *SkuBuilder) marshalWithoutHTMLEscape(v interface{}) ([]byte, error) {
	return jsonutil.MarshalIndentWithoutHTMLEscape(v, "", "  ")
}
