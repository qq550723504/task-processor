package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
)

type stubSubmitRepo struct {
	mu                              sync.Mutex
	task                            *Task
	savedSubmissionPhases           []string
	failSaveWhenCurrentPhaseCleared bool
	saveFailed                      bool
}

func (r *stubSubmitRepo) CreateTask(ctx context.Context, task *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *task
	r.task = &copied
	return nil
}

func (r *stubSubmitRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied := *r.task
	return &copied, nil
}

func (r *stubSubmitRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil {
		return []Task{}, 0, nil
	}
	copied := *r.task
	return []Task{copied}, 1, nil
}

func (r *stubSubmitRepo) MarkProcessing(ctx context.Context, taskID string) error { return nil }
func (r *stubSubmitRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return r.SaveTaskResult(ctx, taskID, result)
}
func (r *stubSubmitRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	return r.SaveTaskResult(ctx, taskID, result)
}
func (r *stubSubmitRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}
func (r *stubSubmitRepo) PrepareRetry(ctx context.Context, taskID string) error { return nil }
func (r *stubSubmitRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return nil
}
func (r *stubSubmitRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	if r.failSaveWhenCurrentPhaseCleared && !r.saveFailed && result != nil && result.Shein != nil && result.Shein.Submission != nil && result.Shein.Submission.CurrentPhase == "" {
		r.saveFailed = true
		return errors.New("save task result failed")
	}
	r.task.Result = result
	r.task.UpdatedAt = time.Now()
	if result != nil && result.Shein != nil && result.Shein.Submission != nil {
		r.savedSubmissionPhases = append(r.savedSubmissionPhases, result.Shein.Submission.CurrentPhase)
	}
	return nil
}

func (r *stubSubmitRepo) MutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied := *r.task
	out := &copied
	if mutate != nil {
		if err := mutate(r.task); err != nil {
			return out, err
		}
	}
	r.task.UpdatedAt = time.Now()
	copied = *r.task
	return &copied, nil
}

func (r *stubSubmitRepo) hasSavedSubmissionPhase(phase string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, saved := range r.savedSubmissionPhases {
		if saved == phase {
			return true
		}
	}
	return false
}

type stubSubmitProductService struct{}

func (stubSubmitProductService) CreateGenerateTask(ctx context.Context, req *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return nil, errors.New("not implemented")
}

func (stubSubmitProductService) GetTaskResult(ctx context.Context, taskID string) (*productenrich.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (stubSubmitProductService) ProcessProduct(ctx context.Context, task *productenrich.Task) (*productenrich.ProductJSON, error) {
	return nil, errors.New("not implemented")
}

type submitResolutionCacheStore struct {
	mu      sync.Mutex
	entries []*sheinpub.SheinResolutionCacheEntry
}

func (s *submitResolutionCacheStore) GetResolutionCache(ctx context.Context, kind string, storeID string, cacheKey string) (*sheinpub.SheinResolutionCacheEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		if entry.CacheKind == kind && entry.StoreID == storeID && entry.CacheKey == cacheKey {
			copied := *entry
			return &copied, nil
		}
	}
	return nil, nil
}

func (s *submitResolutionCacheStore) SaveResolutionCache(ctx context.Context, entry *sheinpub.SheinResolutionCacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry == nil {
		return nil
	}
	copied := *entry
	s.entries = append(s.entries, &copied)
	return nil
}

func (s *submitResolutionCacheStore) DeleteResolutionCache(ctx context.Context, kind string, storeID string, cacheKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.entries[:0]
	for _, entry := range s.entries {
		if entry.CacheKind == kind && entry.StoreID == storeID && entry.CacheKey == cacheKey {
			continue
		}
		out = append(out, entry)
	}
	s.entries = out
	return nil
}

func (s *submitResolutionCacheStore) snapshot() []sheinpub.SheinResolutionCacheEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]sheinpub.SheinResolutionCacheEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry != nil {
			out = append(out, *entry)
		}
	}
	return out
}

type stubSheinProductAPIBuilder struct {
	api sheinproduct.ProductAPI
	msg string
}

func (s stubSheinProductAPIBuilder) BuildProductAPI(storeID int64) (sheinproduct.ProductAPI, string) {
	return s.api, s.msg
}

type stubSheinImageAPIBuilder struct {
	api sheinimage.ImageAPI
	msg string
}

func (s stubSheinImageAPIBuilder) BuildImageAPI(storeID int64) (sheinimage.ImageAPI, string) {
	return s.api, s.msg
}

type stubSheinTranslateAPIBuilder struct {
	api sheintranslateapi.TranslateAPI
	msg string
}

func (s stubSheinTranslateAPIBuilder) BuildTranslateAPI(storeID int64) (sheintranslateapi.TranslateAPI, string) {
	return s.api, s.msg
}

type stubSheinTranslateAPI struct {
	calls []string
}

func (s *stubSheinTranslateAPI) Translate(text string, from, to string) (string, error) {
	s.calls = append(s.calls, from+"->"+to+":"+text)
	switch to {
	case "en":
		return "English " + text, nil
	case "es":
		return "Spanish " + text, nil
	default:
		return to + " " + text, nil
	}
}

type stubSheinContentAI struct {
	response string
	err      error
	calls    int
}

func (s *stubSheinContentAI) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	response := strings.TrimSpace(s.response)
	if response == "" {
		response = `{"title":"Optimized English Product Title for SHEIN","description":"Optimized English product description for SHEIN with clear features and customer-friendly wording."}`
	}
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Content: response},
		}},
	}, nil
}

func (s *stubSheinContentAI) Generate(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubSheinContentAI) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *stubSheinContentAI) GetDefaultModel() string {
	return "test"
}

type stubSheinImageAPI struct {
	mu             sync.Mutex
	uploaded       map[string]string
	calls          map[string]int
	originalCalls  int
	originalUpload string
	err            error
}

func (s *stubSheinImageAPI) UploadOriginalImage(imageData []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.originalCalls++
	if s.err != nil {
		return "", s.err
	}
	if s.originalUpload != "" {
		return s.originalUpload, nil
	}
	return fmt.Sprintf("https://img.shein.com/uploaded/color-block-%d.jpg", s.originalCalls), nil
}

func (s *stubSheinImageAPI) DownloadAndUploadImage(imageURL string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[imageURL]++
	if s.err != nil {
		return "", s.err
	}
	if s.uploaded != nil {
		if uploadedURL, ok := s.uploaded[imageURL]; ok {
			return uploadedURL, nil
		}
	}
	return "https://img.shein.com/uploaded/" + strings.TrimPrefix(imageURL, "https://"), nil
}

type stubSheinProductAPI struct {
	publishResponse *sheinproduct.SheinResponse
	publishErr      error
	publishHook     func(*sheinproduct.Product)
	saveResponse    *sheinproduct.SheinResponse
	saveErr         error
	saveHook        func(*sheinproduct.Product)
	recordResponse  *sheinproduct.RecordResponse
	recordErr       error
	recordHook      func(*sheinproduct.ProductRecordRequest)
}

