package temporal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sdktestsuite "go.temporal.io/sdk/testsuite"

	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPublishWorkflowWithConcreteActivitiesPersistsStateAndBuildsPreview(t *testing.T) {
	t.Parallel()

	repo := listingkitstore.NewMemTaskRepository()
	task := makeTemporalReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	publishCalls := 0
	svc, err := listingkit.NewService(&listingkit.ServiceConfig{
		Repository:     repo,
		ProductService: temporalStubSubmitProductService{},
		SheinProductAPIBuilder: temporalStubSheinProductAPIBuilder{
			api: temporalStubSheinProductAPI{
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
		SheinImageAPIBuilder: temporalStubSheinImageAPIBuilder{api: &temporalStubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	host, err := listingkit.NewSheinPublishActivityHost(svc)
	if err != nil {
		t.Fatalf("new shein publish activity host: %v", err)
	}

	var suite sdktestsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(PublishWorkflow)
	if err := RegisterSubmitActivities(env, &SubmitActivities{Host: host}); err != nil {
		t.Fatalf("register submit activities: %v", err)
	}

	requestID := "workflow-real-123"
	env.ExecuteWorkflow(PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:      task.ID,
		Platform:    "shein",
		Action:      "publish",
		RequestID:   requestID,
		RequestedAt: time.Now().UTC(),
	})

	if !env.IsWorkflowCompleted() {
		t.Fatalf("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow err: %v", err)
	}
	if publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", publishCalls)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("saved result = %+v, want shein submission state", saved.Result)
	}
	for _, phase := range []string{
		sheinpub.SubmissionPhasePrepareProduct,
		sheinpub.SubmissionPhasePreValidate,
		sheinpub.SubmissionPhaseSubmitRemote,
		sheinpub.SubmissionPhasePersistResult,
		sheinpub.SubmissionPhaseConfirmRemote,
	} {
		if !temporalHasSubmissionEventPhase(saved.Result.Shein.SubmissionEvents, phase) {
			t.Fatalf("submission events = %+v, want phase %q", saved.Result.Shein.SubmissionEvents, phase)
		}
	}

	preview, err := host.BuildSheinTaskPreview(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("build shein task preview: %v", err)
	}
	if preview == nil || preview.Shein == nil || preview.Shein.Submission == nil {
		t.Fatalf("preview = %+v, want shein submission payload", preview)
	}
	if preview.Shein.Submission.LastStatus != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("preview last status = %q, want %q", preview.Shein.Submission.LastStatus, sheinpub.SubmissionStatusSuccess)
	}
	if preview.Shein.Submission.Publish == nil || preview.Shein.Submission.Publish.RequestID != requestID {
		t.Fatalf("preview publish record = %+v, want request id %q", preview.Shein.Submission.Publish, requestID)
	}
	if preview.Shein.Submission.Publish.Result == nil || !preview.Shein.Submission.Publish.Result.Success {
		t.Fatalf("preview publish result = %+v, want persisted success result", preview.Shein.Submission.Publish)
	}
}

func temporalHasSubmissionEventPhase(events []sheinpub.SubmissionEvent, phase string) bool {
	for _, event := range events {
		if event.Phase == phase {
			return true
		}
	}
	return false
}

type temporalStubSubmitProductService struct{}

func (temporalStubSubmitProductService) CreateGenerateTask(context.Context, *productenrich.GenerateRequest) (*productenrich.Task, error) {
	return nil, errors.New("not implemented")
}

func (temporalStubSubmitProductService) GetTaskResult(context.Context, string) (*productenrich.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (temporalStubSubmitProductService) ProcessProduct(context.Context, *productenrich.Task) (*productenrich.ProductJSON, error) {
	return nil, errors.New("not implemented")
}

type temporalStubSheinProductAPIBuilder struct {
	api sheinproduct.ProductAPI
	msg string
}

func (s temporalStubSheinProductAPIBuilder) BuildProductAPI(storeID int64) (sheinproduct.ProductAPI, string) {
	return s.api, s.msg
}

type temporalStubSheinImageAPIBuilder struct {
	api sheinimage.ImageAPI
	msg string
}

func (s temporalStubSheinImageAPIBuilder) BuildImageAPI(storeID int64) (sheinimage.ImageAPI, string) {
	return s.api, s.msg
}

type temporalStubSheinImageAPI struct {
	uploaded map[string]string
	calls    map[string]int
	err      error
}

func (s *temporalStubSheinImageAPI) UploadOriginalImage(imageData []byte) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "https://img.shein.com/uploaded/original.jpg", nil
}

func (s *temporalStubSheinImageAPI) DownloadAndUploadImage(imageURL string) (string, error) {
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

type temporalStubSheinProductAPI struct {
	publishResponse *sheinproduct.SheinResponse
	publishErr      error
	publishHook     func(*sheinproduct.Product)
	confirmNeed     bool
	confirmErr      error
	recordResponse  *sheinproduct.RecordResponse
	recordErr       error
	inventoryResp   *sheinproduct.InventoryQueryResponse
	inventoryErr    error
}

func (s temporalStubSheinProductAPI) GetProduct(string) (*sheinproduct.Product, error) {
	return nil, errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) UpdateProduct(*sheinproduct.Product) error {
	return errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) DeleteProduct(string) error {
	return errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) GetPartInfo(int) (*sheinproduct.PartInfoResponse, error) {
	return nil, errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) SaveDraftProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) PublishProduct(prod *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	if s.publishHook != nil {
		s.publishHook(prod)
	}
	return s.publishResponse, "", s.publishErr
}
func (s temporalStubSheinProductAPI) ConfirmPublish(*sheinproduct.Product) (bool, string, error) {
	return s.confirmNeed, "", s.confirmErr
}
func (s temporalStubSheinProductAPI) Record(*sheinproduct.ProductRecordRequest) (*sheinproduct.RecordResponse, error) {
	return s.recordResponse, s.recordErr
}
func (s temporalStubSheinProductAPI) ListProducts(int, int, *sheinproduct.ProductListRequest) (*sheinproduct.ProductListResponse, error) {
	return nil, errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) QueryStock(*sheinproduct.StockQueryRequest) (*sheinproduct.StockQueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) QueryInventory(string) (*sheinproduct.InventoryQueryResponse, error) {
	return s.inventoryResp, s.inventoryErr
}
func (s temporalStubSheinProductAPI) UpdateInventory(*sheinproduct.InventoryUpdateRequest) error {
	return errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) QueryPrice(string) (*sheinproduct.PriceQueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) QueryCostPrice(string, []string) (*sheinproduct.CostPriceQueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) OffShelf(*sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}
func (s temporalStubSheinProductAPI) OnShelf(*sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}

func makeTemporalReadySheinTask() *listingkit.Task {
	productTypeID := 901
	valueID := 2493
	sizeValueID := 267
	return &listingkit.Task{
		ID: "submit-task-1",
		Request: &listingkit.GenerateRequest{
			SheinStoreID: 869,
		},
		Status: listingkit.TaskStatusCompleted,
		Result: &listingkit.ListingKitResult{
			TaskID: "submit-task-1",
			Shein: &listingkit.SheinPackage{
				CategoryID:     3221,
				CategoryIDList: []int{1, 2, 3221},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  1,
				CategoryResolution: &listingkit.SheinCategoryResolution{
					Status:         "resolved",
					Source:         "target_category_hint",
					CategoryID:     3221,
					CategoryIDList: []int{1, 2, 3221},
					ProductTypeID:  productTypeID,
					TopCategoryID:  1,
					MatchedPath:    []string{"家居&生活", "厨房&餐厅", "饮具", "真空瓶和保温杯"},
				},
				AttributeResolution: &listingkit.SheinAttributeResolution{
					Status:          "resolved",
					Source:          "attribute_templates",
					CategoryID:      3221,
					TemplateCount:   3,
					ResolvedCount:   1,
					UnresolvedCount: 0,
				},
				ResolvedAttributes: []listingkit.SheinResolvedAttribute{{
					Name:        "Capacity",
					Value:       "420ml",
					AttributeID: 7001,
					MatchedBy:   "template_exact",
				}},
				SaleAttributeResolution: &listingkit.SheinSaleAttributeResolution{
					Status:                   "resolved",
					Source:                   "sale_attribute_templates",
					CategoryID:               3221,
					PrimaryAttributeID:       27,
					SecondaryAttributeID:     87,
					PrimarySourceDimension:   "颜色",
					SecondarySourceDimension: "尺码",
					RecommendCategoryReview:  false,
					SKCAttributes: []listingkit.SheinResolvedSaleAttribute{{
						Scope:            "skc",
						Name:             "Color",
						Value:            "Black",
						AttributeID:      27,
						AttributeValueID: &valueID,
						MatchedBy:        "template_exact",
					}},
					SKUAttributes: []listingkit.SheinResolvedSaleAttribute{{
						Scope:            "sku",
						Name:             "Size",
						Value:            "39",
						AttributeID:      87,
						AttributeValueID: &sizeValueID,
						MatchedBy:        "template_exact",
					}},
				},
				SkcList: []listingkit.SheinSKCPackage{{
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
				Images: &listingkit.PlatformImageSet{
					MainImage: "https://cdn.example.com/main.jpg",
				},
				RequestDraft: &listingkit.SheinRequestDraft{
					SKCList: []listingkit.SheinSKCRequestDraft{{
						SupplierCode: "SKC-1",
						SaleAttribute: &listingkit.SheinResolvedSaleAttribute{
							Scope:            "skc",
							Name:             "Color",
							Value:            "Black",
							AttributeID:      27,
							AttributeValueID: &valueID,
						},
						SKUList: []listingkit.SheinSKUDraft{{
							SupplierSKU: "SKU-1",
							Currency:    "USD",
							CostPrice:   "10.00",
							BasePrice:   "19.99",
							StockCount:  20,
							SaleAttributes: []listingkit.SheinResolvedSaleAttribute{{
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
