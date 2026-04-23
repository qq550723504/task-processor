package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
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