func (s stubSheinProductAPI) GetProduct(productID string) (*sheinproduct.Product, error) {
	return nil, errors.New("not implemented")
}
func (s stubSheinProductAPI) UpdateProduct(product *sheinproduct.Product) error {
	return errors.New("not implemented")
}
func (s stubSheinProductAPI) DeleteProduct(productID string) error {
	return errors.New("not implemented")
}
func (s stubSheinProductAPI) GetPartInfo(categoryID int) (*sheinproduct.PartInfoResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubSheinProductAPI) SaveDraftProduct(prod *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	if s.saveHook != nil {
		s.saveHook(prod)
	}
	return s.saveResponse, "", s.saveErr
}
func (s stubSheinProductAPI) PublishProduct(prod *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	if s.publishHook != nil {
		s.publishHook(prod)
	}
	return s.publishResponse, "", s.publishErr
}
func (s stubSheinProductAPI) ConfirmPublish(product *sheinproduct.Product) (bool, string, error) {
	return false, "", errors.New("not implemented")
}
func (s stubSheinProductAPI) Record(request *sheinproduct.ProductRecordRequest) (*sheinproduct.RecordResponse, error) {
	if s.recordHook != nil {
		s.recordHook(request)
	}
	return s.recordResponse, s.recordErr
}
func (s stubSheinProductAPI) ListProducts(pageNum, pageSize int, request *sheinproduct.ProductListRequest) (*sheinproduct.ProductListResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubSheinProductAPI) QueryStock(request *sheinproduct.StockQueryRequest) (*sheinproduct.StockQueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubSheinProductAPI) QueryInventory(spuName string) (*sheinproduct.InventoryQueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubSheinProductAPI) UpdateInventory(request *sheinproduct.InventoryUpdateRequest) error {
	return errors.New("not implemented")
}
func (s stubSheinProductAPI) QueryPrice(spuName string) (*sheinproduct.PriceQueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubSheinProductAPI) QueryCostPrice(spuName string, skcNameList []string) (*sheinproduct.CostPriceQueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (s stubSheinProductAPI) OffShelf(request *sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}
func (s stubSheinProductAPI) OnShelf(request *sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}

func makeSheinRecordResponse(items ...sheinproduct.RecordItem) *sheinproduct.RecordResponse {
	resp := &sheinproduct.RecordResponse{Code: "0", Msg: "success"}
	resp.Info.Data = append(resp.Info.Data, items...)
	resp.Info.Meta.Count = len(items)
	return resp
}

func makeReadySheinTask() *Task {
	productTypeID := 901
	valueID := 2493
	sizeValueID := 267
	return &Task{
		ID: "submit-task-1",
		Request: &GenerateRequest{
			SheinStoreID: 869,
		},
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "submit-task-1",
			Shein: &SheinPackage{
				CategoryID:     3221,
				CategoryIDList: []int{1, 2, 3221},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  1,
				CategoryResolution: &SheinCategoryResolution{
					Status:         "resolved",
					Source:         "target_category_hint",
					CategoryID:     3221,
					CategoryIDList: []int{1, 2, 3221},
					ProductTypeID:  productTypeID,
					TopCategoryID:  1,
					MatchedPath:    []string{"家居&生活", "厨房&餐厅", "饮具", "真空瓶和保温杯"},
				},
				AttributeResolution: &SheinAttributeResolution{
					Status:          "resolved",
					Source:          "attribute_templates",
					CategoryID:      3221,
					TemplateCount:   3,
					ResolvedCount:   1,
					UnresolvedCount: 0,
				},
				ResolvedAttributes: []SheinResolvedAttribute{{
					Name:        "Capacity",
					Value:       "420ml",
					AttributeID: 7001,
					MatchedBy:   "template_exact",
				}},
				SaleAttributeResolution: &SheinSaleAttributeResolution{
					Status:                   "resolved",
					Source:                   "sale_attribute_templates",
					CategoryID:               3221,
					PrimaryAttributeID:       27,
					SecondaryAttributeID:     87,
					PrimarySourceDimension:   "颜色",
					SecondarySourceDimension: "尺码",
					RecommendCategoryReview:  false,
					SKCAttributes: []SheinResolvedSaleAttribute{{
						Scope:            "skc",
						Name:             "Color",
						Value:            "Black",
						AttributeID:      27,
						AttributeValueID: &valueID,
						MatchedBy:        "template_exact",
					}},
					SKUAttributes: []SheinResolvedSaleAttribute{{
						Scope:            "sku",
						Name:             "Size",
						Value:            "39",
						AttributeID:      87,
						AttributeValueID: &sizeValueID,
						MatchedBy:        "template_exact",
					}},
				},
				SkcList: []SheinSKCPackage{{
					SkcName:      "Black",
					SaleName:     "Black",
					SupplierCode: "SKC-1",
					MainImageURL: "https://cdn.example.com/main.jpg",
					SKUs: []common.Variant{{
						SKU: "SKU-1",
						Attributes: map[string]string{
							"颜色": "Black",
							"尺码": "39",
						},
					}},
				}},
				Images: &PlatformImageSet{
					MainImage: "https://cdn.example.com/main.jpg",
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{
						SupplierCode: "SKC-1",
						SaleAttribute: &SheinResolvedSaleAttribute{
							Scope:            "skc",
							Name:             "Color",
							Value:            "Black",
							AttributeID:      27,
							AttributeValueID: &valueID,
						},
						SKUList: []SheinSKUDraft{{
							SupplierSKU: "SKU-1",
							Currency:    "USD",
							CostPrice:   "10.00",
							BasePrice:   "19.99",
							StockCount:  20,
							SaleAttributes: []SheinResolvedSaleAttribute{{
								Scope:            "sku",
								Name:             "Size",
								Value:            "39",
								AttributeID:      87,
								AttributeValueID: &sizeValueID,
							}},
						}},
					}},
				},
				PreviewProduct: &sheinproduct.Product{
					CategoryID:            3221,
					CategoryIDList:        []int{1, 2, 3221},
					ProductTypeID:         &productTypeID,
					TopCategoryID:         1,
					MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Bottle"}},
					MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Bottle desc"}},
					ProductAttributeList:  []sheinproduct.ProductAttribute{{AttributeID: 7001, AttributeExtraValue: "420ml"}},
					SKCList: []sheinproduct.SKC{{
						SaleAttribute: sheinproduct.SaleAttribute{
							AttributeID:      27,
							AttributeValueID: valueID,
						},
						ImageInfo: sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{{
							ImageType: 1,
							ImageSort: 1,
							ImageURL:  "https://img.shein.com/uploaded/default-main.jpg",
						}}},
						SKUS: []sheinproduct.SKU{{
							SupplierSKU: "SKU-1",
							CostInfo: &sheinproduct.CostInfo{
								CostPrice: "10.00",
								Currency:  "USD",
							},
							PriceInfoList: []sheinproduct.PriceInfo{{
								SubSite:   "US",
								BasePrice: 19.99,
								Currency:  "USD",
							}},
							StockInfoList: []sheinproduct.StockInfo{{
								MerchantWarehouseCode: "US",
								InventoryNum:          20,
							}},
							SaleAttributeList: []sheinproduct.SaleAttribute{{
								AttributeID:      87,
								AttributeValueID: 267,
							}},
						}},
					}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestSubmitTaskReturnsBlockedWhenReadinessIsNotReady(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.SaleAttributeResolution.Status = "partial"
	task.Result.Shein.SaleAttributeResolution.SKCAttributes = nil
	task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:             repo,
		ProductService:         stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-fail-123"})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submit err = %v, want ErrSubmitBlocked", err)
	}
}

func TestSubmitTaskPersistsSheinSubmissionWhenProductAPIUnavailable(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			msg: "store token missing",
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-fail-123"})
	if err == nil || !strings.Contains(err.Error(), "store token missing") {
		t.Fatalf("submit err = %v, want store token missing", err)
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("submission was not persisted: %+v", saved.Result)
	}
	if saved.Result.Shein.Submission.LastAction != "publish" ||
		saved.Result.Shein.Submission.LastStatus != "failed" ||
		!strings.Contains(saved.Result.Shein.Submission.LastError, "store token missing") {
		t.Fatalf("submission failure = %+v", saved.Result.Shein.Submission)
	}
	if saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submit current state was not cleared: %+v", saved.Result.Shein.Submission)
	}
	if saved.Result.Shein.Submission.Publish == nil || saved.Result.Shein.Submission.Publish.RequestID != "publish-fail-123" {
		t.Fatalf("publish record = %+v, want request id publish-fail-123", saved.Result.Shein.Submission.Publish)
	}
	if saved.Result.Shein.Submission.Publish.Phase != sheinpub.SubmissionPhaseValidate {
		t.Fatalf("publish phase = %q, want %q", saved.Result.Shein.Submission.Publish.Phase, sheinpub.SubmissionPhaseValidate)
	}
	if len(saved.Result.Shein.SubmissionEvents) == 0 || saved.Result.Shein.SubmissionEvents[len(saved.Result.Shein.SubmissionEvents)-1].RequestID != "publish-fail-123" {
		t.Fatalf("submission events = %+v, want request id publish-fail-123", saved.Result.Shein.SubmissionEvents)
	}
}

func TestSubmitTaskPersistsSheinSubmissionOnPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SPUName = "Display Title Should Not Be Submitted"
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{
						Success: true,
						SPUName: "SPU-123",
						Version: "v1",
					},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if submitted.SPUName != "" {
		t.Fatalf("submitted spu_name = %q, want empty for new SHEIN product", submitted.SPUName)
	}
	if len(submitted.MultiLanguageNameList) == 0 {
		t.Fatal("submitted product title is missing from multi_language_name_list")
	}
	if preview == nil || preview.Shein == nil || preview.Shein.Submission == nil {
		t.Fatalf("preview submission = %+v", preview)
	}
	if preview.Shein.Submission.LastAction != "publish" || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submission = %+v", preview.Shein.Submission)
	}
	if preview.Shein.Submission.Publish == nil || preview.Shein.Submission.Publish.Result == nil || !preview.Shein.Submission.Publish.Result.Success {
		t.Fatalf("submission publish = %+v", preview.Shein.Submission.Publish)
	}
	if preview.Shein.Submission.CurrentAction != "" || preview.Shein.Submission.CurrentPhase != "" || preview.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("submit current state was not cleared: %+v", preview.Shein.Submission)
	}
	if preview.Shein.Submission.Publish.RequestID != "publish-123" {
		t.Fatalf("publish request id = %q, want publish-123", preview.Shein.Submission.Publish.RequestID)
	}
	if preview.Shein.Submission.Publish.StartedAt.IsZero() || preview.Shein.Submission.Publish.FinishedAt == nil {
		t.Fatalf("publish timing was not recorded: %+v", preview.Shein.Submission.Publish)
	}
	if len(preview.Shein.SubmissionEvents) == 0 || preview.Shein.SubmissionEvents[len(preview.Shein.SubmissionEvents)-1].RequestID != "publish-123" {
		t.Fatalf("submission events = %+v, want request id publish-123", preview.Shein.SubmissionEvents)
	}
}

