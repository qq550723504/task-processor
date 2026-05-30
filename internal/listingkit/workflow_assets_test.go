package listingkit

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsclient "task-processor/internal/sds/client"
	sdsdesign "task-processor/internal/sds/design"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
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
	planResult      *assetgeneration.Result
	executeErr      error
	planErr         error
	dispatchErr     error
	dispatchResult  *assetgeneration.Result
	dispatchCalls   int
	dispatchErrAt   map[int]error
	lastDispatchReq *assetgeneration.DispatchRequest
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
	clonedReq := req
	clonedReq.Tasks = cloneGenerationTasks(req.Tasks)
	s.lastDispatchReq = &clonedReq
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

type stubWorkflowAssetRepository struct {
	delegate               *assetrepo.MemRepository
	saveInventoryErr       error
	saveGenerationTasksErr error
	saveInventoryCalls     int
	savedTaskID            string
	savedTasks             []assetgeneration.Task
}

func newStubWorkflowAssetRepository() *stubWorkflowAssetRepository {
	return &stubWorkflowAssetRepository{delegate: assetrepo.NewMemRepository()}
}

func (s *stubWorkflowAssetRepository) SaveInventory(ctx context.Context, inventory *asset.Inventory) error {
	s.saveInventoryCalls++
	if s.saveInventoryErr != nil {
		return s.saveInventoryErr
	}
	return s.delegate.SaveInventory(ctx, inventory)
}

func (s *stubWorkflowAssetRepository) GetInventory(ctx context.Context, ref asset.InventoryRef) (*asset.Inventory, error) {
	return s.delegate.GetInventory(ctx, ref)
}

func (s *stubWorkflowAssetRepository) SaveGenerationTasks(ctx context.Context, taskID string, tasks []assetgeneration.Task) error {
	s.savedTaskID = taskID
	s.savedTasks = cloneGenerationTasks(tasks)
	if s.saveGenerationTasksErr != nil {
		return s.saveGenerationTasksErr
	}
	return s.delegate.SaveGenerationTasks(ctx, taskID, tasks)
}

func (s *stubWorkflowAssetRepository) ListGenerationTasks(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
	return s.delegate.ListGenerationTasks(ctx, taskID)
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

func TestApplyPlatformAssetDispatchMutationMergesDispatchArtifacts(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source-1.jpg"},
			},
		},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}
	dispatchResult := &assetgeneration.Result{
		Assets: []asset.AssetRecord{
			{
				ID:       "generated-1",
				Kind:     asset.KindSceneImage,
				Origin:   asset.OriginGenerated,
				URL:      "https://example.com/generated-1.jpg",
				RecipeID: "scene",
				Lineage:  &asset.AssetLineage{SourceAssetIDs: []string{"source-1"}},
			},
		},
		Tasks: []assetgeneration.Task{
			{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
			{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
		},
	}

	mutation := applyPlatformAssetDispatchMutation(
		final,
		inventory,
		nil,
		generationTasks,
		dispatchResult,
		newDefaultAssetBundleBuilder(),
	)

	if got := len(mutation.inventory.Records); got != 2 {
		t.Fatalf("inventory records = %d, want 2", got)
	}
	if mutation.final.AssetBundle == nil || len(mutation.final.AssetBundle.Assets) != 2 {
		t.Fatalf("asset bundle = %+v, want generated asset merged", mutation.final.AssetBundle)
	}
	if mutation.final.AssetInventorySummary == nil || mutation.final.AssetInventorySummary.GeneratedRecords != 1 {
		t.Fatalf("asset inventory summary = %+v, want generated record count updated", mutation.final.AssetInventorySummary)
	}

	wantTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
		{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
	}
	if !reflect.DeepEqual(mutation.generationTasks, wantTasks) {
		t.Fatalf("generation tasks = %+v, want %+v", mutation.generationTasks, wantTasks)
	}
}

func TestApplyPlatformAssetDispatchMutationKeepsGenerationTasksWhenDispatchResultNil(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		AssetBundle:           &asset.Bundle{},
		AssetInventorySummary: &asset.InventorySummary{},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned", Metadata: map[string]string{"k": "v"}},
	}

	mutation := applyPlatformAssetDispatchMutation(
		final,
		inventory,
		nil,
		generationTasks,
		nil,
		newDefaultAssetBundleBuilder(),
	)

	if !reflect.DeepEqual(mutation.generationTasks, generationTasks) {
		t.Fatalf("generation tasks = %+v, want unchanged %+v", mutation.generationTasks, generationTasks)
	}
	if got := len(mutation.inventory.Records); got != 1 {
		t.Fatalf("inventory records = %d, want unchanged count 1", got)
	}
}

