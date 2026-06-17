package amazon

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	legacyamazon "task-processor/internal/crawler/amazon"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// Source is the legacy-compatible Amazon crawler capability used by this adapter.
type Source interface {
	ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error)
	ProcessBatchWithContext(ctx context.Context, requests []model.ProductRequest) []model.ProductResult
}

// Processor adapts an Amazon crawler source to the integration crawler boundary.
type Processor struct {
	source Source
}

// LegacyCrawlSource adapts the legacy Amazon crawler to app/product crawl-source
// contracts without exposing the legacy package outside this integration boundary.
type LegacyCrawlSource struct {
	source Source
}

// NewProcessor creates an Amazon crawler integration adapter.
func NewProcessor(source Source) *Processor {
	if source == nil {
		return nil
	}
	return &Processor{source: source}
}

// NewLegacySource constructs the legacy Amazon crawler behind the integration source interface.
func NewLegacySource(cfg *config.Config, logger *logrus.Logger) Source {
	return legacyamazon.CreateProcessor(cfg, logger)
}

// NewLegacyCrawlSource constructs a legacy-backed crawl source.
func NewLegacyCrawlSource(cfg *config.Config, logger *logrus.Logger) *LegacyCrawlSource {
	source := NewLegacySource(cfg, logger)
	if source == nil {
		return nil
	}
	return &LegacyCrawlSource{source: source}
}

// Process fetches one Amazon source product.
func (p *Processor) Process(ctx context.Context, req model.ProductRequest) (*model.Product, error) {
	if p == nil || p.source == nil {
		return nil, fmt.Errorf("amazon crawler source is not configured")
	}
	return p.source.ProcessWithContext(ctx, req.URL, req.Zipcode)
}

// ProcessWithContext fetches one Amazon source product using URL and zipcode inputs.
func (p *Processor) ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error) {
	return p.Process(ctx, model.ProductRequest{URL: url, Zipcode: zipcode})
}

// ProcessBatch fetches multiple Amazon source products.
func (p *Processor) ProcessBatch(ctx context.Context, requests []model.ProductRequest) []model.ProductResult {
	if p == nil || p.source == nil {
		results := make([]model.ProductResult, len(requests))
		for i := range requests {
			results[i] = model.ProductResult{Error: fmt.Errorf("amazon crawler source is not configured")}
		}
		return results
	}
	return p.source.ProcessBatchWithContext(ctx, requests)
}

// ProcessBatchWithContext fetches multiple Amazon source products.
func (p *Processor) ProcessBatchWithContext(ctx context.Context, requests []model.ProductRequest) []model.ProductResult {
	return p.ProcessBatch(ctx, requests)
}

// Process fetches one Amazon source product.
func (s *LegacyCrawlSource) Process(url, zipcode string) (*model.Product, error) {
	return s.ProcessWithContext(context.Background(), url, zipcode)
}

// ProcessWithContext fetches one Amazon source product with cancellation.
func (s *LegacyCrawlSource) ProcessWithContext(ctx context.Context, url, zipcode string) (*model.Product, error) {
	if s == nil || s.source == nil {
		return nil, fmt.Errorf("amazon crawler source is not configured")
	}
	return s.source.ProcessWithContext(ctx, url, zipcode)
}

// ProcessBatchWithContext fetches multiple Amazon source products with cancellation.
func (s *LegacyCrawlSource) ProcessBatchWithContext(ctx context.Context, requests []model.ProductRequest) []model.ProductResult {
	if s == nil || s.source == nil {
		results := make([]model.ProductResult, len(requests))
		for i := range requests {
			results[i] = model.ProductResult{Error: fmt.Errorf("amazon crawler source is not configured")}
		}
		return results
	}
	return s.source.ProcessBatchWithContext(ctx, requests)
}

// Shutdown releases the underlying legacy crawler if it supports shutdown.
func (s *LegacyCrawlSource) Shutdown() {
	if s == nil || s.source == nil {
		return
	}
	if shutdown, ok := s.source.(interface{ Shutdown() }); ok {
		shutdown.Shutdown()
	}
}