func TestSubmitTaskRemembersSheinResolutionCacheAfterPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	cacheStore := &submitResolutionCacheStore{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinCategoryResolver: sheinpub.NewCachedCategoryResolver(
			sheinpub.NewCategoryResolver(nil),
			cacheStore,
		),
		SheinAttributeResolver: sheinpub.NewCachedAttributeResolver(
			sheinpub.NewAttributeResolver(nil, nil),
			cacheStore,
		),
		SheinSaleAttributeResolver: sheinpub.NewCachedSaleAttributeResolver(
			sheinpub.NewSaleAttributeResolver(nil, nil),
			cacheStore,
		),
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "publish-cache-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if preview.Shein.ResolutionCache == nil ||
		preview.Shein.ResolutionCache.Category == nil ||
		preview.Shein.ResolutionCache.Attributes == nil ||
		preview.Shein.ResolutionCache.SaleAttributes == nil {
		t.Fatalf("resolution cache summary = %+v, want category/attribute/sale_attribute after publish", preview.Shein.ResolutionCache)
	}
	entries := cacheStore.snapshot()
	if len(entries) != 3 {
		t.Fatalf("cache entry count = %d, want 3: %+v", len(entries), entries)
	}
	for _, entry := range entries {
		if entry.Source != "manual_cache" || !entry.Manual {
			t.Fatalf("cache entry = %+v, want manual_cache confirmed by publish", entry)
		}
	}
}

func TestSubmitTaskConfirmsRemoteRecordAfterPublishSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var recordSupplierCodes []string
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordHook: func(request *sheinproduct.ProductRecordRequest) {
					if request.SupplierCodeList != nil {
						recordSupplierCodes = append(recordSupplierCodes, (*request.SupplierCodeList)...)
					}
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-123",
					SupplierCode: "SUP-submit-task-1",
					State:        2,
					AuditState:   3,
				}),
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "remote-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", got)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "record-123" {
		t.Fatalf("remote record id = %q, want record-123", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if len(recordSupplierCodes) != 1 || recordSupplierCodes[0] == "" {
		t.Fatalf("record supplier codes = %+v, want one supplier code", recordSupplierCodes)
	}
	if !repo.hasSavedSubmissionPhase(sheinpub.SubmissionPhaseConfirmRemote) {
		t.Fatalf("confirm_remote phase was not persisted; saved phases = %+v", repo.savedSubmissionPhases)
	}
}

func TestSubmitTaskMarksRemoteConfirmationPendingWhenRecordMissing(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(),
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "pending-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if got := preview.Shein.Submission.RemoteStatus; got != sheinpub.SubmissionRemoteStatusPending {
		t.Fatalf("remote status = %q, want pending", got)
	}
	if preview.Shein.Submission.LastStatus != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("last status = %q, want success", preview.Shein.Submission.LastStatus)
	}
}

func TestRefreshSubmissionStatusUpdatesRemoteRecordWithoutSubmitting(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	now := time.Now().Add(-time.Hour)
	task.Result.Shein.Submission = &sheinpub.SubmissionReport{
		LastAction:  "publish",
		LastStatus:  sheinpub.SubmissionStatusSuccess,
		SubmittedAt: &now,
		Publish: &sheinpub.SubmissionRecord{
			Action:       "publish",
			Status:       sheinpub.SubmissionStatusSuccess,
			SubmittedAt:  now,
			RequestID:    "refresh-123",
			SupplierCode: "SKC-1",
			StartedAt:    now,
			FinishedAt:   &now,
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	var recordCalls int32
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
				},
				recordHook: func(request *sheinproduct.ProductRecordRequest) {
					atomic.AddInt32(&recordCalls, 1)
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-refreshed",
					SupplierCode: "SKC-1",
					State:        4,
					AuditState:   5,
				}),
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.RefreshSubmissionStatus(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("refresh submission status: %v", err)
	}

	if got := atomic.LoadInt32(&publishCalls); got != 0 {
		t.Fatalf("publish calls = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&recordCalls); got != 1 {
		t.Fatalf("record calls = %d, want 1", got)
	}
	if preview.Shein.Submission.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", preview.Shein.Submission.RemoteStatus)
	}
	if preview.Shein.Submission.Publish.RemoteRecordID != "record-refreshed" {
		t.Fatalf("remote record id = %q, want record-refreshed", preview.Shein.Submission.Publish.RemoteRecordID)
	}
	if len(preview.Shein.SubmissionEvents) == 0 || preview.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhaseConfirmRemote {
		t.Fatalf("submission events = %+v, want confirm_remote event", preview.Shein.SubmissionEvents)
	}
}

func TestSubmitTaskRecoversRemoteSubmitAfterFinalSaveFailure(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{failSaveWhenCurrentPhaseCleared: true}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
				recordResponse: makeSheinRecordResponse(sheinproduct.RecordItem{
					RecordID:     "record-recovered",
					SupplierCode: "SUP-submit-task-1",
				}),
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, firstErr := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-123"})
	if firstErr == nil || !strings.Contains(firstErr.Error(), "save task result failed") {
		t.Fatalf("first submit err = %v, want save failure", firstErr)
	}
	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "recover-123"})
	if err != nil {
		t.Fatalf("recovery submit: %v", err)
	}

	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
	if preview.Shein.Submission.RemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", preview.Shein.Submission.RemoteStatus)
	}
	if preview.Shein.Submission.CurrentPhase != "" {
		t.Fatalf("current phase = %q, want cleared", preview.Shein.Submission.CurrentPhase)
	}
}

func TestSubmitTaskReplaysCompletedIdempotencyKeyWithoutPublishingAgain(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "replay-123"}); err != nil {
			t.Fatalf("submit task %d: %v", i+1, err)
		}
	}

	if publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", publishCalls)
	}
	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.Submission.AttemptCount != 1 {
		t.Fatalf("attempt count = %d, want 1", saved.Result.Shein.Submission.AttemptCount)
	}
}