func TestApplyPlatformAssetDispatchMutationShapesBundlesWhenDispatchReturnsTasksOnly(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		Amazon: &AmazonPackage{},
		Shein:  &SheinPackage{},
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source-1.jpg"},
			},
		},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"amazon", "shein"}, nil)
	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}
	dispatchResult := &assetgeneration.Result{
		Tasks: []assetgeneration.Task{
			{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
			{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
		},
	}

	mutation := applyPlatformAssetDispatchMutation(
		final,
		inventory,
		recipesByPlatform,
		generationTasks,
		dispatchResult,
		newDefaultAssetBundleBuilder(),
	)

	if got := len(mutation.inventory.Records); got != 1 {
		t.Fatalf("inventory records = %d, want unchanged count 1", got)
	}
	if mutation.final.AssetInventorySummary == nil || mutation.final.AssetInventorySummary.GeneratedRecords != 0 {
		t.Fatalf("asset inventory summary = %+v, want unchanged source-only summary", mutation.final.AssetInventorySummary)
	}
	if mutation.final.Shein == nil || mutation.final.Shein.ImageBundle == nil {
		t.Fatalf("shein image bundle = %+v, want shaped bundle from returned tasks", mutation.final.Shein)
	}
	hasSheinPending := false
	for _, pending := range mutation.final.Shein.ImageBundle.PendingGeneration {
		if pending.ID == "shein:gallery" && pending.ExecutionStatus == "planned" {
			hasSheinPending = true
			break
		}
	}
	if !hasSheinPending {
		t.Fatalf("shein pending generation = %+v, want planned returned task attached", mutation.final.Shein.ImageBundle.PendingGeneration)
	}

	wantTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
		{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
	}
	if !reflect.DeepEqual(mutation.generationTasks, wantTasks) {
		t.Fatalf("generation tasks = %+v, want %+v", mutation.generationTasks, wantTasks)
	}
}

func TestApplyPlatformAssetDispatchMutationShapesBundlesWhenDispatchReturnsAssetsOnly(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		Shein: &SheinPackage{},
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source-1.jpg"},
			},
		},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"shein"}, nil)
	generationTasks := []assetgeneration.Task{
		{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
	}
	dispatchResult := &assetgeneration.Result{
		Assets: []asset.AssetRecord{
			{
				ID:       "generated-1",
				Kind:     asset.KindSceneImage,
				Origin:   asset.OriginGenerated,
				URL:      "https://example.com/generated-1.jpg",
				RecipeID: "scene",
				Lineage:  &asset.AssetLineage{SourceAssetIDs: []string{"source-1"}},
			},
		},
	}

	mutation := applyPlatformAssetDispatchMutation(
		final,
		inventory,
		recipesByPlatform,
		generationTasks,
		dispatchResult,
		newDefaultAssetBundleBuilder(),
	)

	if got := len(mutation.inventory.Records); got != 2 {
		t.Fatalf("inventory records = %d, want 2", got)
	}
	if mutation.final.Shein == nil || mutation.final.Shein.ImageBundle == nil {
		t.Fatalf("shein image bundle = %+v, want reshaped bundle when assets return without tasks", mutation.final.Shein)
	}
	if mutation.final.Shein.ImageBundle.Main == nil {
		t.Fatalf("shein image bundle = %+v, want generated asset assigned after reshape", mutation.final.Shein.ImageBundle)
	}
	if !reflect.DeepEqual(mutation.generationTasks, generationTasks) {
		t.Fatalf("generation tasks = %+v, want unchanged %+v", mutation.generationTasks, generationTasks)
	}
}

func TestPlatformAssetDispatchInventoryApplyPhaseRunMergesReturnedAssets(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source-1.jpg"},
			},
		},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	dispatchAssets := []asset.AssetRecord{
		{
			ID:       "generated-1",
			Kind:     asset.KindSceneImage,
			Origin:   asset.OriginGenerated,
			URL:      "https://cdn.example.com/generated-1.jpg",
			RecipeID: "scene",
			Lineage:  &asset.AssetLineage{SourceAssetIDs: []string{"source-1"}},
		},
	}

	buildPlatformAssetDispatchInventoryApplyPhase().run(final, inventory, dispatchAssets)

	if got := len(inventory.Records); got != 2 {
		t.Fatalf("inventory records = %d, want 2", got)
	}
	if inventory.Summary == nil || inventory.Summary.TotalRecords != 2 || inventory.Summary.GeneratedRecords != 1 {
		t.Fatalf("inventory summary = %+v, want returned assets reflected", inventory.Summary)
	}
	if final.AssetBundle == nil || len(final.AssetBundle.Assets) != 2 {
		t.Fatalf("asset bundle = %+v, want generated asset merged", final.AssetBundle)
	}
	if final.AssetInventorySummary != inventory.Summary {
		t.Fatalf("asset inventory summary pointer = %+v, want refreshed summary %+v", final.AssetInventorySummary, inventory.Summary)
	}
}

func TestPlatformAssetDispatchInventoryApplyPhaseRunSkipsWhenNoReturnedAssets(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{
				{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source-1.jpg"},
			},
		},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}

	buildPlatformAssetDispatchInventoryApplyPhase().run(final, inventory, nil)

	if got := len(inventory.Records); got != 1 {
		t.Fatalf("inventory records = %d, want unchanged count 1", got)
	}
	if inventory.Summary == nil || inventory.Summary.TotalRecords != 1 || inventory.Summary.GeneratedRecords != 0 {
		t.Fatalf("inventory summary = %+v, want unchanged source-only summary", inventory.Summary)
	}
	if final.AssetBundle == nil || len(final.AssetBundle.Assets) != 1 {
		t.Fatalf("asset bundle = %+v, want unchanged bundle", final.AssetBundle)
	}
	if final.AssetInventorySummary == nil || final.AssetInventorySummary.TotalRecords != 1 || final.AssetInventorySummary.GeneratedRecords != 0 {
		t.Fatalf("asset inventory summary = %+v, want unchanged summary", final.AssetInventorySummary)
	}
}

func TestPlatformAssetDispatchBundleApplyPhaseRunReattachesBundlesAndMergesTasks(t *testing.T) {
	t.Parallel()

	final := &ListingKitResult{
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				PendingGeneration: []assetgeneration.Task{{
					ID:              "stale-task",
					Platform:        "amazon",
					RecipeID:        "stale",
					ExecutionStatus: "planned",
				}},
			},
		},
		Shein: &SheinPackage{},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"amazon", "shein"}, nil)
	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned"},
	}
	dispatchTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
		{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
	}

	gotTasks := buildPlatformAssetDispatchBundleApplyPhase(newDefaultAssetBundleBuilder()).run(
		final,
		inventory,
		recipesByPlatform,
		generationTasks,
		dispatchTasks,
	)

	if final.Amazon == nil || final.Amazon.ImageBundle == nil {
		t.Fatalf("amazon image bundle = %+v, want rebuilt bundle", final.Amazon)
	}
	for _, pending := range final.Amazon.ImageBundle.PendingGeneration {
		if pending.ID == "amazon:hero" {
			t.Fatalf("amazon pending generation = %+v, want completed dispatch task removed", final.Amazon.ImageBundle.PendingGeneration)
		}
	}
	if final.Shein == nil || final.Shein.ImageBundle == nil {
		t.Fatalf("shein image bundle = %+v, want rebuilt bundle", final.Shein)
	}
	hasSheinDispatchTask := false
	for _, pending := range final.Shein.ImageBundle.PendingGeneration {
		if pending.ID == "shein:gallery" && pending.ExecutionStatus == "planned" {
			hasSheinDispatchTask = true
			break
		}
	}
	if !hasSheinDispatchTask {
		t.Fatalf("shein pending generation = %+v, want planned dispatch task reattached", final.Shein.ImageBundle.PendingGeneration)
	}

	wantTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "completed"},
		{ID: "shein:gallery", Platform: "shein", RecipeID: "gallery", ExecutionStatus: "planned"},
	}
	if !reflect.DeepEqual(gotTasks, wantTasks) {
		t.Fatalf("generation tasks = %+v, want %+v", gotTasks, wantTasks)
	}
}

