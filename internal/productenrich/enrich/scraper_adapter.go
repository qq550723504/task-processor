package enrich

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/productenrich"
)

type scraper1688 struct {
	processor *alibaba1688.Alibaba1688Processor
}

func NewCrawler1688Adapter(cfg *config.Config) productenrich.WebScraper {
	return &scraper1688{
		processor: alibaba1688.NewAlibaba1688Processor(cfg),
	}
}

func (s *scraper1688) Scrape(_ context.Context, url string) (*productenrich.ScrapedData, error) {
	product, err := s.processor.Process(url)
	if err != nil {
		return nil, fmt.Errorf("1688 scrape failed: %w", err)
	}

	specs := make(map[string]string, len(product.Specifications))
	for _, sp := range product.Specifications {
		specs[sp.Name] = sp.Value
	}

	return &productenrich.ScrapedData{
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
