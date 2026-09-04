package sourceproduct

import (
	"context"
	"errors"
	"reflect"

	"task-processor/internal/model"
)

// ErrFetchRequestRequired classifies calls that omit the marketplace product
// request value required by fetch and cache operations.
var ErrFetchRequestRequired = errors.New("product fetch request is required")

// ProductFetcherOptions is the marketplace-owned product fetch/cache policy
// snapshot needed by ProductFetcher.
type ProductFetcherOptions struct {
	Enabled           bool
	DataFreshnessDays int
}

// SourceFetchRequest contains only the source identity fields needed to fetch
// one marketplace product.
type SourceFetchRequest struct {
	Region    string
	ProductID string
	Zipcode   string
}

// SourceFetcher is the provider-neutral source capability consumed by the
// marketplace product fetch/cache orchestrator.
type SourceFetcher interface {
	Configured() bool
	Fetch(context.Context, SourceFetchRequest) (*model.Product, error)
}

func sourceFetcherConfigured(fetcher SourceFetcher) bool {
	if fetcher == nil {
		return false
	}
	value := reflect.ValueOf(fetcher)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return false
		}
	}
	return fetcher.Configured()
}
