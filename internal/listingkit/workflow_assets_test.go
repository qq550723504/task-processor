package listingkit

import (
	"context"
	"fmt"
	"testing"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsdesign "task-processor/internal/sds/design"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

type stubWorkflowProductService struct {
	task    *productenrich.Task
	product *productenrich.ProductJSON
	lastReq *productenrich.GenerateRequest
}

func (s *stubWorkflowProductService) CreateGenerateTask(ctx context.Context, req *productenrich.GenerateRequest) (*productenrich.Task, error) {
	s.lastReq = req
	return s.task, nil
}

func (s *stubWorkflowProductService) GetTaskResult(ctx context.Context, taskID string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (s *stubWorkflowProductService) ProcessProduct(ctx context.Context, task *productenrich.Task) (*productenrich.ProductJSON, error) {
	return s.product, nil
}

type stubWorkflowImageService struct {
	task    *productimage.Task
	result  *productimage.ImageProcessResult
	lastReq *productimage.ImageProcessRequest
}

type stubWorkflowSDSSyncService struct {
	result    *sdsadapter.SyncResult
	err       error
	lastInput sdsusecase.ImageResultInput
}

func (s *stubWorkflowImageService) CreateProcessTask(ctx context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error) {
	s.lastReq = req
	return s.task, nil
}

func (s *stubWorkflowImageService) GetTaskResult(ctx context.Context, taskID string) (*productimage.TaskResult, error) {
	return nil, nil
}

func (s *stubWorkflowImageService) ProcessImages(ctx context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error) {
	return s.result, nil
}

func (s *stubWorkflowSDSSyncService) SyncFromRemoteImage(ctx context.Context, input sdsusecase.RemoteImageInput) (*sdsworkflow.SyncResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubWorkflowSDSSyncService) SyncFromLocalFile(ctx context.Context, input sdsusecase.LocalFileInput) (*sdsworkflow.SyncResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubWorkflowSDSSyncService) SyncFromImageResult(ctx context.Context, input sdsusecase.ImageResultInput) (*sdsadapter.SyncResult, error) {
	s.lastInput = input
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &sdsadapter.SyncResult{}, nil
}

func (s *stubWorkflowSDSSyncService) SyncFromImageRequest(ctx context.Context, input sdsusecase.ImageRequestInput) (*sdsadapter.SyncResult, error) {
	return nil, fmt.Errorf("not implemented")
}

type stubWorkflowSceneRenderer struct{}

func (s *stubWorkflowSceneRenderer) Render(ctx context.Context, asset *productimage.ImageAsset, context *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	return []productimage.ImageAsset{
		{
			URL:       "file:///tmp/rendered-scene.jpg",
			Type:      productimage.AssetTypeGalleryImage,
			SourceURL: asset.SourceURL,
			Operations: []string{
				"render_scene_stub",
			},
			Metadata: map[string]string{
				"renderer": "workflow-scene",
			},
		},
	}, nil
}

func TestRunWorkflowPersistsAssetInventoryAndBuildsPlatformBundles(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-1",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/source-1.jpg"},
			Text:      "wireless earbuds",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:       "Wireless Earbuds",
			Description: "Noise cancelling earbuds",
			Category:    []string{"Electronics", "Headphones"},
			Images:      []string{"https://example.com/source-1.jpg"},
			Attributes:  map[string]string{"brand": "DemoBrand"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-1"},
		result: &productimage.ImageProcessResult{
			MainImage:     &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg", SourceURL: "https://example.com/source-1.jpg"},
			WhiteBgImage:  &productimage.ImageAsset{URL: "https://cdn.example.com/white.jpg"},
			GalleryImages: []productimage.ImageAsset{{URL: "https://cdn.example.com/gallery.jpg"}},
		},
	}
	assetRepository := assetrepo.NewMemRepository()

	svc := &service{
		productSvc:          productSvc,
		imageSvc:            imageSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetRepository,
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      newDefaultAssetGenerationService(),
	}

	task := &Task{
		ID: "listingkit-task-1",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-1.jpg"},
			Text:      "wireless earbuds",
			Platforms: []string{"amazon", "shein", "temu", "walmart"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: true},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if result.AssetInventorySummary == nil {
		t.Fatal("expected asset inventory summary")
	}
	if result.AssetInventorySummary.TotalRecords == 0 {
		t.Fatalf("asset inventory summary = %+v", result.AssetInventorySummary)
	}
	if result.AssetInventorySummary.GeneratedRecords == 0 {
		t.Fatalf("asset inventory summary = %+v, want generated records", result.AssetInventorySummary)
	}
	if result.Amazon == nil || result.Amazon.ImageBundle == nil {
		t.Fatalf("amazon image bundle = %+v", result.Amazon)
	}
	if result.Shein == nil || result.Shein.ImageBundle == nil {
		t.Fatalf("shein image bundle = %+v", result.Shein)
	}
	if result.Shein.ImageBundle.Main == nil {
		t.Fatalf("shein image bundle = %+v, want generated main slot", result.Shein.ImageBundle)
	}
	if len(result.Shein.ImageBundle.PendingGeneration) != 0 {
		t.Fatalf("shein pending generation = %+v, want completed deferred tasks", result.Shein.ImageBundle)
	}
	if result.Temu == nil || result.Temu.ImageBundle == nil {
		t.Fatalf("temu image bundle = %+v", result.Temu)
	}
	if result.Walmart == nil || result.Walmart.ImageBundle == nil {
		t.Fatalf("walmart image bundle = %+v", result.Walmart)
	}
	if imageSvc.lastReq == nil || imageSvc.lastReq.Marketplace != "amazon" {
		t.Fatalf("image request = %+v, want platform-specific marketplace", imageSvc.lastReq)
	}

	inventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: "listingkit-task-1"})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	if inventory == nil || len(inventory.Records) == 0 {
		t.Fatalf("inventory = %+v, want persisted records", inventory)
	}
	if !hasInventoryKind(inventory, asset.KindCleanImage) {
		t.Fatalf("inventory = %+v, want clean_image asset", inventory)
	}
	generationTasks, err := assetRepository.ListGenerationTasks(context.Background(), "listingkit-task-1")
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if len(generationTasks) == 0 {
		t.Fatal("expected persisted generation tasks")
	}
	if generationTasks[0].Platform == "" || generationTasks[0].ExecutionStatus == "" {
		t.Fatalf("generation task = %+v, want platform and execution status", generationTasks[0])
	}
	hasCompletedDeferredTask := false
	for _, item := range generationTasks {
		if item.ExecutionMode == "deferred_stub" && item.ExecutionStatus == "completed" {
			hasCompletedDeferredTask = true
			break
		}
	}
	if !hasCompletedDeferredTask {
		t.Fatalf("generation tasks = %+v, want completed deferred generation task", generationTasks)
	}
}

