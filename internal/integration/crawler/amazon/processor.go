package amazon

import (
	"context"
	"fmt"
	"strings"

	legacyamazon "task-processor/internal/crawler/amazon"
	"task-processor/internal/model"
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

// NewProcessor creates an Amazon crawler integration adapter.
func NewProcessor(source Source) *Processor {
	if source == nil {
		return nil
	}
	return &Processor{source: source}
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

// ZipcodePolicy preserves legacy Amazon default-zipcode behavior at the crawler
// integration boundary.
type ZipcodePolicy struct{}

// ShouldUseDefaultZipcode reports whether a region should receive a default zipcode.
func (ZipcodePolicy) ShouldUseDefaultZipcode(region string) bool {
	return legacyamazon.NewDomainResolver().ShouldUseDefaultZipcode(region)
}

// DefaultZipcode returns the legacy Amazon default zipcode for a region.
func (ZipcodePolicy) DefaultZipcode(region string) string {
	return legacyamazon.GetDefaultZipcode(strings.ToLower(region))
}
