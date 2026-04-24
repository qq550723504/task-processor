package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
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

type stubSheinImageAPI struct {
	uploaded map[string]string
	calls    map[string]int
	err      error
}

func (s *stubSheinImageAPI) UploadOriginalImage(imageData []byte) (string, error) {
	return "", errors.New("not implemented")
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

func TestSubmitTaskPersistsSheinSubmissionOnPublishSuccess(t *testing.T) {
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
					Info: sheinproduct.ResponseInfo{
						Success: true,
						SPUName: "SPU-123",
						Version: "v1",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
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

func TestSubmitTaskMarksSaveDraftCodeZeroAsSuccess(t *testing.T) {
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
	if preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submission = %+v", preview.Shein)
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
	if submitted.ImageInfo == nil || len(submitted.ImageInfo.ImageInfoList) != len(rendered) {
		t.Fatalf("submitted image info = %+v, want %d SDS images", submitted.ImageInfo, len(rendered))
	}
	for index, image := range submitted.ImageInfo.ImageInfoList {
		if image.ImageURL != uploaded[index] {
			t.Fatalf("submitted SPU image %d = %q, want uploaded %q", index, image.ImageURL, uploaded[index])
		}
		if image.ImageURL == sourceImage {
			t.Fatalf("submitted SPU image still uses source image: %q", sourceImage)
		}
	}
	if len(submitted.SKCList) != 1 || len(submitted.SKCList[0].ImageInfo.ImageInfoList) != len(rendered) {
		t.Fatalf("submitted SKC image info = %+v", submitted.SKCList)
	}
	if submitted.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL != uploaded[0] {
		t.Fatalf("submitted SKC main image = %q, want uploaded %q", submitted.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL, uploaded[0])
	}
	if len(submitted.SKCList[0].SKUS) != 1 || submitted.SKCList[0].SKUS[0].ImageInfo == nil || len(submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList) != 1 {
		t.Fatalf("submitted SKU image info = %+v", submitted.SKCList[0].SKUS)
	}
	if got := submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList[0].ImageURL; got != uploaded[0] {
		t.Fatalf("submitted SKU main image = %q, want uploaded %q", got, uploaded[0])
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