func TestPlatformAssetDispatchBundleApplyPhaseRunReattachesBundlesWhenNoReturnedTasks(t *testing.T) {
	t.Parallel()

	existingPending := []assetgeneration.Task{{
		ID:              "amazon:hero",
		Platform:        "amazon",
		RecipeID:        "hero",
		ExecutionStatus: "planned",
	}}
	final := &ListingKitResult{
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				PendingGeneration: cloneGenerationTasks(existingPending),
			},
		},
	}
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"},
		},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"amazon"}, nil)
	generationTasks := []assetgeneration.Task{
		{ID: "amazon:hero", Platform: "amazon", RecipeID: "hero", ExecutionStatus: "planned", Metadata: map[string]string{"k": "v"}},
	}

	gotTasks := buildPlatformAssetDispatchBundleApplyPhase(newDefaultAssetBundleBuilder()).run(
		final,
		inventory,
		recipesByPlatform,
		generationTasks,
		nil,
	)

	if !reflect.DeepEqual(gotTasks, generationTasks) {
		t.Fatalf("generation tasks = %+v, want unchanged %+v", gotTasks, generationTasks)
	}
	if final.Amazon == nil || final.Amazon.ImageBundle == nil {
		t.Fatalf("amazon image bundle = %+v, want rebuilt bundle even without returned tasks", final.Amazon)
	}
	for _, pending := range final.Amazon.ImageBundle.PendingGeneration {
		if pending.ID == "stale-task" {
			t.Fatalf("amazon pending generation = %+v, want stale pending task cleared during rebuild", final.Amazon.ImageBundle.PendingGeneration)
		}
	}
}

func TestPlatformAssetDispatchPersistPhaseRunDecoratesAndPersistsGenerationTasks(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	phase := buildPlatformAssetDispatchPersistPhase(&service{assetRepo: assetRepository})
	final := &ListingKitResult{
		Summary: &GenerationSummary{},
		Amazon: &AmazonPackage{
			ImageBundle: &common.PublishImageBundle{
				PendingGeneration: []assetgeneration.Task{{
					ID:              "stale-task",
					Platform:        "amazon",
					RecipeID:        "hero",
					ExecutionStatus: "planned",
				}},
			},
		},
	}
	tasks := []assetgeneration.Task{{
		ID:              "persisted-task",
		Platform:        "amazon",
		RecipeID:        "hero",
		ExecutionStatus: "completed",
		ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
		CanExecute:      true,
	}}

	got := phase.run(context.Background(), &Task{ID: "task-persist"}, final, tasks)

	if !reflect.DeepEqual(got, tasks) {
		t.Fatalf("returned generation tasks = %+v, want %+v", got, tasks)
	}
	if assetRepository.savedTaskID != "task-persist" {
		t.Fatalf("saved task id = %q, want task-persist", assetRepository.savedTaskID)
	}
	if !reflect.DeepEqual(assetRepository.savedTasks, tasks) {
		t.Fatalf("saved generation tasks = %+v, want %+v", assetRepository.savedTasks, tasks)
	}
	if !reflect.DeepEqual(final.AssetGenerationTasks, tasks) {
		t.Fatalf("decorated generation tasks = %+v, want %+v", final.AssetGenerationTasks, tasks)
	}
	if final.AssetGenerationSummary == nil || final.AssetGenerationSummary.CompletedTasks != 1 {
		t.Fatalf("generation summary = %+v, want completed task summary", final.AssetGenerationSummary)
	}
}

func TestPlatformAssetDispatchPersistPhaseRunAddsWarningIssueWhenPersistenceFails(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	assetRepository.saveGenerationTasksErr = fmt.Errorf("write failed")
	phase := buildPlatformAssetDispatchPersistPhase(&service{assetRepo: assetRepository})
	final := &ListingKitResult{Summary: &GenerationSummary{}}
	tasks := []assetgeneration.Task{{
		ID:              "persisted-task",
		Platform:        "amazon",
		RecipeID:        "hero",
		ExecutionStatus: "planned",
	}}

	phase.run(context.Background(), &Task{ID: "task-persist-error"}, final, tasks)

	if len(final.Summary.Warnings) != 1 || !strings.Contains(final.Summary.Warnings[0], "asset generation task persistence failed: write failed") {
		t.Fatalf("summary warnings = %+v, want persistence warning", final.Summary.Warnings)
	}
	if !hasWorkflowIssue(final.WorkflowIssues, "asset_generation_platform", WorkflowIssueSeverityWarning, "asset_generation_task_persistence_failed") {
		t.Fatalf("workflow issues = %+v, want persistence warning issue", final.WorkflowIssues)
	}
	if !reflect.DeepEqual(final.AssetGenerationTasks, tasks) {
		t.Fatalf("decorated generation tasks = %+v, want %+v", final.AssetGenerationTasks, tasks)
	}
}

func TestPlatformAssetDispatchInventoryPersistPhaseRunPersistsReturnedAssets(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	phase := buildPlatformAssetDispatchInventoryPersistPhase(&service{assetRepo: assetRepository})
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-inventory-persist"},
		Records: []asset.AssetRecord{{
			ID:     "generated-1",
			Kind:   asset.KindSceneImage,
			Origin: asset.OriginGenerated,
			URL:    "https://cdn.example.com/generated-1.jpg",
		}},
		Summary: &asset.InventorySummary{TotalRecords: 1, GeneratedRecords: 1},
	}
	phase.run(context.Background(), inventory, 1)

	if assetRepository.saveInventoryCalls != 1 {
		t.Fatalf("save inventory calls = %d, want 1", assetRepository.saveInventoryCalls)
	}
	savedInventory, err := assetRepository.GetInventory(context.Background(), inventory.Ref)
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	if !hasInventoryURL(savedInventory, "https://cdn.example.com/generated-1.jpg") {
		t.Fatalf("saved inventory = %+v, want returned asset persisted", savedInventory)
	}
}