func TestRunWorkflowSkipsAssetGenerationWhenProcessImagesDisabled(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-2",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/source-2.jpg"},
			Text:      "travel bottle",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:       "Travel Bottle",
			Description: "Portable insulated bottle",
			Category:    []string{"Home", "Kitchen"},
			Images:      []string{"https://example.com/source-2.jpg"},
			Attributes:  map[string]string{"brand": "DemoBrand"},
		},
	}
	assetRepository := assetrepo.NewMemRepository()

	svc := &service{
		productSvc:          productSvc,
		imageSvc:            nil,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetRepository,
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      newDefaultAssetGenerationService(),
	}

	task := &Task{
		ID: "listingkit-task-2",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-2.jpg"},
			Text:      "travel bottle",
			Platforms: []string{"amazon", "shein"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: false},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if result.AssetInventorySummary == nil || result.AssetInventorySummary.GeneratedRecords != 0 {
		t.Fatalf("asset inventory summary = %+v, want no generated records", result.AssetInventorySummary)
	}
	inventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: "listingkit-task-2"})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	if inventory == nil || len(inventory.Records) == 0 {
		t.Fatalf("inventory = %+v, want persisted source inventory", inventory)
	}
	if hasInventoryKind(inventory, asset.KindCleanImage) || hasInventoryKind(inventory, asset.KindWhiteBgImage) || hasInventoryKind(inventory, asset.KindSubjectCutout) {
		t.Fatalf("inventory = %+v, want no derived assets when process_images=false", inventory)
	}
	if result.Amazon != nil && result.Amazon.ImageBundle != nil {
		t.Fatalf("amazon image bundle = %+v, want no generated image bundle", result.Amazon)
	}
}

