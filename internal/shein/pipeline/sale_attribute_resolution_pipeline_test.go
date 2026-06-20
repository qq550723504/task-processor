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
	"task-processor/internal/state"
	"task-processor/internal/taskstatus"

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

func newSheinPipelineTestProcessor(cfg *config.Config, productFetcher fetcher.ProductFetcher) *SheinProcessor {
	clientMgr := management.NewClientManager(&cfg.Management)
	mem := state.NewMemoryManager(context.Background(), clientMgr)
	mem.ShopPauseManager.SetStoreClient(clientMgr.GetStoreClient())
	base := processor.NewBaseProcessorWithMemoryManager(&processor.BaseProcessorConfig{
		Config:   cfg,
		Logger:   logrus.New(),
		Platform: "SHEIN",
	}, mem)
	return &SheinProcessor{
		BaseProcessor:     base,
		managementClient:  clientMgr,
		taskStatusRuntime: taskstatus.NewManagementRuntime(clientMgr),
		imageDownloader:   clientMgr.GetImageDownloader(),
		productFetcher:    productFetcher,
	}
}

func TestCreateTaskProcessingPipelineInsertsSaleAttributeResolutionBeforeBuildSkcList(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	processor := newSheinPipelineTestProcessor(cfg, stubPipelineProductFetcher{})

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