func TestPlatformAssetDispatchInventoryPersistPhaseRunSkipsWhenNoReturnedAssets(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	phase := buildPlatformAssetDispatchInventoryPersistPhase(&service{assetRepo: assetRepository})
	inventory := &asset.Inventory{
		Ref:     asset.InventoryRef{TaskID: "task-inventory-skip"},
		Records: []asset.AssetRecord{{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://example.com/source-1.jpg"}},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}

	phase.run(context.Background(), inventory, 0)

	if assetRepository.saveInventoryCalls != 0 {
		t.Fatalf("save inventory calls = %d, want 0", assetRepository.saveInventoryCalls)
	}
	if _, err := assetRepository.GetInventory(context.Background(), inventory.Ref); err == nil {
		t.Fatal("expected inventory to remain unpersisted when dispatch returns no assets")
	}
}

func TestPlatformAssetDispatchInventoryPersistPhaseRunKeepsBestEffortPersistence(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	assetRepository.saveInventoryErr = fmt.Errorf("write failed")
	phase := buildPlatformAssetDispatchInventoryPersistPhase(&service{assetRepo: assetRepository})
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-inventory-best-effort"},
		Records: []asset.AssetRecord{{
			ID:     "generated-1",
			Kind:   asset.KindSceneImage,
			Origin: asset.OriginGenerated,
			URL:    "https://cdn.example.com/generated-best-effort.jpg",
		}},
		Summary: &asset.InventorySummary{TotalRecords: 1, GeneratedRecords: 1},
	}
	phase.run(context.Background(), inventory, 1)

	if assetRepository.saveInventoryCalls != 1 {
		t.Fatalf("save inventory calls = %d, want 1", assetRepository.saveInventoryCalls)
	}
	if _, err := assetRepository.GetInventory(context.Background(), inventory.Ref); err == nil {
		t.Fatal("expected inventory write failure to remain best-effort and not persist inventory")
	}
}

func TestPlatformAssetDispatchPhaseRunOrchestratesDispatchMutationAndPersistence(t *testing.T) {
	t.Parallel()

	assetRepository := newStubWorkflowAssetRepository()
	assetGenerator := &stubWorkflowAssetGenerator{
		dispatchResult: &assetgeneration.Result{
			Tasks: []assetgeneration.Task{{
				ID:              "amazon-main",
				TaskID:          "task-dispatch-phase",
				Platform:        "amazon",
				RecipeID:        "amazon-lifestyle",
				ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
				ExecutionStatus: "completed",
			}},
			Assets: []asset.AssetRecord{{
				ID:       "generated-main",
				Kind:     asset.KindSceneImage,
				Origin:   asset.OriginGenerated,
				URL:      "https://cdn.example.com/generated-main.jpg",
				RecipeID: "amazon-lifestyle",
			}},
		},
	}
	phase := buildPlatformAssetDispatchPhase(&service{
		assetGenerator:     assetGenerator,
		assetRepo:          assetRepository,
		assetBundleBuilder: newDefaultAssetBundleBuilder(),
	})
	final := &ListingKitResult{
		Summary: &GenerationSummary{},
		Amazon:  &AmazonPackage{},
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{{
				ID:   "source-1",
				Kind: asset.KindSourceImage,
				URL:  "https://example.com/source-1.jpg",
			}},
		},
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	inventory := &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: "task-dispatch-phase"},
		Records: []asset.AssetRecord{{
			ID:     "source-1",
			Kind:   asset.KindSourceImage,
			Origin: asset.OriginSource,
			URL:    "https://example.com/source-1.jpg",
		}},
		Summary: &asset.InventorySummary{TotalRecords: 1, SourceRecords: 1},
	}
	recipesByPlatform := resolveRecipesForPlatforms(newDefaultAssetRecipeResolver(), []string{"amazon"}, nil)
	generationPlan := &assetgeneration.Result{
		Tasks: []assetgeneration.Task{{
			ID:              "amazon-main",
			TaskID:          "task-dispatch-phase",
			Platform:        "amazon",
			RecipeID:        "amazon-lifestyle",
			ExecutionStatus: "planned",
			CanExecute:      true,
		}},
	}
	persistedGenerationTasks := []assetgeneration.Task{{
		ID:              "amazon-main",
		TaskID:          "task-dispatch-phase",
		Platform:        "amazon",
		RecipeID:        "amazon-lifestyle",
		ExecutionStatus: "planned",
		CanExecute:      true,
	}}

	got := phase.run(
		context.Background(),
		&Task{ID: "task-dispatch-phase"},
		final,
		inventory,
		recipesByPlatform,
		generationPlan,
		persistedGenerationTasks,
		true,
	)

	if assetGenerator.lastDispatchReq == nil {
		t.Fatal("expected dispatch request to be captured")
	}
	if len(assetGenerator.lastDispatchReq.Tasks) != 1 || assetGenerator.lastDispatchReq.Tasks[0].ID != "amazon-main" {
		t.Fatalf("dispatch request tasks = %+v, want collected pending platform task", assetGenerator.lastDispatchReq.Tasks)
	}
	if !hasWorkflowStageStatus(final.WorkflowStages, "asset_generation_platform", WorkflowStageStatusCompleted) {
		t.Fatalf("workflow stages = %+v, want completed asset_generation_platform", final.WorkflowStages)
	}
	if assetRepository.saveInventoryCalls != 1 {
		t.Fatalf("save inventory calls = %d, want 1", assetRepository.saveInventoryCalls)
	}
	savedInventory, err := assetRepository.GetInventory(context.Background(), asset.InventoryRef{TaskID: "task-dispatch-phase"})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	if !hasInventoryURL(savedInventory, "https://cdn.example.com/generated-main.jpg") {
		t.Fatalf("saved inventory = %+v, want dispatched asset persisted", savedInventory)
	}
	if final.AssetInventorySummary == nil || final.AssetInventorySummary.GeneratedRecords != 1 {
		t.Fatalf("asset inventory summary = %+v, want generated record count updated", final.AssetInventorySummary)
	}
	if final.Amazon == nil || final.Amazon.ImageBundle == nil {
		t.Fatalf("amazon image bundle = %+v, want platform bundle attached", final.Amazon)
	}
	for _, pending := range final.Amazon.ImageBundle.PendingGeneration {
		if pending.RecipeID == "amazon-lifestyle" {
			t.Fatalf("amazon pending generation = %+v, dispatched lifestyle recipe should no longer be pending", final.Amazon.ImageBundle.PendingGeneration)
		}
	}
	if assetRepository.savedTaskID != "task-dispatch-phase" {
		t.Fatalf("saved task id = %q, want task-dispatch-phase", assetRepository.savedTaskID)
	}
	if !reflect.DeepEqual(got, assetRepository.savedTasks) {
		t.Fatalf("returned generation tasks = %+v, want persisted generation tasks %+v", got, assetRepository.savedTasks)
	}
	if len(final.AssetGenerationTasks) != 1 || final.AssetGenerationTasks[0].ExecutionStatus != "completed" {
		t.Fatalf("decorated generation tasks = %+v, want completed dispatched task", final.AssetGenerationTasks)
	}
}

