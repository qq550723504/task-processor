package sourcing

import (
	"context"
	"fmt"

	"task-processor/internal/model"
)

// AmazonCrawlerSource is the minimal source capability needed to fetch one
// Amazon product.
type AmazonCrawlerSource interface {
	ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error)
}

// AmazonBatchCrawlerSource is the optional batch capability for Amazon source fetches.
type AmazonBatchCrawlerSource interface {
	ProcessBatchWithContext(ctx context.Context, requests []model.ProductRequest) []model.ProductResult
}

// AmazonSourceFetcher plans and executes Amazon source product fetches.
type AmazonSourceFetcher struct {
	Planner AmazonCrawlRequestPlanner
	Source  AmazonCrawlerSource
}

// Configured reports whether the fetcher has an executable source.
func (f AmazonSourceFetcher) Configured() bool {
	return f.Source != nil
}

// Fetch plans a crawler request and delegates execution to the source adapter.
func (f AmazonSourceFetcher) Fetch(ctx context.Context, input AmazonCrawlRequestInput) (*model.Product, error) {
	if f.Source == nil {
		return nil, fmt.Errorf("amazon crawler source is not configured")
	}
	req, err := f.Planner.BuildRequest(input)
	if err != nil {
		return nil, err
	}
	return f.Source.ProcessWithContext(ctx, req.URL, req.Zipcode)
}

// FetchBatch plans crawler requests and delegates batch execution to the source
// when available.
func (f AmazonSourceFetcher) FetchBatch(ctx context.Context, input AmazonCrawlRequestInput, productIDs []string) ([]model.ProductResult, error) {
	requests, err := f.Planner.BuildBatchRequests(input, productIDs)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return []model.ProductResult{}, nil
	}
	if f.Source == nil {
		return nil, fmt.Errorf("amazon crawler source is not configured")
	}
	batchSource, ok := f.Source.(AmazonBatchCrawlerSource)
	if !ok || batchSource == nil {
		return f.fetchBatchSequentially(ctx, requests), nil
	}
	return batchSource.ProcessBatchWithContext(ctx, requests), nil
}

func (f AmazonSourceFetcher) fetchBatchSequentially(ctx context.Context, requests []model.ProductRequest) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))
	for i, req := range requests {
		product, err := f.Source.ProcessWithContext(ctx, req.URL, req.Zipcode)
		results[i] = model.ProductResult{Product: product, Error: err}
	}
	return results
}