func TestSubmitTaskReturnsCurrentPreviewForSameInFlightIdempotencyKey(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "in-flight-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "in-flight-123"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0", publishCalls)
	}
	if preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.CurrentPhase != sheinpub.SubmissionPhaseSubmitRemote {
		t.Fatalf("preview submission = %+v, want current submit_remote phase", preview.Shein)
	}
}

func TestSubmitTaskBlocksDifferentIdempotencyKeyWhileSubmitInFlight(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "in-flight-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "different-123"})

	if !errors.Is(err, ErrSubmitInProgress) {
		t.Fatalf("submit err = %v, want ErrSubmitInProgress", err)
	}
	if publishCalls != 0 {
		t.Fatalf("publish calls = %d, want 0", publishCalls)
	}
}

func TestSubmitTaskAllowsNewAttemptWhenInFlightAttemptIsStale(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-sheinSubmitInFlightTTL - time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "stale-123", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	publishCalls := 0
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalls++
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "new-123"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", publishCalls)
	}
	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.Submission.Publish == nil || saved.Result.Shein.Submission.Publish.RequestID != "new-123" {
		t.Fatalf("publish record = %+v, want new request id", saved.Result.Shein.Submission.Publish)
	}
}

func TestSubmitTaskPersistsSubmitRemotePhaseBeforePublishCall(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					if !repo.hasSavedSubmissionPhase(sheinpub.SubmissionPhaseSubmitRemote) {
						t.Fatalf("submit_remote phase was not persisted before publish call; saved phases = %+v", repo.savedSubmissionPhases)
					}
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "phase-123"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
}

func TestSubmitTaskSerializesConcurrentSameIdempotencyKey(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	enteredPublish := make(chan struct{}, 2)
	releasePublish := make(chan struct{})
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					atomic.AddInt32(&publishCalls, 1)
					enteredPublish <- struct{}{}
					<-releasePublish
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	errs := make(chan error, 2)
	go func() {
		_, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "concurrent-123"})
		errs <- err
	}()
	select {
	case <-enteredPublish:
	case <-time.After(time.Second):
		t.Fatal("first submit did not reach publish")
	}
	go func() {
		_, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "concurrent-123"})
		errs <- err
	}()
	time.Sleep(30 * time.Millisecond)
	close(releasePublish)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("submit %d error: %v", i+1, err)
		}
	}

	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
}

func TestSubmitTaskBlocksConcurrentDifferentRequestAcrossServiceInstances(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var publishCalls int32
	enteredPublish := make(chan struct{}, 1)
	releasePublish := make(chan struct{})
	productAPI := stubSheinProductAPI{
		publishHook: func(product *sheinproduct.Product) {
			atomic.AddInt32(&publishCalls, 1)
			enteredPublish <- struct{}{}
			<-releasePublish
		},
		publishResponse: &sheinproduct.SheinResponse{
			Code: "0",
			Msg:  "success",
			Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
		},
		recordResponse: makeSheinRecordResponse(),
	}
	newSvc := func() Service {
		svc, err := NewService(&ServiceConfig{
			Repository:             repo,
			ProductService:         stubSubmitProductService{},
			SheinProductAPIBuilder: stubSheinProductAPIBuilder{api: productAPI},
			SheinImageAPIBuilder:   stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
		})
		if err != nil {
			t.Fatalf("new service: %v", err)
		}
		return svc
	}
	svc1 := newSvc()
	svc2 := newSvc()
	errs := make(chan error, 2)
	go func() {
		_, err := svc1.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "request-a"})
		errs <- err
	}()
	select {
	case <-enteredPublish:
	case <-time.After(time.Second):
		t.Fatal("first submit did not reach publish")
	}
	go func() {
		_, err := svc2.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish", IdempotencyKey: "request-b"})
		errs <- err
	}()
	var conflict error
	select {
	case conflict = <-errs:
	case <-time.After(time.Second):
		t.Fatal("second submit did not return")
	}
	if !errors.Is(conflict, ErrSubmitInProgress) {
		t.Fatalf("second submit err = %v, want ErrSubmitInProgress", conflict)
	}
	close(releasePublish)
	if err := <-errs; err != nil {
		t.Fatalf("first submit err: %v", err)
	}
	if got := atomic.LoadInt32(&publishCalls); got != 1 {
		t.Fatalf("publish calls = %d, want 1", got)
	}
}

