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
	sdsclient "task-processor/internal/sds/client"
	sdsdesign "task-processor/internal/sds/design"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

type stubWorkflowProductService struct {
	task       *productenrich.Task
	product    *productenrich.ProductJSON
	processErr error
	lastReq    *productenrich.GenerateRequest
}

func (s *stubWorkflowProductService) CreateGenerateTask(ctx context.Context, req *productenrich.GenerateRequest) (*productenrich.Task, error) {
	s.lastReq = req
	return s.task, nil
}

func (s *stubWorkflowProductService) GetTaskResult(ctx context.Context, taskID string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (s *stubWorkflowProductService) ProcessProduct(ctx context.Context, task *productenrich.Task) (*productenrich.ProductJSON, error) {
	if s.processErr != nil {
		return nil, s.processErr
	}
	return s.product, nil
}

type stubWorkflowAssetGenerator struct {
	planResult     *assetgeneration.Result
	executeErr     error
	planErr        error
	dispatchErr    error
	dispatchResult *assetgeneration.Result
	dispatchCalls  int
	dispatchErrAt  map[int]error
}

func (s *stubWorkflowAssetGenerator) Plan(ctx context.Context, req assetgeneration.Request) (*assetgeneration.Result, error) {
	if s.planErr != nil {
		return nil, s.planErr
	}
	if s.planResult != nil {
		return s.planResult, nil
	}
	return &assetgeneration.Result{}, nil
}

func (s *stubWorkflowAssetGenerator) Execute(ctx context.Context, req assetgeneration.Request) (*assetgeneration.Result, error) {
	if s.executeErr != nil {
		return nil, s.executeErr
	}
	return &assetgeneration.Result{}, nil
}

func (s *stubWorkflowAssetGenerator) Dispatch(ctx context.Context, req assetgeneration.DispatchRequest) (*assetgeneration.Result, error) {
	s.dispatchCalls++
	if s.dispatchErrAt != nil {
		if err := s.dispatchErrAt[s.dispatchCalls]; err != nil {
			return nil, err
		}
	}
	if s.dispatchErr != nil {
		return nil, s.dispatchErr
	}
	if s.dispatchResult != nil {
		return s.dispatchResult, nil
	}
	return &assetgeneration.Result{Tasks: req.Tasks}, nil
}

type stubWorkflowImageService struct {
	task       *productimage.Task
	result     *productimage.ImageProcessResult
	createErr  error
	processErr error
	lastReq    *productimage.ImageProcessRequest
}

type stubWorkflowSDSSyncService struct {
	result           *sdsadapter.SyncResult
	remoteResult     *sdsworkflow.SyncResult
	remoteResults    []*sdsworkflow.SyncResult
	err              error
	remoteErr        error
	lastInput        sdsusecase.ImageResultInput
	lastRemoteInput  sdsusecase.RemoteImageInput
	lastRemoteInputs []sdsusecase.RemoteImageInput
	remoteCalls      int
}

func (s *stubWorkflowImageService) CreateProcessTask(ctx context.Context, req *productimage.ImageProcessRequest) (*productimage.Task, error) {
	s.lastReq = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	return s.task, nil
}

func (s *stubWorkflowImageService) GetTaskResult(ctx context.Context, taskID string) (*productimage.TaskResult, error) {
	return nil, nil
}

func (s *stubWorkflowImageService) ProcessImages(ctx context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error) {
	if s.processErr != nil {
		return nil, s.processErr
	}
	return s.result, nil
}

func (s *stubWorkflowSDSSyncService) SyncFromRemoteImage(ctx context.Context, input sdsusecase.RemoteImageInput) (*sdsworkflow.SyncResult, error) {
	s.remoteCalls++
	s.lastRemoteInput = input
	s.lastRemoteInputs = append(s.lastRemoteInputs, input)
	if s.remoteErr != nil {
		return nil, s.remoteErr
	}
	if len(s.remoteResults) > 0 {
		index := s.remoteCalls - 1
		if index < len(s.remoteResults) {
			return s.remoteResults[index], nil
		}
	}
	if s.remoteResult != nil {
		return s.remoteResult, nil
	}
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
	if result.StandardProductSnapshot == nil {
		t.Fatal("expected persisted standard product snapshot")
	}
	if result.StandardProductSnapshot.CanonicalProduct == nil || result.StandardProductSnapshot.CatalogProduct == nil {
		t.Fatalf("standard snapshot = %+v, want canonical/catalog product", result.StandardProductSnapshot)
	}
	if result.StandardProductSnapshot.AssetInventorySummary == nil || result.StandardProductSnapshot.AssetInventorySummary.TotalRecords == 0 {
		t.Fatalf("standard snapshot inventory = %+v, want persisted inventory summary", result.StandardProductSnapshot)
	}
	if result.StandardProductSnapshot.ImageAssets == nil {
		t.Fatalf("standard snapshot image assets = %+v, want copied image stage output", result.StandardProductSnapshot)
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

func TestRunWorkflowRecordsDegradedImageStageWhenImageProcessingFails(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-image-fail",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/source-image-fail.jpg"},
			Text:      "canvas tote",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:      "Canvas Tote",
			Category:   []string{"Bags"},
			Images:     []string{"https://example.com/source-image-fail.jpg"},
			Attributes: map[string]string{"material": "canvas"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task:       &productimage.Task{ID: "image-task-fail"},
		processErr: fmt.Errorf("renderer unavailable"),
	}
	svc := &service{
		productSvc:          productSvc,
		imageSvc:            imageSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetrepo.NewMemRepository(),
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      newDefaultAssetGenerationService(),
	}

	task := &Task{
		ID: "listingkit-task-image-fail",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-image-fail.jpg"},
			Text:      "canvas tote",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: true},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if !hasWorkflowStageStatus(result.WorkflowStages, "product_image", WorkflowStageStatusDegraded) {
		t.Fatalf("workflow stages = %+v, want degraded product_image", result.WorkflowStages)
	}
	if !hasWorkflowIssue(result.WorkflowIssues, "product_image", WorkflowIssueSeverityWarning, "image_processing_failed") {
		t.Fatalf("workflow issues = %+v, want image processing warning", result.WorkflowIssues)
	}
	if result.Summary == nil || result.Summary.WarningCount == 0 || result.Summary.IssueCount == 0 {
		t.Fatalf("summary = %+v, want warning counts", result.Summary)
	}
	if result.Amazon == nil {
		t.Fatal("expected assembler result despite image processing failure")
	}
}

func TestRunWorkflowUsesSDSCatalogCanonicalAndSkipsImageProcessing(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductService{
		task: &productenrich.Task{ID: "unexpected-product-task"},
		product: &productenrich.ProductJSON{
			Title: "Unexpected enrich result",
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-sds-catalog"},
		result: &productimage.ImageProcessResult{
			MainImage: &productimage.ImageAsset{
				URL:       "https://cdn.example.com/sds-main.jpg",
				SourceURL: "https://cdn.example.com/sds-source.jpg",
			},
		},
	}
	svc := &service{
		productSvc: productSvc,
		imageSvc:   imageSvc,
		assembler:  NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}

	task := &Task{
		ID: "listingkit-task-sds-catalog",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/sds-source.jpg"},
			Text:      "SDS supplied title",
			Platforms: []string{"amazon"},
			Options: &GenerateOptions{
				ProcessImages: true,
				SDS: &SDSSyncOptions{
					VariantID:       212095,
					ParentProductID: 212094,
					ProductName:     "SDS Clock",
					ProductSKU:      "MG17701061",
					VariantSKU:      "MG17701061001",
					CategoryPath:    []string{"Home", "Decor", "Clock"},
				},
			},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if productSvc.lastReq != nil {
		t.Fatalf("product enrich request = %+v, want skipped for SDS catalog task", productSvc.lastReq)
	}
	if result.CanonicalProduct == nil || result.CanonicalProduct.Title != "SDS Clock" {
		t.Fatalf("canonical product = %+v, want SDS catalog title", result.CanonicalProduct)
	}
	if !hasWorkflowStageStatus(result.WorkflowStages, "sds_catalog_product", WorkflowStageStatusCompleted) {
		t.Fatalf("workflow stages = %+v, want completed sds_catalog_product", result.WorkflowStages)
	}
	if hasWorkflowStageStatus(result.WorkflowStages, "product_enrich", WorkflowStageStatusCompleted) {
		t.Fatalf("workflow stages = %+v, product_enrich should not run", result.WorkflowStages)
	}
	if imageSvc.lastReq != nil {
		t.Fatalf("image processing request = %+v, want skipped for SDS catalog task", imageSvc.lastReq)
	}
	if hasWorkflowStageStatus(result.WorkflowStages, "product_image", WorkflowStageStatusCompleted) {
		t.Fatalf("workflow stages = %+v, product_image should not run", result.WorkflowStages)
	}
}

func TestRunWorkflowRecordsBlockingProductEnrichFailure(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductService{
		task:       &productenrich.Task{ID: "product-task-blocking", Request: &productenrich.GenerateRequest{Text: "unknown product"}},
		processErr: fmt.Errorf("llm unavailable"),
	}
	svc := &service{
		productSvc: productSvc,
		assembler:  NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}
	task := &Task{
		ID: "listingkit-task-product-blocking",
		Request: &GenerateRequest{
			Text:      "unknown product",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: false},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err == nil {
		t.Fatal("runWorkflow() error = nil, want product enrichment failure")
	}
	if !hasWorkflowStageStatus(result.WorkflowStages, "product_enrich", WorkflowStageStatusFailed) {
		t.Fatalf("workflow stages = %+v, want failed product_enrich", result.WorkflowStages)
	}
	if !hasWorkflowIssue(result.WorkflowIssues, "product_enrich", WorkflowIssueSeverityBlocking, "product_enrich_failed") {
		t.Fatalf("workflow issues = %+v, want blocking product enrich issue", result.WorkflowIssues)
	}
	if result.Summary == nil || result.Summary.BlockingCount != 1 {
		t.Fatalf("summary = %+v, want blocking count", result.Summary)
	}
}

func TestRunWorkflowRecordsAssetGenerationDispatchFailure(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductService{
		task: &productenrich.Task{ID: "product-task-asset-dispatch", Request: &productenrich.GenerateRequest{ImageURLs: []string{"https://example.com/source.jpg"}, Text: "poster"}},
		product: &productenrich.ProductJSON{
			Title:      "Poster",
			Category:   []string{"Home"},
			Images:     []string{"https://example.com/source.jpg"},
			Attributes: map[string]string{"material": "paper"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-asset-dispatch"},
		result: &productimage.ImageProcessResult{
			MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg"},
		},
	}
	assetGenerator := &stubWorkflowAssetGenerator{
		planResult: &assetgeneration.Result{
			Tasks: []assetgeneration.Task{{
				TaskID:          "asset-task-1",
				ID:              "asset-task-1",
				Platform:        "amazon",
				ExecutionStatus: "queued",
				CanExecute:      true,
			}},
		},
		dispatchErr: fmt.Errorf("queue unavailable"),
	}
	svc := &service{
		productSvc:          productSvc,
		imageSvc:            imageSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetrepo.NewMemRepository(),
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      assetGenerator,
	}
	task := &Task{
		ID: "listingkit-task-asset-dispatch",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source.jpg"},
			Text:      "poster",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: true},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if !hasWorkflowStageStatus(result.WorkflowStages, "asset_generation_platform", WorkflowStageStatusDegraded) {
		t.Fatalf("workflow stages = %+v, want degraded asset_generation_platform", result.WorkflowStages)
	}
	if !hasWorkflowIssue(result.WorkflowIssues, "asset_generation_platform", WorkflowIssueSeverityWarning, "asset_generation_platform_dispatch_failed") {
		t.Fatalf("workflow issues = %+v, want asset dispatch warning", result.WorkflowIssues)
	}
	if result.Summary == nil || result.Summary.WarningCount == 0 {
		t.Fatalf("summary = %+v, want warning count", result.Summary)
	}
}

func TestRunWorkflowRecordsDeferredAssetGenerationDispatchFailure(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductService{
		task: &productenrich.Task{ID: "product-task-deferred-asset-dispatch", Request: &productenrich.GenerateRequest{ImageURLs: []string{"https://example.com/source.jpg"}, Text: "poster"}},
		product: &productenrich.ProductJSON{
			Title:      "Poster",
			Category:   []string{"Home"},
			Images:     []string{"https://example.com/source.jpg"},
			Attributes: map[string]string{"material": "paper"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-deferred-asset-dispatch"},
		result: &productimage.ImageProcessResult{
			MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg"},
		},
	}
	assetGenerator := &stubWorkflowAssetGenerator{
		planResult: &assetgeneration.Result{
			Tasks: []assetgeneration.Task{{
				TaskID:          "asset-task-1",
				ID:              "asset-task-1",
				Platform:        "amazon",
				ExecutionStatus: "queued",
				CanExecute:      true,
			}},
		},
		dispatchErrAt: map[int]error{2: fmt.Errorf("renderer queue unavailable")},
	}
	svc := &service{
		productSvc:          productSvc,
		imageSvc:            imageSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetrepo.NewMemRepository(),
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      assetGenerator,
	}
	task := &Task{
		ID: "listingkit-task-deferred-asset-dispatch",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source.jpg"},
			Text:      "poster",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: true},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if assetGenerator.dispatchCalls != 2 {
		t.Fatalf("dispatch calls = %d, want 2", assetGenerator.dispatchCalls)
	}
	if !hasWorkflowStageStatus(result.WorkflowStages, "asset_generation_platform", WorkflowStageStatusDegraded) {
		t.Fatalf("workflow stages = %+v, want degraded asset_generation_platform", result.WorkflowStages)
	}
	if !hasWorkflowIssue(result.WorkflowIssues, "asset_generation_platform", WorkflowIssueSeverityWarning, "asset_generation_platform_deferred_dispatch_failed") {
		t.Fatalf("workflow issues = %+v, want deferred asset dispatch warning", result.WorkflowIssues)
	}
	if result.Summary == nil || result.Summary.WarningCount == 0 {
		t.Fatalf("summary = %+v, want warning count", result.Summary)
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

func TestSyncSDSDesignVariantsSubmitsEachRepresentativeVariantAsPrimary(t *testing.T) {
	t.Parallel()

	sdsSvc := &stubWorkflowSDSSyncService{
		remoteResults: []*sdsworkflow.SyncResult{
			{
				DesignResult: &sdsdesign.PrepareSyncDesignResult{
					Page: &sdsdesign.DesignProductPage{
						Product: sdsdesign.DesignProduct{ID: 101},
					},
					Request: &sdsdesign.SyncDesignRequest{
						PrototypeGroupID: 15506,
						Prototypes: []sdsdesign.SyncDesignPrototype{
							{Layers: []sdsdesign.SyncDesignLayer{{LayerID: "layer-101"}}},
						},
					},
					RenderedImageURLsByProduct: map[int64][]string{
						101: []string{"https://cdn.sdspod.com/out/101-main.jpg"},
					},
					RenderedImageURLs: []string{"https://cdn.sdspod.com/out/101-main.jpg"},
				},
			},
			{
				DesignResult: &sdsdesign.PrepareSyncDesignResult{
					Page: &sdsdesign.DesignProductPage{
						Product: sdsdesign.DesignProduct{ID: 102},
					},
					Request: &sdsdesign.SyncDesignRequest{
						PrototypeGroupID: 15506,
						Prototypes: []sdsdesign.SyncDesignPrototype{
							{Layers: []sdsdesign.SyncDesignLayer{{LayerID: "layer-102"}}},
						},
					},
					RenderedImageURLsByProduct: map[int64][]string{
						102: []string{"https://cdn.sdspod.com/out/102-main.jpg"},
					},
					RenderedImageURLs: []string{"https://cdn.sdspod.com/out/102-main.jpg"},
				},
			},
		},
	}
	svc := &service{sdsSyncSvc: sdsSvc}
	task := &Task{
		ID: "listingkit-task-variants",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/style.png"},
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					ParentProductID:  100,
					PrototypeGroupID: 15506,
					Variants: []SDSSyncVariantOption{
						{VariantID: 101, VariantSKU: "SKU-101", Color: "Black", LayerID: "layer-101"},
						{VariantID: 102, VariantSKU: "SKU-102", Color: "White", LayerID: "layer-102"},
					},
				},
			},
		},
	}
	result := &ListingKitResult{}

	svc.syncSDSDesignVariantsFromRemote(context.Background(), task, result, task.Request.ImageURLs[0], newWorkflowRecorder(result))

	if sdsSvc.remoteCalls != 2 {
		t.Fatalf("remote calls = %d, want one SDS sync request per representative variant", sdsSvc.remoteCalls)
	}
	if got := sdsSvc.lastRemoteInputs[0].Sync; got.VariantID != 101 || len(got.RelatedVariantIDs) != 0 {
		t.Fatalf("first sds input = %+v, want variant 101 without related variants", got)
	}
	if got := sdsSvc.lastRemoteInputs[1].Sync; got.VariantID != 102 || len(got.RelatedVariantIDs) != 0 {
		t.Fatalf("second sds input = %+v, want variant 102 without related variants", got)
	}
	if result.SDSSync == nil || result.SDSSync.Status != "completed" {
		t.Fatalf("sds sync = %+v, want completed when all selected variants render", result.SDSSync)
	}
	if len(result.SDSSync.VariantResults) != 2 {
		t.Fatalf("variant results = %+v, want 2", result.SDSSync.VariantResults)
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

func TestRunWorkflowMarksSDSAuthRequiredAsBlockingIssue(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductService{
		task: &productenrich.Task{ID: "product-task-sds-auth"},
		product: &productenrich.ProductJSON{
			Title:      "Sports Towel",
			Category:   []string{"Sports"},
			Images:     []string{"https://example.com/source-sds-auth.jpg"},
			Attributes: map[string]string{"brand": "DemoBrand"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-sds-auth"},
		result: &productimage.ImageProcessResult{
			MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/main-sds-auth.jpg"},
		},
	}
	sdsSvc := &stubWorkflowSDSSyncService{
		err: &sdsclient.AuthRequiredError{
			Op:         "POST /ps/design/add_and_design",
			StatusCode: 400,
			Message:    "用户未登录",
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
		ID: "listingkit-task-sds-auth",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-sds-auth.jpg"},
			Text:      "sports towel",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options: &GenerateOptions{
				ProcessImages: true,
				SDS: &SDSSyncOptions{
					VariantID: 89766,
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
	if !hasWorkflowIssue(result.WorkflowIssues, "sds_design_sync", WorkflowIssueSeverityBlocking, sdsAuthRequiredIssueCode) {
		t.Fatalf("workflow issues = %+v, want SDS auth blocking issue", result.WorkflowIssues)
	}
	if result.Summary == nil || result.Summary.BlockingCount != 1 || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want blocking needs review", result.Summary)
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

func hasWorkflowStageStatus(stages []WorkflowStage, kind string, status WorkflowStageStatus) bool {
	for _, item := range stages {
		if item.Kind == kind && item.Status == status {
			return true
		}
	}
	return false
}

func hasWorkflowIssue(issues []WorkflowIssue, stage string, severity WorkflowIssueSeverity, code string) bool {
	for _, item := range issues {
		if item.Stage == stage && item.Severity == severity && item.Code == code {
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
