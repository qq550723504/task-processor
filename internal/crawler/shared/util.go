// Package shared 提供爬虫共享工具函数
package shared

import "encoding/json"

// ProductToMap 将任意产品结构体序列化为 map，供 CrawlerResult.ProductData 使用。
// 若序列化失败（通常不会发生）则返回 nil。
func ProductToMap(product any) map[string]any {
	if product == nil {
		return nil
	}
	data, err := json.Marshal(product)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}