func TestSubmitTaskMarksPublishPreValidationNotesAsFailed(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
					Info: sheinproduct.ResponseInfo{
						Success: false,
						PreValidResult: []sheinproduct.PreValidResult{{
							Messages: []string{
								"数量: 类型下模板属性为必填项",
								"方形图必须有一个",
							},
						}},
					},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !strings.Contains(err.Error(), "数量: 类型下模板属性为必填项") {
		t.Fatalf("submit err = %v, want pre-validation note", err)
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	submission := saved.Result.Shein.Submission
	if submission == nil || submission.LastStatus != "failed" || !strings.Contains(submission.LastError, "方形图必须有一个") {
		t.Fatalf("submission = %+v", submission)
	}
	if submission.Publish == nil || submission.Publish.Result == nil || len(submission.Publish.Result.ValidationNotes) != 2 {
		t.Fatalf("publish result = %+v", submission.Publish)
	}
}

func TestSubmitTaskRebuildsNormalizedProductAttributesFromPackage(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	compositionValueID := 526
	materialValueID := 3473050
	task.Result.Shein.ResolvedAttributes = []SheinResolvedAttribute{
		{
			Name:             "Composition",
			Value:            "Polyester",
			AttributeID:      62,
			AttributeValueID: &compositionValueID,
			AttributeType:    3,
		},
		{
			Name:             "Material",
			Value:            "100%涤纶",
			AttributeID:      160,
			AttributeValueID: &materialValueID,
			AttributeType:    4,
		},
		{
			Name:             "Material",
			Value:            "Made with polyester",
			AttributeID:      160,
			AttributeValueID: &materialValueID,
			AttributeType:    4,
		},
	}
	task.Result.Shein.PreviewProduct.ProductAttributeList = []sheinproduct.ProductAttribute{
		{AttributeID: 160, AttributeValueID: &materialValueID},
		{AttributeID: 160, AttributeValueID: &materialValueID},
		{AttributeID: 62, AttributeValueID: &compositionValueID},
	}
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if len(submitted.ProductAttributeList) != 2 {
		t.Fatalf("submitted product attributes = %#v, want deduped composition+material", submitted.ProductAttributeList)
	}
	if submitted.ProductAttributeList[0].AttributeID != 62 || submitted.ProductAttributeList[0].AttributeExtraValue != "100" {
		t.Fatalf("submitted composition attribute = %#v, want extra value 100", submitted.ProductAttributeList[0])
	}
}

func TestSubmitTaskNormalizesLegacyStudioSupplierSKUs(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	oldSKU := "MG8014186001-D7E68190"
	sizeImage := "https://img.shein.com/uploaded/size-map.jpg"
	blackValueID := 2493
	whiteValueID := 2494
	sizeValueID := 267
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{StyleID: "D7E68190"},
		SDS: &SDSSyncOptions{
			ProductSKU: "MG8014186001",
			StyleID:    "D7E68190",
			Variants: []SDSSyncVariantOption{
				{VariantID: 101, Color: "black", Size: "均码"},
				{VariantID: 102, Color: "white", Size: "均码"},
			},
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: "https://img.shein.com/uploaded/default-main.jpg",
		Gallery: []string{
			"https://img.shein.com/uploaded/default-main.jpg",
			"https://img.shein.com/uploaded/default-gallery.jpg",
			sizeImage,
		},
	}
	task.Result.Shein.RequestDraft.SKCList = []SheinSKCRequestDraft{
		{
			SupplierCode: "BLACK",
			SaleAttribute: &SheinResolvedSaleAttribute{
				Scope: "skc", Name: "Color", Value: "black", AttributeID: 27, AttributeValueID: &blackValueID,
			},
			ImageInfo: &SheinImageDraft{MainImage: "https://img.shein.com/uploaded/black.jpg"},
			SKUList: []SheinSKUDraft{{
				SupplierSKU: oldSKU,
				Currency:    "USD",
				CostPrice:   "10.00",
				BasePrice:   "19.99",
				StockCount:  20,
				SitePriceList: []sheinpub.SitePrice{{
					SubSite: "US", BasePrice: "19.99", Currency: "USD",
				}},
				SaleAttributes: []SheinResolvedSaleAttribute{{
					Scope: "sku", Name: "Size", Value: "均码", AttributeID: 87, AttributeValueID: &sizeValueID,
				}},
				Attributes: map[string]string{
					"Color": "black",
					"Size":  "均码",
				},
			}},
		},
		{
			SupplierCode: "WHITE",
			SaleAttribute: &SheinResolvedSaleAttribute{
				Scope: "skc", Name: "Color", Value: "white", AttributeID: 27, AttributeValueID: &whiteValueID,
			},
			ImageInfo: &SheinImageDraft{MainImage: "https://img.shein.com/uploaded/white.jpg"},
			SKUList: []SheinSKUDraft{{
				SupplierSKU: oldSKU,
				Currency:    "USD",
				CostPrice:   "11.00",
				BasePrice:   "21.99",
				StockCount:  20,
				SitePriceList: []sheinpub.SitePrice{{
					SubSite: "US", BasePrice: "21.99", Currency: "USD",
				}},
				SaleAttributes: []SheinResolvedSaleAttribute{{
					Scope: "sku", Name: "Size", Value: "均码", AttributeID: 87, AttributeValueID: &sizeValueID,
				}},
				Attributes: map[string]string{
					"Color": "white",
					"Size":  "均码",
				},
			}},
		},
	}
	task.Result.Shein.SkcList = []SheinSKCPackage{
		{SupplierCode: "BLACK", SkcName: "black", SaleName: "black", MainImageURL: "https://img.shein.com/uploaded/black.jpg", SKUs: []common.Variant{{SKU: oldSKU, Attributes: map[string]string{"Color": "black", "Size": "均码"}}}},
		{SupplierCode: "WHITE", SkcName: "white", SaleName: "white", MainImageURL: "https://img.shein.com/uploaded/white.jpg", SKUs: []common.Variant{{SKU: oldSKU, Attributes: map[string]string{"Color": "white", "Size": "均码"}}}},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{
		"https://img.shein.com/uploaded/default-main.jpg",
		"https://img.shein.com/uploaded/default-gallery.jpg",
		sizeImage,
	})
	task.Result.Shein.PreviewProduct.SKCList = []sheinproduct.SKC{
		{
			SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 27, AttributeValueID: blackValueID},
			ImageInfo:     sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{{ImageType: 1, ImageSort: 1, ImageURL: "https://img.shein.com/uploaded/black.jpg"}}},
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: oldSKU,
				CostInfo:    &sheinproduct.CostInfo{CostPrice: "10.00", Currency: "USD"},
				PriceInfoList: []sheinproduct.PriceInfo{{
					SubSite: "US", BasePrice: 19.99, Currency: "USD",
				}},
				StockInfoList:     []sheinproduct.StockInfo{{MerchantWarehouseCode: "US", InventoryNum: 20}},
				SaleAttributeList: []sheinproduct.SaleAttribute{{AttributeID: 87, AttributeValueID: sizeValueID}},
			}},
		},
		{
			SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 27, AttributeValueID: whiteValueID},
			ImageInfo:     sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{{ImageType: 1, ImageSort: 1, ImageURL: "https://img.shein.com/uploaded/white.jpg"}}},
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: oldSKU,
				CostInfo:    &sheinproduct.CostInfo{CostPrice: "11.00", Currency: "USD"},
				PriceInfoList: []sheinproduct.PriceInfo{{
					SubSite: "US", BasePrice: 21.99, Currency: "USD",
				}},
				StockInfoList:     []sheinproduct.StockInfo{{MerchantWarehouseCode: "US", InventoryNum: 20}},
				SaleAttributeList: []sheinproduct.SaleAttribute{{AttributeID: 87, AttributeValueID: sizeValueID}},
			}},
		},
	}
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    "https://img.shein.com/uploaded/default-main.jpg",
		FinalImageOrder: []string{"https://img.shein.com/uploaded/default-main.jpg", "https://img.shein.com/uploaded/default-gallery.jpg", sizeImage},
		ImageRoleOverrides: map[string]string{
			sizeImage: "size_map",
		},
		ManualPriceOverrides: map[string]float64{oldSKU: 25.55},
	}
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		Ready:           true,
		ManualOverrides: map[string]float64{oldSKU: 25.55},
		SKUPrices: []sheinpub.SKUPriceReview{
			{SupplierSKU: oldSKU, FinalPrice: 25.55, Currency: "USD"},
			{SupplierSKU: oldSKU, FinalPrice: 25.55, Currency: "USD"},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var submitted *sheinproduct.Product
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	got := []string{
		submitted.SKCList[0].SKUS[0].SupplierSKU,
		submitted.SKCList[1].SKUS[0].SupplierSKU,
	}
	want := []string{
		"MG8014186001-V101-TSUBMITTA-D7E68190",
		"MG8014186001-V102-TSUBMITTA-D7E68190",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("submitted supplier skus = %#v, want %#v", got, want)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.FinalDraft == nil {
		t.Fatalf("saved shein final draft = %+v", saved.Result)
	}
	overrides := saved.Result.Shein.FinalDraft.ManualPriceOverrides
	if len(overrides) != 2 || overrides[want[0]] != 25.55 || overrides[want[1]] != 25.55 {
		t.Fatalf("final draft overrides = %#v, want fan-out to both new skus", overrides)
	}
	if _, exists := overrides[oldSKU]; exists {
		t.Fatalf("final draft overrides still contains legacy sku %q", oldSKU)
	}
	if saved.Result.Shein.Pricing == nil || len(saved.Result.Shein.Pricing.SKUPrices) != 2 {
		t.Fatalf("saved pricing = %+v", saved.Result.Shein.Pricing)
	}
	if saved.Result.Shein.Pricing.SKUPrices[0].SupplierSKU != want[0] || saved.Result.Shein.Pricing.SKUPrices[1].SupplierSKU != want[1] {
		t.Fatalf("pricing sku prices = %#v, want normalized sku order", saved.Result.Shein.Pricing.SKUPrices)
	}
}

func TestSubmitTaskNormalizesSingleStudioSupplierSKUWithTaskDiscriminator(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.ID = "fe7413d2-ac75-4c97-be0f-800a40dffa00"
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{StyleID: "D7E68190"},
		SDS: &SDSSyncOptions{
			ProductSKU:   "MG8014186001",
			VariantSKU:   "MG8014186001",
			StyleID:      "D7E68190",
			VariantColor: "black",
			VariantSize:  "均码",
		},
	}
	task.Result.TaskID = task.ID
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU = "MG8014186001-D7E68190"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].Attributes = map[string]string{
		"Color":          "black",
		"Size":           "均码",
		"source_sds_sku": "MG8014186001",
	}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU = "MG8014186001-D7E68190"
	task.Result.Shein.SkcList[0].SKUs[0].SKU = "MG8014186001-D7E68190"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:            true,
		MainImageURL:         "https://img.shein.com/uploaded/default-main.jpg",
		FinalImageOrder:      []string{"https://img.shein.com/uploaded/default-main.jpg"},
		ManualPriceOverrides: map[string]float64{"MG8014186001-D7E68190": 13.56},
	}
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		Ready:           true,
		ManualOverrides: map[string]float64{"MG8014186001-D7E68190": 13.56},
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: "MG8014186001-D7E68190",
			FinalPrice:  13.56,
			Currency:    "USD",
		}},
	}
	pkg := &sheinpub.Package{
		RequestDraft:   task.Result.Shein.RequestDraft,
		PreviewProduct: task.Result.Shein.PreviewProduct,
		SkcList:        task.Result.Shein.SkcList,
		FinalDraft:     task.Result.Shein.FinalDraft,
		Pricing:        task.Result.Shein.Pricing,
	}
	changed := normalizeSheinStudioSubmitSupplierSKUs(task, pkg)
	if !changed {
		t.Fatal("expected single-variant supplier sku normalization to change payload")
	}
	wantSKU := "MG8014186001-BLACK-V1-TFE7413D2-D7E68190"
	if got := pkg.RequestDraft.SKCList[0].SKUList[0].SupplierSKU; got != wantSKU {
		t.Fatalf("request draft supplier sku = %q, want %q", got, wantSKU)
	}
	if got := pkg.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU; got != wantSKU {
		t.Fatalf("preview supplier sku = %q, want %q", got, wantSKU)
	}
	if got := pkg.SkcList[0].SKUs[0].SKU; got != wantSKU {
		t.Fatalf("package supplier sku = %q, want %q", got, wantSKU)
	}
	if pkg.FinalDraft.ManualPriceOverrides[wantSKU] != 13.56 {
		t.Fatalf("final draft overrides = %#v, want remapped key %q", pkg.FinalDraft.ManualPriceOverrides, wantSKU)
	}
	if _, exists := pkg.FinalDraft.ManualPriceOverrides["MG8014186001-D7E68190"]; exists {
		t.Fatalf("final draft overrides still contains legacy sku")
	}
	if len(pkg.Pricing.SKUPrices) != 1 || pkg.Pricing.SKUPrices[0].SupplierSKU != wantSKU {
		t.Fatalf("pricing sku prices = %#v, want remapped single sku", pkg.Pricing.SKUPrices)
	}
}