func TestRunWorkflowAppliesSheinPlatformFinalizationDecorations(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-shein-copy",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/pillow.jpg"},
			Text:      "pillow cover",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:         "Envelope style pillow cover",
			Description:   "Simple pillow cover for home decor.",
			Category:      []string{"Home", "Textiles", "Pillow Covers"},
			Images:        []string{"https://example.com/pillow.jpg"},
			SellingPoints: []string{"Soft polyester", "Botanical print"},
			Attributes:    map[string]string{"brand": "DemoBrand"},
		},
	}
	ai := &stubSheinContentAI{
		response: `{"title":"Botanical Envelope Pillow Cover for Sofa Couch Bedroom Decor, Soft Polyester Accent Cushion Case","description":"A soft polyester envelope pillow cover designed to refresh sofas, beds, and reading corners with a botanical accent print. The overlap closure keeps the insert tucked in while making everyday styling changes easy."}`,
	}
	svc := &service{
		productSvc:            productSvc,
		assembler:             NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		sheinContentOptimizer: ai,
		assetRecipeResolver:   newDefaultAssetRecipeResolver(),
		assetBundleBuilder:    newDefaultAssetBundleBuilder(),
		assetGenerator:        newDefaultAssetGenerationService(),
	}
	task := &Task{
		ID: "listingkit-task-shein-copy",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/pillow.jpg"},
			Text:      "pillow cover",
			Platforms: []string{"shein"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: false},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if result.Shein == nil {
		t.Fatal("expected shein package")
	}
	if ai.calls != 1 {
		t.Fatal("shein content optimizer was not called")
	}
	if got := result.Shein.ProductNameEn; !strings.Contains(got, "Botanical Envelope Pillow Cover") {
		t.Fatalf("shein title = %q", got)
	}
	if got := result.Shein.Description; !strings.Contains(got, "reading corners") {
		t.Fatalf("shein description = %q", got)
	}

	preview := buildSheinPreviewPayload(result.Shein, result.PodExecution, result.CanonicalProduct, nil, nil)
	if preview == nil || preview.FinalReview == nil {
		t.Fatalf("preview final review = %+v", preview)
	}
	if got := preview.FinalReview.Title; !strings.Contains(got, "Botanical Envelope Pillow Cover") {
		t.Fatalf("final review title = %q", got)
	}
	if got := preview.FinalReview.Description; !strings.Contains(got, "reading corners") {
		t.Fatalf("final review description = %q", got)
	}
}

