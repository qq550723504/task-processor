// Package productcrawler 提供基于 1688 爬虫的 WebScraper 适配器。
// 将 Alibaba1688Processor 适配为 productenrich.WebScraper 接口，
// 遵循"消费者定义接口，infra 层实现"原则。
package productcrawler

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
)

// ScrapedData 是 productenrich.ScrapedData 的镜像类型，
// 避免 infra 层直接依赖 productenrich 包（防止循环依赖）。
type ScrapedData struct {
	Title       string
	Description string
	Images      []string
	Price       float64
	Specs       map[string]string
}

// WebScraper 是 productenrich.WebScraper 的本地定义，
// 供 Scraper1688 实现，由 cmd 层做类型断言或直接赋值给 productenrich.WebScraper。
type WebScraper interface {
	Scrape(ctx context.Context, url string) (*ScrapedData, error)
}

// Scraper1688 将 Alibaba1688Processor 适配为 WebScraper 接口。
type Scraper1688 struct {
	processor *alibaba1688.Alibaba1688Processor
}

// NewScraper1688 创建基于 1688 爬虫的 WebScraper。
func NewScraper1688(cfg *config.Config) *Scraper1688 {
	return &Scraper1688{
		processor: alibaba1688.NewAlibaba1688Processor(cfg),
	}
}

// Scrape 实现 WebScraper.Scrape。
func (s *Scraper1688) Scrape(_ context.Context, url string) (*ScrapedData, error) {
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