func TestSubmitTaskMarksSaveDraftCodeZeroAsSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SPUName = "Draft Display Title Should Not Be Submitted"
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
					Info: sheinproduct.ResponseInfo{
						SPUName: "SPU-123",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected save draft payload to be captured")
	}
	if submitted.SPUName != "" {
		t.Fatalf("submitted draft spu_name = %q, want empty for new SHEIN product", submitted.SPUName)
	}
	if preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submission = %+v", preview.Shein)
	}
}

func TestSubmitTaskReappliesReadyPricingBeforeSubmit(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		RuleSnapshot: &sheinpub.PricingRule{
			SourceCurrency: "CNY",
			TargetCurrency: "USD",
		},
		Ready: true,
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU,
			FinalPrice:  25.55,
			Currency:    "USD",
		}},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "19.99"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "19.99",
		Currency:  "USD",
	}}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList = []sheinproduct.PriceInfo{{
		SubSite:   "US",
		BasePrice: 19.99,
		Currency:  "USD",
	}}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].CostInfo = &sheinproduct.CostInfo{
		CostPrice: "73.8",
		Currency:  "USD",
	}

	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected save draft payload to be captured")
	}
	if len(submitted.SKCList) == 0 || len(submitted.SKCList[0].SKUS) == 0 || len(submitted.SKCList[0].SKUS[0].PriceInfoList) == 0 {
		t.Fatalf("submitted price info = %+v", submitted.SKCList)
	}
	if submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice != 25.55 {
		t.Fatalf("submitted base price = %v, want 25.55", submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice)
	}
	if submitted.SKCList[0].SKUS[0].CostInfo == nil || submitted.SKCList[0].SKUS[0].CostInfo.Currency != "USD" {
		t.Fatalf("submitted cost info = %+v, want currency USD", submitted.SKCList[0].SKUS[0].CostInfo)
	}
	if submitted.SKCList[0].SKUS[0].CostInfo.CostPrice != "10.25" {
		t.Fatalf("submitted cost price = %q, want 10.25", submitted.SKCList[0].SKUS[0].CostInfo.CostPrice)
	}
}

func TestSubmitTaskSaveDraftDoesNotFailWhenContentOptimizerFails(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var submitted *sheinproduct.Product
	contentAI := &stubSheinContentAI{err: errors.New("upstream EOF")}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		},
		SheinContentOptimizer: contentAI,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected save draft payload to be captured")
	}
	if contentAI.calls == 0 {
		t.Fatal("expected content optimizer to be attempted")
	}
	if preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submission = %+v", preview.Shein)
	}
}

func TestSubmitTaskNormalizesSheinPublishOnlyFields(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SiteList = nil
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{
		"https://cdn.example.com/main.jpg",
		"https://cdn.example.com/gallery-1.jpg",
	})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{
		"https://cdn.example.com/main.jpg",
		"https://cdn.example.com/gallery-1.jpg",
		"https://cdn.example.com/gallery-2.jpg",
		"https://cdn.example.com/gallery-3.jpg",
	})
	sku := &task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0]
	sku.ImageInfo = sheinImageInfo([]string{"https://cdn.example.com/main.jpg"})
	sku.Length = ""
	sku.Width = ""
	sku.Height = ""
	sku.LengthUnit = ""
	sku.StockInfoList = nil
	stockCount := 999
	sku.StockCount = &stockCount
	sku.QuantityInfo = nil
	sku.PackageType = 0
	sku.PriceInfoList = []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 19.99, Currency: "USD"}}
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if submitted.ImageInfo == nil || len(submitted.ImageInfo.ImageInfoList) != 0 {
		t.Fatalf("product image_info = %+v, want empty for publish payload", submitted.ImageInfo)
	}
	if len(submitted.SKCList[0].ImageInfo.ImageInfoList) != 6 {
		t.Fatalf("skc image count = %d, want 6", len(submitted.SKCList[0].ImageInfo.ImageInfoList))
	}
	if submitted.SKCList[0].ImageInfo.ImageInfoList[4].ImageType != 5 {
		t.Fatalf("skc square image type = %d, want 5", submitted.SKCList[0].ImageInfo.ImageInfoList[4].ImageType)
	}
	if submitted.SKCList[0].ImageInfo.ImageInfoList[5].ImageType != 6 {
		t.Fatalf("skc color block image type = %d, want 6", submitted.SKCList[0].ImageInfo.ImageInfoList[5].ImageType)
	}
	if submitted.SKCList[0].SKUS[0].ImageInfo == nil || len(submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList) != 0 {
		t.Fatalf("sku image_info = %+v, want empty for publish payload", submitted.SKCList[0].SKUS[0].ImageInfo)
	}
	if len(submitted.SiteList) != 1 || submitted.SiteList[0].MainSite != "shein" || len(submitted.SiteList[0].SubSiteList) != 1 || submitted.SiteList[0].SubSiteList[0] != "shein-us" {
		t.Fatalf("site_list = %+v, want shein/shein-us", submitted.SiteList)
	}
	submittedSKU := submitted.SKCList[0].SKUS[0]
	if len(submittedSKU.StockInfoList) != 1 || submittedSKU.StockInfoList[0].MerchantWarehouseCode != defaultSheinWarehouseCode || submittedSKU.StockInfoList[0].InventoryNum != 999 {
		t.Fatalf("stock_info_list = %+v", submittedSKU.StockInfoList)
	}
	if submittedSKU.StockCount != nil {
		t.Fatalf("stock_count = %v, want nil when stock_info_list is populated", *submittedSKU.StockCount)
	}
	if submittedSKU.QuantityInfo == nil || submittedSKU.QuantityInfo.Quantity == nil || *submittedSKU.QuantityInfo.Quantity != 1 ||
		submittedSKU.QuantityInfo.QuantityType == nil || *submittedSKU.QuantityInfo.QuantityType != 1 ||
		submittedSKU.QuantityInfo.QuantityUnit == nil || *submittedSKU.QuantityInfo.QuantityUnit != 1 {
		t.Fatalf("quantity_info = %+v, want 1/1/1", submittedSKU.QuantityInfo)
	}
	if submittedSKU.PackageType != 3 {
		t.Fatalf("package_type = %d, want 3", submittedSKU.PackageType)
	}
	if len(submittedSKU.PriceInfoList) != 1 || submittedSKU.PriceInfoList[0].SubSite != "shein-us" {
		t.Fatalf("price_info_list = %+v, want sub_site shein-us", submittedSKU.PriceInfoList)
	}
	if submittedSKU.Length == "" || submittedSKU.Width == "" || submittedSKU.Height == "" || submittedSKU.LengthUnit == "" {
		t.Fatalf("dimensions not normalized: length=%q width=%q height=%q unit=%q", submittedSKU.Length, submittedSKU.Width, submittedSKU.Height, submittedSKU.LengthUnit)
	}
	if submittedSKU.WeightUnit != "g" {
		t.Fatalf("weight_unit = %q, want g", submittedSKU.WeightUnit)
	}
}