func TestRunWorkflowAppliesVariantCoverageGuardAfterSheinReview(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductService{
		task: &productenrich.Task{
			ID: "product-task-variant-coverage",
			Request: &productenrich.GenerateRequest{
				ImageURLs: []string{"https://example.com/shared-main.jpg"},
				Text:      "insulated tumbler",
			},
		},
		product: &productenrich.ProductJSON{
			Title:       "Insulated Tumbler",
			Description: "Double wall tumbler",
			Category:    []string{"Home", "Kitchen"},
			Images:      []string{"https://example.com/shared-main.jpg"},
			Attributes:  map[string]string{"brand": "DemoBrand"},
			Variants: []productenrich.ProductVariant{
				{
					SKU:        "RED-20OZ",
					Attributes: map[string]string{"Color": "red", "Size": "20oz"},
					Images:     []string{"https://example.com/shared-main.jpg"},
					IsDefault:  true,
				},
				{
					SKU:        "GREEN-20OZ",
					Attributes: map[string]string{"Color": "green", "Size": "20oz"},
					Images:     []string{"https://example.com/shared-main.jpg"},
				},
			},
		},
	}
	svc := &service{
		productSvc: productSvc,
		assembler: NewAssemblerWithConfig(AssemblerConfig{
			AmazonBuilder: stubAmazonDraftBuilder{},
			SheinCategoryResolver: sheinpub.NewCategoryResolver(stubSheinCategoryAPI{
				info: &sheincategory.CategoryInfo{
					ProductTypeID:        9001,
					LevelOneCategoryID:   1001,
					LevelOneCategoryName: "Drinkware",
				},
			}),
			SheinAttributeResolver: sheinpub.NewAttributeResolver(stubSheinAttributeAPI{
				templates: &sheinattribute.AttributeTemplateInfo{
					Data: []sheinattribute.AttributeTemplate{{
						AttributeInfos: []sheinattribute.AttributeInfo{
							{
								AttributeID:     501,
								AttributeName:   "颜色",
								AttributeNameEn: "Color",
								AttributeValueInfoList: []sheinattribute.AttributeValue{
									{AttributeValueID: 90001, AttributeValue: "红色", AttributeValueEn: "red"},
									{AttributeValueID: 90002, AttributeValue: "绿色", AttributeValueEn: "green"},
								},
							},
							{
								AttributeID:     502,
								AttributeName:   "尺寸",
								AttributeNameEn: "Size",
								AttributeValueInfoList: []sheinattribute.AttributeValue{
									{AttributeValueID: 90003, AttributeValue: "20盎司", AttributeValueEn: "20oz"},
								},
							},
						},
					}},
				},
			}, nil),
			SheinSaleAttributeResolver: sheinpub.NewSaleAttributeResolver(stubSheinAttributeAPI{
				templates: &sheinattribute.AttributeTemplateInfo{
					Data: []sheinattribute.AttributeTemplate{{
						AttributeInfos: []sheinattribute.AttributeInfo{
							{
								AttributeID:     501,
								AttributeName:   "颜色",
								AttributeNameEn: "Color",
								AttributeValueInfoList: []sheinattribute.AttributeValue{
									{AttributeValueID: 90001, AttributeValue: "红色", AttributeValueEn: "red"},
									{AttributeValueID: 90002, AttributeValue: "绿色", AttributeValueEn: "green"},
								},
							},
							{
								AttributeID:     502,
								AttributeName:   "尺寸",
								AttributeNameEn: "Size",
								AttributeValueInfoList: []sheinattribute.AttributeValue{
									{AttributeValueID: 90003, AttributeValue: "20盎司", AttributeValueEn: "20oz"},
								},
							},
						},
					}},
				},
			}, nil),
		}),
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      newDefaultAssetGenerationService(),
	}
	task := &Task{
		ID: "listingkit-task-variant-coverage",
		Request: &GenerateRequest{
			ImageURLs:          []string{"https://example.com/shared-main.jpg"},
			Text:               "insulated tumbler",
			Platforms:          []string{"shein"},
			Country:            "US",
			Language:           "en_US",
			TargetCategoryHint: "1001",
			Options: &GenerateOptions{
				ProcessImages: false,
				ImageStrategy: sheinImageStrategyAIGenerated,
				SheinStudio: &SheinStudioOptions{
					ProductImageURLs: []string{"https://cdn.example.com/shared-ai-main.jpg"},
				},
			},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if result.Shein == nil || result.Shein.RequestDraft == nil {
		t.Fatalf("shein package = %+v, want populated package", result.Shein)
	}
	if got := len(result.Shein.RequestDraft.SKCList); got != 2 {
		t.Fatalf("shein skc count = %d, want 2", got)
	}
	if result.Summary == nil || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs review", result.Summary)
	}
	if len(result.Summary.Warnings) == 0 {
		t.Fatalf("summary warnings = %+v, want coverage warning", result.Summary)
	}
	if len(result.ReviewReasons) == 0 {
		t.Fatalf("review reasons = %+v, want coverage reason", result.ReviewReasons)
	}
	if len(result.Shein.ReviewNotes) == 0 {
		t.Fatalf("shein review notes = %+v, want coverage review note", result.Shein.ReviewNotes)
	}
	if result.Shein.Metadata[sheinVariantImageCoverageStatusKey] != "blocked" {
		t.Fatalf("shein metadata = %#v, want blocked coverage status", result.Shein.Metadata)
	}
	coverageWarning := result.Shein.Metadata[sheinVariantImageCoverageMessageKey]
	if coverageWarning == "" {
		t.Fatalf("shein metadata = %#v, want coverage warning message", result.Shein.Metadata)
	}
	for _, issue := range result.WorkflowIssues {
		if issue.Stage == "shein_review" && issue.Severity == WorkflowIssueSeverityReview && issue.Message == coverageWarning {
			t.Fatalf("workflow issues = %+v, variant coverage guard should not add shein_review issue before review phase completes", result.WorkflowIssues)
		}
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

func TestRunStandardProductWorkflowRunsRemoteSDSSyncWhenImageProcessingIsDisabled(t *testing.T) {
	t.Parallel()

	sdsSvc := &stubWorkflowSDSSyncService{
		remoteResult: &sdsworkflow.SyncResult{
			DesignResult: &sdsdesign.PrepareSyncDesignResult{
				Page: &sdsdesign.DesignProductPage{
					Product: sdsdesign.DesignProduct{
						ID:   101,
						Name: "Remote Rendered Product",
					},
				},
				Request: &sdsdesign.SyncDesignRequest{
					PrototypeGroupID: 7001,
					Prototypes: []sdsdesign.SyncDesignPrototype{{
						Layers: []sdsdesign.SyncDesignLayer{{LayerID: "layer-1"}},
					}},
				},
				RenderedImageURLs: []string{
					"https://cdn.sdspod.com/out/remote-main.jpg",
				},
			},
		},
	}
	svc := &service{
		sdsSyncSvc:          sdsSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator:      newDefaultAssetGenerationService(),
	}
	task := &Task{
		ID: "task-remote-sds-sync",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-remote.jpg"},
			Text:      "remote sds sync",
			Platforms: []string{"shein"},
			Country:   "US",
			Language:  "en_US",
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					VariantID:        101,
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
				},
			},
		},
	}

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v", err)
	}
	if sdsSvc.remoteCalls != 1 {
		t.Fatalf("remote calls = %d, want 1", sdsSvc.remoteCalls)
	}
	if state.result.SDSDesignResult == nil || state.result.SDSDesignResult.Status != "completed" {
		t.Fatalf("sds design result = %+v, want completed remote SDS sync", state.result.SDSDesignResult)
	}
	if state.result.CanonicalProduct == nil || len(state.result.CanonicalProduct.Images) != 1 || state.result.CanonicalProduct.Images[0].URL != "https://cdn.sdspod.com/out/remote-main.jpg" {
		t.Fatalf("canonical product = %+v, want remote rendered image applied", state.result.CanonicalProduct)
	}
	if !hasChildTaskStatus(state.result.ChildTasks, "sds_design_sync", string(TaskStatusCompleted)) {
		t.Fatalf("child tasks = %+v, want completed sds_design_sync", state.result.ChildTasks)
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

func TestRunWorkflowPersistsDeferredPlatformDispatchOutputs(t *testing.T) {
	t.Parallel()

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
		dispatchResult: &assetgeneration.Result{
			Tasks: []assetgeneration.Task{{
				TaskID:          "asset-task-1",
				ID:              "asset-task-1",
				Platform:        "amazon",
				ExecutionMode:   "deferred_stub",
				ExecutionStatus: "completed",
			}},
			Assets: []asset.AssetRecord{{
				Kind: asset.KindGalleryImage,
				URL:  "https://cdn.example.com/generated-gallery.jpg",
			}},
		},
	}

	result, repo := runWorkflowWithDeferredDispatchFixture(t, "listingkit-task-deferred-success", assetGenerator)

	if result.AssetInventorySummary == nil || result.AssetInventorySummary.TotalRecords == 0 {
		t.Fatalf("asset inventory summary = %+v, want persisted records", result.AssetInventorySummary)
	}
	if result.Amazon == nil || result.Amazon.ImageBundle == nil {
		t.Fatalf("amazon image bundle = %+v, want bundle after deferred dispatch", result.Amazon)
	}
	hasGeneratedGalleryAsset := false
	for _, item := range result.Amazon.ImageBundle.Gallery {
		if item.URL == "https://cdn.example.com/generated-gallery.jpg" {
			hasGeneratedGalleryAsset = true
			break
		}
	}
	if !hasGeneratedGalleryAsset {
		t.Fatalf("amazon image bundle = %+v, want merged deferred-dispatch gallery asset", result.Amazon.ImageBundle)
	}
	inventory, err := repo.GetInventory(context.Background(), asset.InventoryRef{TaskID: "listingkit-task-deferred-success"})
	if err != nil {
		t.Fatalf("GetInventory() error = %v", err)
	}
	if inventory == nil || !hasInventoryURL(inventory, "https://cdn.example.com/generated-gallery.jpg") {
		t.Fatalf("inventory = %+v, want merged deferred-dispatch asset", inventory)
	}
	tasks, err := repo.ListGenerationTasks(context.Background(), "listingkit-task-deferred-success")
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected persisted generation tasks")
	}
	if tasks[0].ExecutionMode != "deferred_stub" || tasks[0].ExecutionStatus != "completed" {
		t.Fatalf("generation tasks = %+v, want completed deferred dispatch task", tasks)
	}
}

