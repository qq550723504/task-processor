package context

import (
	"context"

	"task-processor/internal/app/ports"
	"task-processor/internal/app/state"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/temu/api"
	temutemplate "task-processor/internal/temu/api/template"
)

type RuntimeState struct {
	ManagementClientMgr *management.ClientManager
	MemoryManager       *state.MemoryManager
	AmazonProcessor     ports.ProductSource
	APIClient           api.APIClientInterface
	QueryAPI            api.QueryAPIInterface
}

type ProductState struct {
	AmazonProduct *model.Product
	Variants      []*model.Product
	TemuProduct   *api.Product
	StoreInfo     *managementapi.StoreRespDTO
}

type TemplateState struct {
	TemplateInfo            *temutemplate.TemplateInfo
	UserInputParentSpecList []temutemplate.UserInputParentSpec
	InputMaxSpecNum         int
	SingleSpecValueNum      int
}

type PublishState struct {
	SaveResult         *api.SaveResponse
	SavedToDraft       bool
	PriceQueryResponse *api.PriceQueryResponse
	CommitDetail       *api.CommitDetailResponse
	SubmitResponse     *api.SubmitResponse
	ProductData        *api.Product
}

type AssetState struct {
	PaddedImages        map[string][]byte
	PaddedImageSizes    map[string][2]int
	CurrentSkuContext   string
	RequiresImageUpload bool
	TotalImageCount     int
}

type VariantState struct {
	AsinSkuMap         map[string]string
	VariantAsins       []string
	CleanedTitle       string
	ProductDescription string
}

type RuleState struct {
	ProfitRule *managementapi.ProfitRuleRespDTO
	FilterRule *managementapi.FilterRuleRespDTO
}

type TemuTaskContext struct {
	*pipeline.DefaultTaskContext
	RuntimeState
	ProductState
	TemplateState
	PublishState
	AssetState
	VariantState
	RuleState
	AISkuMapping *AISkuMappingResponse
}

func NewTemuTaskContext(ctx context.Context, task *model.Task) *TemuTaskContext {
	return &TemuTaskContext{
		DefaultTaskContext: pipeline.NewTaskContext(ctx, task),
		AssetState: AssetState{
			PaddedImages:     make(map[string][]byte),
			PaddedImageSizes: make(map[string][2]int),
		},
		VariantState: VariantState{
			AsinSkuMap: make(map[string]string),
		},
	}
}

func (tc *TemuTaskContext) AttachRuntime(managementClient *management.ClientManager, memoryManager *state.MemoryManager, productSource ports.ProductSource) {
	tc.ManagementClientMgr = managementClient
	tc.MemoryManager = memoryManager
	tc.AmazonProcessor = productSource
}

func (tc *TemuTaskContext) SetAPIClients(apiClient api.APIClientInterface, queryAPI api.QueryAPIInterface) {
	tc.APIClient = apiClient
	tc.QueryAPI = queryAPI
}

func (tc *TemuTaskContext) GetAmazonProduct() *model.Product {
	return tc.AmazonProduct
}

func (tc *TemuTaskContext) SetAmazonProduct(product *model.Product) {
	tc.AmazonProduct = product
}

func (tc *TemuTaskContext) GetVariants() []*model.Product {
	return tc.Variants
}

func (tc *TemuTaskContext) SetVariants(variants []*model.Product) {
	tc.Variants = variants
}

func (tc *TemuTaskContext) AddVariant(variant *model.Product) {
	tc.Variants = append(tc.Variants, variant)
}

func (tc *TemuTaskContext) SetTemuProduct(product *api.Product) {
	tc.TemuProduct = product
}

func (tc *TemuTaskContext) SetPublishProductData(product *api.Product) {
	tc.ProductData = product
}

func (tc *TemuTaskContext) SetSaveResponse(response *api.SaveResponse) {
	tc.SaveResult = response
}

func (tc *TemuTaskContext) SetCommitDetailResponse(response *api.CommitDetailResponse) {
	tc.CommitDetail = response
}

func (tc *TemuTaskContext) SetSubmitResponse(response *api.SubmitResponse) {
	tc.SubmitResponse = response
}

func (tc *TemuTaskContext) SetSavedToDraft(saved bool) {
	tc.SavedToDraft = saved
}

func (tc *TemuTaskContext) ApplySaveResult(result *api.SaveResult) {
	if tc == nil || tc.TemuProduct == nil || result == nil {
		return
	}

	basic := &tc.TemuProduct.GoodsBasic
	if result.ListingCommitID != "" {
		basic.ListingCommitID = result.ListingCommitID
	}
	if result.ListingCommitVersion != "" {
		basic.ListingCommitVersion = result.ListingCommitVersion
	}
	if result.GoodsCommitID != "" {
		basic.GoodsCommitID = result.GoodsCommitID
	}
}

var _ pipeline.AmazonContext = (*TemuTaskContext)(nil)
