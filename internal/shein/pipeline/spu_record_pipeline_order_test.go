package pipeline

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/model"
	"task-processor/internal/processor"
	domainproduct "task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

func TestCreateTaskProcessingPipelinePlacesSpuRecordAfterVariantFetch(t *testing.T) {
	cfg := &config.Config{}
	base := processor.NewBaseProcessor(context.Background(), &processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: management.NewClientManager(&cfg.Management),
		Logger:           logrus.New(),
		Platform:         "SHEIN",
	})
	processor := &SheinProcessor{
		BaseProcessor:  base,
		productFetcher: stubSpuRecordPipelineProductFetcher{},
	}

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

func (stubSpuRecordPipelineProductFetcher) FetchProduct(context.Context, *domainproduct.FetchRequest) (*model.Product, error) {
	return nil, nil
}

func (stubSpuRecordPipelineProductFetcher) FetchVariants(context.Context, *domainproduct.FetchRequest, []string) ([]*model.Product, error) {
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
