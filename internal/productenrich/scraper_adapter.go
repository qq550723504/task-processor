// Package productenrich 提供 1688 爬虫的 WebScraper 适配器。
// 将 alibaba1688.Alibaba1688Processor 适配为本包定义的 WebScraper 接口。
package productenrich

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
)

// scraper1688 将 Alibaba1688Processor 适配为 WebScraper 接口。
type scraper1688 struct {
	processor *alibaba1688.Alibaba1688Processor
}

// NewCrawler1688Adapter 创建基于 1688 爬虫的 WebScraper。
func NewCrawler1688Adapter(cfg *config.Config) WebScraper {
	return &scraper1688{
		processor: alibaba1688.NewAlibaba1688Processor(cfg),
	}
}

func (s *scraper1688) Scrape(_ context.Context, url string) (*ScrapedData, error) {
	product, err := s.processor.Process(url)
	if err != nil {
		return nil, fmt.Errorf("1688 scrape failed: %w", err)
	}

	specs := make(map[string]string, len(product.Specifications))
	for _, sp := range product.Specifications {
		specs[sp.Name] = sp.Value
	}

	return &ScrapedData{
		Title:       product.Title,
		Description: build1688Description(product),
		Images:      product.Images,
		Price:       product.MinPrice,
		Specs:       specs,
	}, nil
}

func build1688Description(product *alibaba1688model.Product1688) string {
	if len(product.ProductDetails) == 0 {
		return product.Title
	}
	var sb strings.Builder
	for _, d := range product.ProductDetails {
		if d.Content != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(d.Content)
		}
	}
	if sb.Len() == 0 {
		return product.Title
	}
	return sb.String()
}
