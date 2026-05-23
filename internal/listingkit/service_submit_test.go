package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	failSaveOnCurrentPhase          string
	saveFailed                      bool
	saveCalls                       int
	mutateCalls                     int
}

func (r *stubSubmitRepo) CreateTask(ctx context.Context, task *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied, err := cloneSubmitTestTask(task)
	if err != nil {
		return err
	}
	r.task = &copied
	return nil
}

func (r *stubSubmitRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied, err := cloneSubmitTestTask(r.task)
	if err != nil {
		return nil, err
	}
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
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Status = TaskStatusCompleted
	r.task.Error = ""
	r.task.UpdatedAt = time.Now()
	return nil
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
	r.saveCalls++
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	if r.failSaveWhenCurrentPhaseCleared && !r.saveFailed && result != nil && result.Shein != nil && result.Shein.Submission != nil && result.Shein.Submission.CurrentPhase == "" {
		r.saveFailed = true
		return errors.New("save task result failed")
	}
	if r.failSaveOnCurrentPhase != "" && !r.saveFailed && result != nil && result.Shein != nil && result.Shein.Submission != nil && result.Shein.Submission.CurrentPhase == r.failSaveOnCurrentPhase {
		r.saveFailed = true
		return errors.New("save task result failed")
	}
	clonedResult, err := cloneListingKitResult(result)
	if err != nil {
		return err
	}
	r.task.Result = clonedResult
	r.task.UpdatedAt = time.Now()
	if clonedResult != nil && clonedResult.Shein != nil && clonedResult.Shein.Submission != nil {
		r.savedSubmissionPhases = append(r.savedSubmissionPhases, clonedResult.Shein.Submission.CurrentPhase)
	}
	return nil
}

func (r *stubSubmitRepo) MutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mutateCalls++
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied, err := cloneSubmitTestTask(r.task)
	if err != nil {
		return nil, err
	}
	out := &copied
	if mutate != nil {
		if err := mutate(r.task); err != nil {
			return out, err
		}
	}
	r.task.UpdatedAt = time.Now()
	copied, err = cloneSubmitTestTask(r.task)
	if err != nil {
		return nil, err
	}
	return &copied, nil
}

func cloneSubmitTestTask(task *Task) (Task, error) {
	if task == nil {
		return Task{}, nil
	}
	copied := *task
	if task.Result != nil {
		result, err := cloneListingKitResult(task.Result)
		if err != nil {
			return Task{}, err
		}
		copied.Result = result
	}
	return copied, nil
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

type stubSheinPublishWorkflowClient struct {
	startCalls int
	lastStart  SheinPublishWorkflowStartInput
	startErr   error
}

func (s *stubSheinPublishWorkflowClient) StartSheinPublish(ctx context.Context, in SheinPublishWorkflowStartInput) error {
	s.startCalls++
	s.lastStart = in
	return s.startErr
}

func (s *stubSheinPublishWorkflowClient) QuerySheinPublishState(ctx context.Context, taskID string) (*SheinPublishWorkflowState, error) {
	return nil, nil
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
	api         sheinproduct.ProductAPI
	msg         string
	lastStoreID *int64
}

func (s stubSheinProductAPIBuilder) BuildProductAPI(_ context.Context, storeID int64) (sheinproduct.ProductAPI, string) {
	if s.lastStoreID != nil {
		*s.lastStoreID = storeID
	}
	return s.api, s.msg
}

type stubSheinImageAPIBuilder struct {
	api         sheinimage.ImageAPI
	msg         string
	lastStoreID *int64
}

func (s stubSheinImageAPIBuilder) BuildImageAPI(_ context.Context, storeID int64) (sheinimage.ImageAPI, string) {
	if s.lastStoreID != nil {
		*s.lastStoreID = storeID
	}
	return s.api, s.msg
}

type stubSheinTranslateAPIBuilder struct {
	api         sheintranslateapi.TranslateAPI
	msg         string
	lastStoreID *int64
}

func (s stubSheinTranslateAPIBuilder) BuildTranslateAPI(_ context.Context, storeID int64) (sheintranslateapi.TranslateAPI, string) {
	if s.lastStoreID != nil {
		*s.lastStoreID = storeID
	}
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
	publishFunc     func(*sheinproduct.Product) (*sheinproduct.SheinResponse, error)
	saveResponse    *sheinproduct.SheinResponse
	saveErr         error
	saveHook        func(*sheinproduct.Product)
	confirmNeed     bool
	confirmErr      error
	confirmHook     func(*sheinproduct.Product)
	recordResponse  *sheinproduct.RecordResponse
	recordErr       error
	recordHook      func(*sheinproduct.ProductRecordRequest)
	inventoryResp   *sheinproduct.InventoryQueryResponse
	inventoryErr    error
	inventoryHook   func(string)
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
	if s.publishFunc != nil {
		resp, err := s.publishFunc(prod)
		return resp, "", err
	}
	return s.publishResponse, "", s.publishErr
}
func (s stubSheinProductAPI) ConfirmPublish(product *sheinproduct.Product) (bool, string, error) {
	if s.confirmHook != nil {
		s.confirmHook(product)
	}
	return s.confirmNeed, "", s.confirmErr
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
	if s.inventoryHook != nil {
		s.inventoryHook(spuName)
	}
	return s.inventoryResp, s.inventoryErr
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

func TestBuildSheinSubmitProductAPIUsesResolvedProfileStoreID(t *testing.T) {
	t.Parallel()

	var lastStoreID int64
	builder := stubSheinProductAPIBuilder{api: &stubSheinProductAPI{}, lastStoreID: &lastStoreID}
	svc := &service{
		storeProfileRepo:       newInMemoryStoreProfileRepository(),
		routingSettingsRepo:    newInMemoryStoreRoutingSettingsRepository(),
		sheinProductAPIBuilder: builder,
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "505", UserID: "user-e"})
	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:  903,
		Enabled:  true,
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}

	task := &Task{
		TenantID: "505",
		Request:  &GenerateRequest{},
	}
	api, err := svc.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		t.Fatalf("buildSheinSubmitProductAPI error = %v", err)
	}
	if api == nil {
		t.Fatal("expected product api")
	}
	if lastStoreID != 903 {
		t.Fatalf("builder store id = %d, want 903", lastStoreID)
	}
}
