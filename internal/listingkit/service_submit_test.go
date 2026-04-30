package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	task *Task
}

func (r *stubSubmitRepo) CreateTask(ctx context.Context, task *Task) error {
	copied := *task
	r.task = &copied
	return nil
}

func (r *stubSubmitRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	if r.task == nil || r.task.ID != taskID {
		return nil, ErrTaskNotFound
	}
	copied := *r.task
	return &copied, nil
}

func (r *stubSubmitRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
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
	if r.task == nil || r.task.ID != taskID {
		return ErrTaskNotFound
	}
	r.task.Result = result
	r.task.UpdatedAt = time.Now()
	return nil
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
	uploaded       map[string]string
	calls          map[string]int
	originalCalls  int
	originalUpload string
	err            error
}

func (s *stubSheinImageAPI) UploadOriginalImage(imageData []byte) (string, error) {
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
	return nil, errors.New("not implemented")
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

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
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

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
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

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
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

func TestSubmitTaskPublishBlocksMissingSizeMapRole(t *testing.T) {
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
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("publish err = %v, want readiness block", err)
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
	if !foundSizeMapBlocker {
		t.Fatalf("publish readiness = %+v, want size map blocker", readiness)
	}
	if publishCalled {
		t.Fatal("publish should not be called when strict image roles are missing")
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
