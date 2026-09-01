package enrich

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	crawler1688 "task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/productenrich"
)

type scraper1688 struct {
	processor *crawler1688.Processor
}

func NewCrawler1688Adapter(cfg *config.Config) productenrich.WebScraper {
	return &scraper1688{
		processor: crawler1688.NewLegacyProcessor(cfg),
	}
}

func (s *scraper1688) Scrape(ctx context.Context, url string) (*productenrich.ScrapedData, error) {
	product, err := s.processor.Process(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("1688 scrape failed: %w", err)
	}

	return crawler1688.Convert1688ProductToScrapedData(crawler1688.SnapshotFromLegacyProduct(product)), nil
}