func TestSubmitTaskNormalizesSheinWeightToGrams(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].Weight = 0.35
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].WeightUnit = "kg"
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{
						Success: true,
					},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	submittedSKU := submitted.SKCList[0].SKUS[0]
	if submittedSKU.Weight != 350 {
		t.Fatalf("weight = %v, want 350g", submittedSKU.Weight)
	}
	if submittedSKU.WeightUnit != "g" {
		t.Fatalf("weight_unit = %q, want g", submittedSKU.WeightUnit)
	}
}

func TestSubmitTaskPublishesSDSRenderedImages(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	rendered := []string{
		"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-1.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-2.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-3.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-4.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-5.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-6.jpg",
	}
	task.Result.SDSSync = &SDSSyncSummary{
		Status:          "completed",
		MockupImageURLs: rendered,
	}
	task.Result.Shein.Images = &PlatformImageSet{
		MainImage:    rendered[0],
		Gallery:      append([]string(nil), rendered[1:]...),
		SourceImages: []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: rendered[0],
		Gallery:   append([]string(nil), rendered[1:]...),
		Source:    []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].ImageInfo = &SheinImageDraft{
		MainImage: rendered[0],
		Gallery:   append([]string(nil), rendered[1:]...),
		Source:    []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].MainImage = rendered[0]
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo(rendered[:1])
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	uploaded := make([]string, 0, len(rendered))
	uploadMap := map[string]string{}
	for index, url := range rendered {
		uploadedURL := fmt.Sprintf("https://img.shein.com/uploaded/rendered-%d.jpg", index)
		uploaded = append(uploaded, uploadedURL)
		uploadMap[url] = uploadedURL
	}
	imageAPI := &stubSheinImageAPI{uploaded: uploadMap}
	var submitted *sheinproduct.Product
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{
						Success: true,
						SPUName: "SPU-123",
						Version: "v1",
					},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: imageAPI},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if submitted.ImageInfo == nil || len(submitted.ImageInfo.ImageInfoList) != 0 {
		t.Fatalf("submitted SPU image info = %+v, want empty for publish payload", submitted.ImageInfo)
	}
	expectedSKCImages := append([]string(nil), uploaded...)
	expectedSKCImages = append(expectedSKCImages, uploaded[0])
	expectedSKCImages = append(expectedSKCImages, uploaded[0])
	if len(submitted.SKCList) != 1 || len(submitted.SKCList[0].ImageInfo.ImageInfoList) != len(expectedSKCImages) {
		t.Fatalf("submitted SKC image info = %+v", submitted.SKCList)
	}
	for index, image := range submitted.SKCList[0].ImageInfo.ImageInfoList {
		if image.ImageURL != expectedSKCImages[index] {
			t.Fatalf("submitted SKC image %d = %q, want uploaded %q", index, image.ImageURL, expectedSKCImages[index])
		}
		wantType := 2
		if index == 0 {
			wantType = 1
		}
		if index == len(expectedSKCImages)-1 {
			wantType = 6
		} else if index == len(expectedSKCImages)-2 {
			wantType = 5
		}
		if image.ImageType != wantType {
			t.Fatalf("submitted SKC image %d type = %d, want %d", index, image.ImageType, wantType)
		}
		if image.ImageURL == sourceImage {
			t.Fatalf("submitted SKC image still uses source image: %q", sourceImage)
		}
	}
	if len(submitted.SKCList[0].SKUS) != 1 || submitted.SKCList[0].SKUS[0].ImageInfo == nil || len(submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList) != 0 {
		t.Fatalf("submitted SKU image info = %+v, want empty for publish payload", submitted.SKCList[0].SKUS)
	}
	if len(imageAPI.calls) != len(rendered) {
		t.Fatalf("upload calls = %+v, want %d unique uploads", imageAPI.calls, len(rendered))
	}
	for _, url := range rendered {
		if imageAPI.calls[url] != 1 {
			t.Fatalf("upload call count for %q = %d, want 1", url, imageAPI.calls[url])
		}
	}
}

func TestBuildSheinImageUploadPreflightCountsUniqueSDSImages(t *testing.T) {
	t.Parallel()

	rendered := []string{
		"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-1.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-2.jpg",
	}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo(rendered[:1])

	report := buildSheinImageUploadPreflight(task.Result.Shein)
	if report == nil {
		t.Fatal("expected image upload preflight")
	}
	if report.TotalImageReferences != 7 {
		t.Fatalf("total references = %d, want 7", report.TotalImageReferences)
	}
	if report.UniqueImageURLs != len(rendered) {
		t.Fatalf("unique urls = %d, want %d", report.UniqueImageURLs, len(rendered))
	}
	if report.PendingUploadURLs != len(rendered) {
		t.Fatalf("pending upload urls = %d, want %d", report.PendingUploadURLs, len(rendered))
	}
	if !report.UsesSDSMockups || report.SDSMockupURLs != len(rendered) {
		t.Fatalf("sds mockup report = %+v", report)
	}
}

func TestSubmitTaskBlocksPublishWhenSheinImageUploadFails(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	rendered := []string{"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg"}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo(rendered)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	publishCalled := false
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalled = true
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{
			api: &stubSheinImageAPI{err: errors.New("upload rejected")},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !strings.Contains(err.Error(), "upload rejected") {
		t.Fatalf("submit err = %v, want upload rejected", err)
	}
	if publishCalled {
		t.Fatal("publish should not be called when image upload fails")
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("submission was not persisted: %+v", saved.Result)
	}
	if saved.Result.Shein.Submission.LastStatus != "failed" || !strings.Contains(saved.Result.Shein.Submission.LastError, "upload rejected") {
		t.Fatalf("submission failure = %+v", saved.Result.Shein.Submission)
	}
}

func TestSubmitTaskReusesSheinImageUploadCache(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo([]string{sourceImage})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	imageAPI := &stubSheinImageAPI{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "OK"},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: imageAPI},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft", ConfirmedFinal: true})
		if err != nil {
			t.Fatalf("submit task %d: %v", i+1, err)
		}
	}
	if imageAPI.calls[sourceImage] != 1 {
		t.Fatalf("upload calls for source image = %d, want 1", imageAPI.calls[sourceImage])
	}
	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	cache := saved.Result.Shein.FinalDraft.SheinImageUploadCache
	if cache[sourceImage] == "" || !isSheinUploadedImageURL(cache[sourceImage]) {
		t.Fatalf("upload cache = %+v, want shein uploaded url for source", cache)
	}
}

func TestSubmitTaskSaveDraftAllowsMissingStrictPublishImageRoles(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    sourceImage,
		FinalImageOrder: []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{sourceImage})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "OK"},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"})
	if err != nil {
		t.Fatalf("save draft should allow missing strict publish image roles: %v", err)
	}
}

func TestSubmitTaskPublishAllowsMissingSizeMapRole(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    sourceImage,
		FinalImageOrder: []string{sourceImage},
		ImageRoleOverrides: map[string]string{
			sourceImage: "swatch",
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{sourceImage})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	publishCalled := false
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalled = true
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("publish err = %v, want missing size map to be non-blocking", err)
	}
	readiness := buildSheinSubmitReadinessForAction(task.Result.Shein, "publish")
	foundSizeMapBlocker := false
	if readiness != nil {
		for _, item := range readiness.BlockingItems {
			if strings.Contains(item.Message, "尺寸图") {
				foundSizeMapBlocker = true
				break
			}
		}
	}
	if foundSizeMapBlocker {
		t.Fatalf("publish readiness = %+v, want no size map blocker", readiness)
	}
	if !publishCalled {
		t.Fatal("publish should be called when only size map is missing")
	}
}

