package pipeline

import (
	"context"
	"testing"

	"task-processor/internal/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/model"
	"task-processor/internal/processor"
	domainproduct "task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type stubPipelineProductFetcher struct{}

func (stubPipelineProductFetcher) FetchProduct(context.Context, *domainproduct.FetchRequest) (*model.Product, error) {
	return nil, nil
}

func (stubPipelineProductFetcher) FetchVariants(context.Context, *domainproduct.FetchRequest, []string) ([]*model.Product, error) {
	return nil, nil
}

func (stubPipelineProductFetcher) CacheProduct(*domainproduct.FetchRequest, *model.Product) error {
	return nil
}

func (stubPipelineProductFetcher) CacheVariants(*domainproduct.FetchRequest, []*model.Product) error {
	return nil
}

func (stubPipelineProductFetcher) GetStats() map[string]any {
	return nil
}

var _ fetcher.ProductFetcher = (*stubPipelineProductFetcher)(nil)

func TestCreateTaskProcessingPipelineInsertsSaleAttributeResolutionBeforeBuildSkcList(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	base := processor.NewBaseProcessor(context.Background(), &processor.BaseProcessorConfig{
		Config:           cfg,
		ManagementClient: management.NewClientManager(&cfg.Management),
		Logger:           logrus.New(),
		Platform:         "SHEIN",
	})
	processor := &SheinProcessor{
		BaseProcessor:  base,
		productFetcher: stubPipelineProductFetcher{},
	}

	p := CreateTaskProcessingPipeline(processor, cfg)
	handlers := p.Handlers()

	saleAttributeIdx := -1
	resolutionIdx := -1
	buildSkcIdx := -1
	for idx, handler := range handlers {
		switch handler.Name() {
		case "sale_attribute":
			saleAttributeIdx = idx
		case "sale_attribute_resolution":
			resolutionIdx = idx
		case "build_skc_list":
			buildSkcIdx = idx
		}
	}

	if saleAttributeIdx < 0 {
		t.Fatal("sale_attribute handler not found")
	}
	if resolutionIdx < 0 {
		t.Fatal("sale_attribute_resolution handler not found")
	}
	if buildSkcIdx < 0 {
		t.Fatal("build_skc_list handler not found")
	}
	if !(saleAttributeIdx < resolutionIdx && resolutionIdx < buildSkcIdx) {
		t.Fatalf("handler order = sale_attribute:%d resolution:%d build_skc_list:%d, want sale_attribute < resolution < build_skc_list", saleAttributeIdx, resolutionIdx, buildSkcIdx)
	}
}
