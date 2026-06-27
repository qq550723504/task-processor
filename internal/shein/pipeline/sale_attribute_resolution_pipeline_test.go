package pipeline

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/fetcher"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
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

type stubPipelineRuntime struct{}

func (stubPipelineRuntime) RuntimePublishedProductExists(context.Context, int64, string, string, string) (bool, error) {
	return false, nil
}

func (stubPipelineRuntime) FindRuntimeProductImportMappingByTaskAndSKU(context.Context, int64, string) (*listingruntime.ProductImportMapping, error) {
	return nil, nil
}

func (stubPipelineRuntime) CreateRuntimeProductImportMapping(context.Context, *listingruntime.ProductImportMappingUpsert) (int64, error) {
	return 0, nil
}

func (stubPipelineRuntime) UpdateRuntimeProductImportMapping(context.Context, *listingruntime.ProductImportMappingUpsert) error {
	return nil
}

func (stubPipelineRuntime) GetRuntimeStorePauseStatusDetail(int64) (*listingruntime.StorePauseStatusDetail, error) {
	return nil, nil
}

func (stubPipelineRuntime) GetRuntimeStoreService() listingruntime.StoreService {
	return nil
}

func (stubPipelineRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	return nil
}

func (stubPipelineRuntime) GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	return nil
}

func (stubPipelineRuntime) GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	return nil
}

func (stubPipelineRuntime) GetSheinCookie(int64) (string, int64, error) {
	return "", 0, nil
}

func (stubPipelineRuntime) GetSheinStoreCookie(int64) (string, error) {
	return "", nil
}

func (stubPipelineRuntime) DeleteSheinStoreCookie(int64) (bool, error) {
	return false, nil
}

func (stubPipelineRuntime) SetRuntimeStorePauseStatus(int64, bool, string) (bool, error) {
	return false, nil
}

func (stubPipelineRuntime) UpdateRuntimeTaskStatus(*listingruntime.TaskStatusUpdate) error {
	return nil
}

type stubPipelineTaskStatusRuntime struct{}

func (stubPipelineTaskStatusRuntime) UpdateRuntimeTaskStatus(*listingruntime.TaskStatusUpdate) error {
	return nil
}

func (stubPipelineTaskStatusRuntime) GetTaskStatus(int64) (*taskstatus.TaskStatusSnapshot, error) {
	return nil, nil
}

type stubPipelineImageDownloader struct{}

func (stubPipelineImageDownloader) DownloadImage(string) ([]byte, error) {
	return nil, nil
}

func newSheinPipelineTestProcessor(cfg *config.Config, productFetcher fetcher.ProductFetcher) *SheinProcessor {
	runtime := stubPipelineRuntime{}
	mem := state.NewMemoryManager(context.Background(), nil)
	base := processor.NewBaseProcessorWithMemoryManager(&processor.BaseProcessorConfig{
		Config:   cfg,
		Logger:   logrus.New(),
		Platform: "SHEIN",
	}, mem)
	return &SheinProcessor{
		BaseProcessor:     base,
		runtimeRepository: runtime,
		taskStatusRuntime: stubPipelineTaskStatusRuntime{},
		imageDownloader:   stubPipelineImageDownloader{},
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