func TestRunWorkflowSyncsSDSDesignWhenConfigured(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-sds",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/source-sds.jpg"},
			Text:      "sports bottle",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:      "Sports Bottle",
			Category:   []string{"Sports", "Drinkware"},
			Images:     []string{"https://example.com/source-sds.jpg"},
			Attributes: map[string]string{"brand": "DemoBrand"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-sds"},
		result: &productimage.ImageProcessResult{
			WhiteBgImage: &productimage.ImageAsset{URL: "https://cdn.example.com/white-sds.jpg"},
		},
	}
	sdsSvc := &stubWorkflowSDSSyncService{
		result: &sdsadapter.SyncResult{
			DesignSync: &sdsworkflow.SyncResult{
				DesignResult: &sdsdesign.PrepareSyncDesignResult{
					Page: &sdsdesign.DesignProductPage{
						Product: sdsdesign.DesignProduct{ID: 89764},
					},
					Request: &sdsdesign.SyncDesignRequest{
						PrototypeGroupID: 14555,
						Prototypes: []sdsdesign.SyncDesignPrototype{
							{
								Layers: []sdsdesign.SyncDesignLayer{
									{LayerID: "layer-1"},
								},
							},
						},
					},
					Material: &sdsdesign.UploadedMaterial{
						Material: &sdsdesign.Material{ID: 396548287},
					},
					RenderedImageURLs: []string{"https://cdn.example.com/rendered-sds.jpg"},
				},
			},
		},
	}

	svc := &service{
		productSvc:          productSvc,
		imageSvc:            imageSvc,
		sdsSyncSvc:          sdsSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetrepo.NewMemRepository(),
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      newDefaultAssetGenerationService(),
	}

	task := &Task{
		ID: "listingkit-task-sds",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-sds.jpg"},
			Text:      "sports bottle",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options: &GenerateOptions{
				ProcessImages: true,
				SDS: &SDSSyncOptions{
					VariantID: 89764,
				},
			},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if result.SDSSync == nil || result.SDSSync.Status != "completed" {
		t.Fatalf("sds sync = %+v", result.SDSSync)
	}
	if result.SDSSync.MaterialID != 396548287 {
		t.Fatalf("sds sync = %+v, want material id", result.SDSSync)
	}
	if !hasChildTaskStatus(result.ChildTasks, "sds_design_sync", string(TaskStatusCompleted)) {
		t.Fatalf("child tasks = %+v, want completed sds_design_sync", result.ChildTasks)
	}
	if sdsSvc.lastInput.Sync.VariantID != 89764 {
		t.Fatalf("sds input = %+v", sdsSvc.lastInput.Sync)
	}
}

func TestRunWorkflowKeepsMainFlowWhenSDSSyncFails(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-sds-fail",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/source-sds-fail.jpg"},
			Text:      "sports towel",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:      "Sports Towel",
			Category:   []string{"Sports"},
			Images:     []string{"https://example.com/source-sds-fail.jpg"},
			Attributes: map[string]string{"brand": "DemoBrand"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-sds-fail"},
		result: &productimage.ImageProcessResult{
			MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/main-sds-fail.jpg"},
		},
	}
	sdsSvc := &stubWorkflowSDSSyncService{
		err: fmt.Errorf("sync failed"),
	}

	svc := &service{
		productSvc:          productSvc,
		imageSvc:            imageSvc,
		sdsSyncSvc:          sdsSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetrepo.NewMemRepository(),
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      newDefaultAssetGenerationService(),
	}

	task := &Task{
		ID: "listingkit-task-sds-fail",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-sds-fail.jpg"},
			Text:      "sports towel",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options: &GenerateOptions{
				ProcessImages: true,
				SDS: &SDSSyncOptions{
					VariantID: 89765,
				},
			},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if result.SDSSync == nil || result.SDSSync.Status != "failed" {
		t.Fatalf("sds sync = %+v", result.SDSSync)
	}
	if len(result.Summary.Warnings) == 0 {
		t.Fatalf("warnings = %+v, want sds warning", result.Summary)
	}
	if !hasChildTaskStatus(result.ChildTasks, "sds_design_sync", string(TaskStatusFailed)) {
		t.Fatalf("child tasks = %+v, want failed sds_design_sync", result.ChildTasks)
	}
	if result.ImageAssets == nil {
		t.Fatal("expected main workflow to continue despite sds sync failure")
	}
}

func hasChildTaskStatus(tasks []ChildTaskState, kind string, status string) bool {
	for _, item := range tasks {
		if item.Kind == kind && item.Status == status {
			return true
		}
	}
	return false
}

func TestRunWorkflowSkipsDeferredGenerationWhenProcessImagesDisabled(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-3",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/source-3.jpg"},
			Text:      "portable speaker",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:       "Portable Speaker",
			Description: "Wireless speaker",
			Category:    []string{"Electronics", "Audio"},
			Images:      []string{"https://example.com/source-3.jpg"},
			Attributes:  map[string]string{"brand": "DemoBrand"},
		},
	}
	assetRepository := assetrepo.NewMemRepository()

	svc := &service{
		productSvc:          productSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetRepository,
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: assetgeneration.NewProductImageDeferredRenderer(&stubWorkflowSceneRenderer{}),
		}),
	}

	task := &Task{
		ID: "listingkit-task-3",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-3.jpg"},
			Text:      "portable speaker",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: false},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if result.Amazon != nil && result.Amazon.ImageBundle != nil {
		t.Fatalf("amazon image bundle = %+v, want no image bundle when process_images=false", result.Amazon)
	}
	generationTasks, err := assetRepository.ListGenerationTasks(context.Background(), "listingkit-task-3")
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if len(generationTasks) != 0 {
		t.Fatalf("generation tasks = %+v, want none when process_images=false", generationTasks)
	}
}

func hasInventoryKind(inventory *asset.Inventory, kind asset.Kind) bool {
	if inventory == nil {
		return false
	}
	for _, record := range inventory.Records {
		if record.Kind == kind {
			return true
		}
	}
	return false
}