func TestSubmitTaskPublishRepairsMissingSKCImagesFromFinalDraft(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	sizeImage := "https://oss.shuomiai.com/listingkit/source-size.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    sourceImage,
		FinalImageOrder: []string{sourceImage, sizeImage},
		ImageRoleOverrides: map[string]string{
			sizeImage: "size_map",
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage, sizeImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].ImageInfo = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].MainImage = sourceImage
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage, sizeImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = sheinproduct.ImageInfo{}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var submitted *sheinproduct.Product
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if len(submitted.SKCList) == 0 || len(submitted.SKCList[0].ImageInfo.ImageInfoList) == 0 {
		t.Fatalf("submitted skc images = %+v, want repaired images", submitted.SKCList)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.RequestDraft.SKCList[0].ImageInfo == nil || strings.TrimSpace(saved.Result.Shein.RequestDraft.SKCList[0].ImageInfo.MainImage) == "" {
		t.Fatalf("saved request skc image = %+v, want repaired main image", saved.Result.Shein.RequestDraft.SKCList[0].ImageInfo)
	}
	if len(saved.Result.Shein.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList) == 0 {
		t.Fatalf("saved preview skc images = %+v, want repaired images", saved.Result.Shein.PreviewProduct.SKCList[0].ImageInfo)
	}
}

func TestSubmitTaskBlocksSharedSingleImageAcrossMultipleSKCs(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{},
	}
	mainImage := "https://oss.shuomiai.com/listingkit/shared-main.png"
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: mainImage,
		Gallery:   []string{mainImage, "https://oss.shuomiai.com/listingkit/size-map.png"},
	}
	task.Result.Shein.RequestDraft.SKCList = []sheinpub.SKCRequestDraft{
		{
			SkcName:      "black",
			SaleName:     "black",
			SupplierCode: "BLACK",
			ImageInfo:    &SheinImageDraft{MainImage: mainImage},
			SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "BLACK-20OZ", MainImage: mainImage, Attributes: map[string]string{"Color": "black"}}},
		},
		{
			SkcName:      "gray",
			SaleName:     "gray",
			SupplierCode: "GRAY",
			ImageInfo:    &SheinImageDraft{MainImage: mainImage},
			SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "GRAY-20OZ", MainImage: mainImage, Attributes: map[string]string{"Color": "gray"}}},
		},
		{
			SkcName:      "Pale pink",
			SaleName:     "Pale pink",
			SupplierCode: "PALE-PINK",
			ImageInfo:    &SheinImageDraft{MainImage: mainImage},
			SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "PALE-PINK-20OZ", MainImage: mainImage, Attributes: map[string]string{"Color": "Pale pink"}}},
		},
	}
	task.Result.Shein.SkcList = []sheinpub.SKCPackage{
		{SkcName: "black", SaleName: "black", SupplierCode: "BLACK", MainImageURL: mainImage},
		{SkcName: "gray", SaleName: "gray", SupplierCode: "GRAY", MainImageURL: mainImage},
		{SkcName: "Pale pink", SaleName: "Pale pink", SupplierCode: "PALE-PINK", MainImageURL: mainImage},
	}
	task.Result.Shein.PreviewProduct = sheinpub.BuildPreviewProduct(task.Result.Shein)
	task.Result.SDSSync = &SDSSyncSummary{
		Status: "failed",
		Error:  "SDS render failed for selected color variants: gray, Pale pink",
		VariantResults: []SDSSyncSummary{
			{VariantColor: "black", Status: "completed", MockupImageURLs: []string{mainImage}},
			{VariantColor: "gray", Status: "failed"},
			{VariantColor: "Pale pink", Status: "failed"},
		},
	}
	task.Result.Summary = &GenerationSummary{}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	publishCalled := false
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalled = true
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submit err = %v, want readiness block", err)
	}
	if publishCalled {
		t.Fatal("publish should not be called when variant image coverage is incomplete")
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Summary == nil || !saved.Result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs review", saved.Result.Summary)
	}
	if len(saved.Result.ReviewReasons) == 0 || !strings.Contains(saved.Result.ReviewReasons[0], "gray, Pale pink") {
		t.Fatalf("review reasons = %#v, want failed variant reason", saved.Result.ReviewReasons)
	}
	for _, skc := range saved.Result.Shein.RequestDraft.SKCList {
		if skc.ImageInfo == nil || strings.TrimSpace(skc.ImageInfo.MainImage) == "" {
			t.Fatalf("skc image info = %+v, want preserved shared images", skc.ImageInfo)
		}
	}
	if saved.Result.Shein.Metadata[sheinVariantImageCoverageStatusKey] != "blocked" {
		t.Fatalf("metadata = %#v, want blocked variant image coverage status", saved.Result.Shein.Metadata)
	}
}

func TestSubmitReadinessDerivesSwatchFromSKCImage(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	mainImage := "https://oss.shuomiai.com/listingkit/main.png"
	sizeImage := "https://oss.shuomiai.com/listingkit/size.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    mainImage,
		FinalImageOrder: []string{mainImage, sizeImage},
		ImageRoleOverrides: map[string]string{
			sizeImage: "size_map",
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: mainImage,
		Gallery:   []string{mainImage, sizeImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].ImageInfo = &SheinImageDraft{
		MainImage: mainImage,
		Gallery:   []string{mainImage, sizeImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{mainImage, sizeImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{mainImage, sizeImage})

	readiness := buildSheinSubmitReadinessForAction(task.Result.Shein, "publish")
	if readiness == nil || !readiness.Ready {
		t.Fatalf("readiness = %+v, want ready because submit derives swatch from SKC image", readiness)
	}
}

func TestSubmitTaskTranslatesChineseSheinContentBeforePublish(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Request.Country = "US"
	task.Result.Shein.PreviewProduct.MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "啤酒盖铁板画"}}
	task.Result.Shein.PreviewProduct.MultiLanguageDescList = []sheinproduct.LanguageContent{{Language: "en", Name: "适用于酒吧和车库装饰。"}}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: "白色"}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "白色"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	var submitted *sheinproduct.Product
	translateAPI := &stubSheinTranslateAPI{}
	contentAI := &stubSheinContentAI{
		response: `{"title":"Optimized Bottle Cap Metal Sign for Bar and Garage Decor","description":"A durable decorative metal sign designed for wall display in bars, garages, game rooms, and home spaces."}`,
	}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "success", Info: sheinproduct.ResponseInfo{Success: true}},
			},
		},
		SheinTranslateAPIBuilder: stubSheinTranslateAPIBuilder{api: translateAPI},
		SheinContentOptimizer:    contentAI,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageNameList, "en"); got != "Optimized Bottle Cap Metal Sign for Bar and Garage Decor" {
		t.Fatalf("english product name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageNameList, "es"); got != "Spanish Optimized Bottle Cap Metal Sign for Bar and Garage Decor" {
		t.Fatalf("spanish product name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageDescList, "en"); got != "A durable decorative metal sign designed for wall display in bars, garages, game rooms, and home spaces." {
		t.Fatalf("english product description = %q", got)
	}
	if got := submitted.SKCList[0].MultiLanguageName.Name; got != "English 白色" {
		t.Fatalf("skc primary name = %q", got)
	}
	if len(translateAPI.calls) == 0 {
		t.Fatal("expected translate API to be called")
	}
	if contentAI.calls == 0 {
		t.Fatal("expected content optimizer to be called")
	}
}

func sheinImageInfo(urls []string) *sheinproduct.ImageInfo {
	info := &sheinproduct.ImageInfo{
		ImageInfoList: make([]sheinproduct.ImageDetail, 0, len(urls)),
	}
	for index, url := range urls {
		imageType := 2
		if index == 0 {
			imageType = 1
		}
		info.ImageInfoList = append(info.ImageInfoList, sheinproduct.ImageDetail{
			ImageType:          imageType,
			ImageSort:          index + 1,
			ImageURL:           url,
			MarketingMainImage: index == 0,
		})
	}
	return info
}
