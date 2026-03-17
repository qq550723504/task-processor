// Package amazon 提供 Amazon 商品爬虫核心实现。
package amazon

import "task-processor/internal/model"

// Scraper 定义 Amazon 商品抓取能力（消费者定义接口原则）。
// 任何需要抓取 Amazon 商品数据的包应依赖此接口，而非 *AmazonProcessor 具体类型。
type Scraper interface {
	Process(url string, zipcode string) (*model.Product, error)
	ProcessBatch(requests []model.ProductRequest) []model.ProductResult
}

// 编译期验证 AmazonProcessor 实现了 Scraper 接口。
var _ Scraper = (*AmazonProcessor)(nil)