func TestRunWorkflowFinalizesSummaryAfterPlatformDispatch(t *testing.T) {
	t.Parallel()

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
		dispatchResult: &assetgeneration.Result{
			Tasks: []assetgeneration.Task{{
				TaskID:          "asset-task-1",
				ID:              "asset-task-1",
				Platform:        "amazon",
				ExecutionMode:   "deferred_stub",
				ExecutionStatus: "completed",
			}},
			Assets: []asset.AssetRecord{{
				Kind: asset.KindGalleryImage,
				URL:  "https://cdn.example.com/generated-gallery.jpg",
			}},
		},
	}

	result, _ := runWorkflowWithDeferredDispatchFixture(t, "listingkit-task-summary-finalize", assetGenerator)

	if result.Summary == nil {
		t.Fatal("expected summary")
	}
	if result.Summary.WarningCount < 0 || result.Summary.IssueCount < 0 {
		t.Fatalf("summary = %+v, want finalized counts", result.Summary)
	}
	if result.StandardProductSnapshot == nil {
		t.Fatalf("standard snapshot = %+v, want preserved snapshot", result.StandardProductSnapshot)
	}
	if len(result.PlatformAssetRenderPreviews) == 0 {
		t.Fatalf("platform asset render previews = %+v, want synced platform previews", result.PlatformAssetRenderPreviews)
	}
}

func TestPlatformSummaryPhaseFinalizesCompletionState(t *testing.T) {
	t.Parallel()

	task := &Task{ID: "listingkit-task-summary-phase", Request: &GenerateRequest{Platforms: []string{"shein"}}}
	final := &ListingKitResult{
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{{
				ID:   "asset-main",
				Kind: asset.KindMainImage,
				URL:  "https://cdn.example.com/main.jpg",
			}},
		},
		Shein: &SheinPackage{
			Inspection: &SheinInspection{
				NeedsReview: true,
				Summary:     []string{"manual review"},
			},
			ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:     "main",
					AssetID: "asset-main",
					URL:     "https://cdn.example.com/main.jpg",
				},
			},
		},
		Summary: &GenerationSummary{
			Warnings: []string{"existing warning"},
		},
		WorkflowIssues: []WorkflowIssue{{
			Stage:    "asset_generation_platform",
			Severity: WorkflowIssueSeverityReview,
			Code:     "manual_review_required",
			Message:  "manual review",
		}},
	}

	result := buildPlatformSummaryPhase().run(task, final)

	if result != final {
		t.Fatalf("result pointer = %p, want %p", result, final)
	}
	if result.Summary == nil || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs review", result.Summary)
	}
	if result.Summary.WarningCount < 0 || result.Summary.ReviewCount == 0 || result.Summary.IssueCount == 0 {
		t.Fatalf("summary = %+v, want finalized counts", result.Summary)
	}
	if len(result.PlatformAssetRenderPreviews) == 0 {
		t.Fatalf("platform previews = %+v, want synced previews", result.PlatformAssetRenderPreviews)
	}
	if len(result.WorkflowStages) != 0 {
		t.Fatalf("workflow stages = %+v, want summary seam not to prepare review stages", result.WorkflowStages)
	}
}

func TestPlatformFinalizePhasePreparesReviewBeforeCompletion(t *testing.T) {
	t.Parallel()

	task := &Task{ID: "listingkit-task-finalize-phase", Request: &GenerateRequest{Platforms: []string{"shein"}}}
	final := &ListingKitResult{
		Shein: &SheinPackage{
			Inspection: &SheinInspection{
				NeedsReview: true,
				Summary:     []string{"manual review"},
			},
		},
		Summary: &GenerationSummary{
			Warnings: []string{"existing warning"},
		},
	}
	snapshot := &StandardProductSnapshot{
		Summary: &GenerationSummary{
			Warnings: []string{"snapshot warning"},
		},
	}

	result := buildPlatformFinalizePhase(&service{}).run(
		context.Background(),
		task,
		final,
		snapshot,
		nil,
		nil,
		nil,
		nil,
		false,
		nil,
	)

	if result != final {
		t.Fatalf("result pointer = %p, want %p", result, final)
	}
	if result.Summary == nil || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs review", result.Summary)
	}
	if !strings.Contains(strings.Join(result.Summary.Warnings, "\n"), "snapshot warning") {
		t.Fatalf("summary warnings = %#v, want snapshot warnings merged", result.Summary.Warnings)
	}
	if !hasWorkflowStageStatus(result.WorkflowStages, "shein_review", WorkflowStageStatusCompleted) {
		t.Fatalf("workflow stages = %+v, want completed shein_review", result.WorkflowStages)
	}
	if !hasWorkflowIssue(result.WorkflowIssues, "shein_review", WorkflowIssueSeverityReview, "shein_review_required") {
		t.Fatalf("workflow issues = %+v, want shein review workflow issue", result.WorkflowIssues)
	}
	if result.Summary.ReviewCount == 0 || result.Summary.IssueCount == 0 {
		t.Fatalf("summary = %+v, want finalized counts after review prep", result.Summary)
	}
}

