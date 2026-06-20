package pipeline

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/fetcher"
	"task-processor/internal/model"
	domainproduct "task-processor/internal/product"
)

func TestCreateTaskProcessingPipelinePlacesSpuRecordAfterVariantFetch(t *testing.T) {
	cfg := &config.Config{}
	processor := newSheinPipelineTestProcessor(cfg, stubSpuRecordPipelineProductFetcher{})

	p := CreateTaskProcessingPipeline(processor, cfg)
	handlers := p.Handlers()

	spuRecordIdx := -1
	variantFetchIdx := -1
	for i, handler := range handlers {
		switch handler.Name() {
		case "检查SPU发布记录":
			spuRecordIdx = i
		case "fetch_and_cache_variants":
			variantFetchIdx = i
		}
	}

	if spuRecordIdx == -1 || variantFetchIdx == -1 {
		t.Fatalf("handler indexes not found: spu_record=%d variant_fetch=%d", spuRecordIdx, variantFetchIdx)
	}
	if !(variantFetchIdx < spuRecordIdx) {
		t.Fatalf("handler order = variant_fetch:%d spu_record:%d, want variant_fetch < spu_record", variantFetchIdx, spuRecordIdx)
	}
}

type stubSpuRecordPipelineProductFetcher struct{}

func (stubSpuRecordPipelineProductFetcher) FetchProduct(_ context.Context, _ *domainproduct.FetchRequest) (*model.Product, error) {
	return nil, nil
}

func (stubSpuRecordPipelineProductFetcher) FetchVariants(_ context.Context, _ *domainproduct.FetchRequest, _ []string) ([]*model.Product, error) {
	return nil, nil
}

func (stubSpuRecordPipelineProductFetcher) CacheProduct(*domainproduct.FetchRequest, *model.Product) error {
	return nil
}

func (stubSpuRecordPipelineProductFetcher) CacheVariants(*domainproduct.FetchRequest, []*model.Product) error {
	return nil
}

func (stubSpuRecordPipelineProductFetcher) GetStats() map[string]any {
	return nil
}

var _ fetcher.ProductFetcher = (*stubSpuRecordPipelineProductFetcher)(nil)