func TestPlatformReviewPhasePreparesSheinReview(t *testing.T) {
	t.Parallel()

	coverageWarning := "coverage guard warning"
	final := &ListingKitResult{
		Shein: &SheinPackage{
			Inspection: &SheinInspection{
				NeedsReview: true,
				Summary:     []string{"inspection review"},
			},
			ReviewNotes: []string{coverageWarning},
			Metadata: map[string]string{
				sheinVariantImageCoverageStatusKey:  "blocked",
				sheinVariantImageCoverageMessageKey: coverageWarning,
			},
		},
		Summary: &GenerationSummary{
			Warnings: []string{"existing warning"},
		},
	}
	snapshot := &StandardProductSnapshot{
		Summary: &GenerationSummary{
			Warnings: []string{"snapshot warning"},
		},
	}

	buildPlatformReviewPhase().run(final, snapshot)

	if final.Summary == nil || !final.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs review", final.Summary)
	}
	if !strings.Contains(strings.Join(final.Summary.Warnings, "\n"), "snapshot warning") {
		t.Fatalf("summary warnings = %#v, want snapshot warnings merged", final.Summary.Warnings)
	}
	if !hasWorkflowStageStatus(final.WorkflowStages, "shein_review", WorkflowStageStatusCompleted) {
		t.Fatalf("workflow stages = %+v, want completed shein_review", final.WorkflowStages)
	}
	if !hasWorkflowIssue(final.WorkflowIssues, "shein_review", WorkflowIssueSeverityReview, "shein_review_required") {
		t.Fatalf("workflow issues = %+v, want shein review workflow issue", final.WorkflowIssues)
	}
}

func TestPlatformReviewPhaseDoesNotConvertCoverageGuardReasonIntoSheinReviewIssue(t *testing.T) {
	t.Parallel()

	coverageWarning := "coverage guard warning"
	inspectionReason := "inspection review"
	final := &ListingKitResult{
		Shein: &SheinPackage{
			Inspection: &SheinInspection{
				NeedsReview: true,
				Summary:     []string{inspectionReason},
			},
			ReviewNotes: []string{coverageWarning},
			Metadata: map[string]string{
				sheinVariantImageCoverageStatusKey:  "blocked",
				sheinVariantImageCoverageMessageKey: coverageWarning,
			},
		},
		Summary: &GenerationSummary{
			NeedsReview: true,
			Warnings:    []string{coverageWarning},
		},
		ReviewReasons: []string{coverageWarning},
	}

	buildPlatformReviewPhase().run(final, nil)

	if !strings.Contains(strings.Join(final.Summary.Warnings, "\n"), coverageWarning) {
		t.Fatalf("summary warnings = %#v, want coverage warning retained", final.Summary.Warnings)
	}
	if !strings.Contains(strings.Join(final.ReviewReasons, "\n"), coverageWarning) {
		t.Fatalf("review reasons = %#v, want coverage warning retained", final.ReviewReasons)
	}
	if !strings.Contains(strings.Join(final.Shein.ReviewNotes, "\n"), coverageWarning) {
		t.Fatalf("shein review notes = %#v, want coverage warning retained", final.Shein.ReviewNotes)
	}
	if !hasWorkflowIssue(final.WorkflowIssues, "shein_review", WorkflowIssueSeverityReview, "shein_review_required") {
		t.Fatalf("workflow issues = %+v, want non-coverage shein review issue", final.WorkflowIssues)
	}
	for _, issue := range final.WorkflowIssues {
		if issue.Stage == "shein_review" && issue.Severity == WorkflowIssueSeverityReview && issue.Message == coverageWarning {
			t.Fatalf("workflow issues = %+v, coverage guard warning should not become shein_review issue", final.WorkflowIssues)
		}
		if issue.Stage == "shein_review" && issue.Severity == WorkflowIssueSeverityReview && issue.Code == "shein_review_required" && issue.Message == inspectionReason {
			return
		}
	}

	t.Fatalf("workflow issues = %+v, want inspection review issue retained", final.WorkflowIssues)
}

func runWorkflowWithDeferredDispatchFixture(
	t *testing.T,
	taskID string,
	assetGenerator *stubWorkflowAssetGenerator,
) (*ListingKitResult, *assetrepo.MemRepository) {
	t.Helper()

	productSvc := &stubWorkflowProductService{
		task: &productenrich.Task{
			ID: "product-task-" + taskID,
			Request: &productenrich.GenerateRequest{
				ImageURLs: []string{"https://example.com/source.jpg"},
				Text:      "poster",
			},
		},
		product: &productenrich.ProductJSON{
			Title:      "Poster",
			Category:   []string{"Home"},
			Images:     []string{"https://example.com/source.jpg"},
			Attributes: map[string]string{"material": "paper"},
		},
	}
	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-" + taskID},
		result: &productimage.ImageProcessResult{
			MainImage: &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg"},
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
		assetGenerator:      assetGenerator,
	}
	task := &Task{
		ID: taskID,
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
	return result, assetRepository
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

func hasInventoryURL(inventory *asset.Inventory, url string) bool {
	if inventory == nil {
		return false
	}
	for _, record := range inventory.Records {
		if record.URL == url {
			return true
		}
	}
	return false
}
