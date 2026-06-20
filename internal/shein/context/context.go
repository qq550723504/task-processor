package context

import (
	"context"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	"task-processor/internal/shein/aicache"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
	sheinimage "task-processor/internal/shein/api/image"
	sheinmarketing "task-processor/internal/shein/api/marketing"
	"task-processor/internal/shein/api/other"
	sheinpricing "task-processor/internal/shein/api/pricing"
	"task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
	"task-processor/internal/shein/api/warehouse"
	"task-processor/internal/shein/authorizedbrand"
	"task-processor/internal/state"
)

type StepHandler interface {
	Name() string
	Handle(ctx *TaskContext) error
}

type VariantFilterInfo struct {
	FilteredOut  bool
	FilterReason string
}

type PreValidResult struct {
	Form                    string                     `json:"form"`
	FormName                string                     `json:"form_name"`
	Messages                []string                   `json:"messages"`
	Module                  string                     `json:"module"`
	OtherLanguageMessageMap map[string][]string        `json:"other_language_message_map"`
	SkcErrorMessageMap      map[string]SkcErrorMessage `json:"skc_error_message_map"`
}

type SkcErrorMessage struct {
	Messages                []string            `json:"messages"`
	OtherLanguageMessageMap map[string][]string `json:"otherLanguageMessageMap"`
}

type RuntimeRepository interface {
	RuntimePublishedProductExists(ctx context.Context, storeID int64, platform, region, productID string) (bool, error)
	FindRuntimeProductImportMappingByTaskAndSKU(ctx context.Context, importTaskID int64, sku string) (*listingruntime.ProductImportMapping, error)
	CreateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) (int64, error)
	UpdateRuntimeProductImportMapping(ctx context.Context, req *listingruntime.ProductImportMappingUpsert) error
	GetRuntimeStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error)
}

type RuntimeState struct {
	Context           context.Context
	Task              *model.Task
	MemoryManager     *state.MemoryManager
	RuntimeRepository RuntimeRepository
	AICache           *aicache.Cache
	StoreInfo         *listingruntime.StoreInfo
	AuthorizedBrand   *authorizedbrand.Resolved
}

type ProductState struct {
	SupplierInfo           *other.SupplierOperateInfo
	SpuLimitCount          *other.SpuLimitCountInfo
	ShelfQuotaInfo         *other.ShelfQuotaInfo
	AmazonProduct          *model.Product
	Variants               *[]model.Product
	UnFilteredVariants     *[]model.Product
	VariantFilterMap       map[string]*VariantFilterInfo
	AsinSkuMap             map[string]string
	SupplierSkuMap         map[string]string
	ProductData            *product.Product
	FilterRule             *listingruntime.FilterRule
	ProfitRule             *listingruntime.ProfitRule
	Warehouses             *warehouse.WarehouseResponse
	SiteList               []product.SiteInfo
	CategoryTree           *sheincategory.CategoryTreeResponse
	AttributeTemplates     *sheinattribute.AttributeTemplateInfo
	BuildAttributeData     *BuildAttributeInfo
	GenerateAttribute      *AttributeData
	SaleSpecResult         *ResultSaleAttribute
	SaleAttributeSelection *SaleAttributeSelectionState
}

type APIClients struct {
	ProductAPI   *product.Client
	CategoryAPI  *sheincategory.Client
	AttributeAPI *sheinattribute.Client
	WarehouseAPI *warehouse.Client
	TranslateAPI *sheintranslate.Client
	PricingAPI   *sheinpricing.Client
	ImageAPI     *sheinimage.Client
	OtherAPI     *other.Client
	MarketingAPI *sheinmarketing.Client
}

type TaskState struct {
	SheinResponse           *product.SheinResponse
	ValidationErrors        []PreValidResult
	SpecificationErrors     []PreValidResult
	SensitiveWordRetryCount int
	ProcessedSensitiveWords map[string]bool
	CurrentStage            string
	Platform                string
	SkipSheinPipeline       bool
	InitError               error
	DailyQuotaReserved      bool
	DailyQuotaDate          string
	DailyQuotaIncrement     int64
	DailyQuotaPauseApplied  bool
}

type TaskContext struct {
	RuntimeState
	ProductState
	APIClients
	TaskState
}

func NewTaskContext(ctx context.Context, task *model.Task) *TaskContext {
	return &TaskContext{
		RuntimeState: RuntimeState{Context: ctx, Task: task},
		ProductState: ProductState{
			VariantFilterMap: make(map[string]*VariantFilterInfo),
			AsinSkuMap:       make(map[string]string),
			SupplierSkuMap:   make(map[string]string),
		},
		TaskState: TaskState{ProcessedSensitiveWords: make(map[string]bool)},
	}
}

func (ctx *TaskContext) AttachRuntime(memoryManager *state.MemoryManager, runtimeRepository RuntimeRepository, cache *aicache.Cache) {
	ctx.MemoryManager = memoryManager
	ctx.RuntimeRepository = runtimeRepository
	ctx.AICache = cache
}

func (ctx *TaskContext) SetStoreInfo(storeInfo *listingruntime.StoreInfo) {
	ctx.StoreInfo = storeInfo
}

func (ctx *TaskContext) SetAuthorizedBrand(value *authorizedbrand.Resolved) {
	ctx.AuthorizedBrand = value
}

func (ctx *TaskContext) SetValidationRules(filterRule *listingruntime.FilterRule, profitRule *listingruntime.ProfitRule) {
	ctx.FilterRule = filterRule
	ctx.ProfitRule = profitRule
}

func (ctx *TaskContext) SetAmazonProduct(product *model.Product) {
	ctx.AmazonProduct = product
}

func (ctx *TaskContext) SetVariants(variants []model.Product) {
	variantsCopy := variants
	ctx.Variants = &variantsCopy
}

func (ctx *TaskContext) SetUnfilteredVariants(variants []model.Product) {
	variantsCopy := variants
	ctx.UnFilteredVariants = &variantsCopy
}

func (ctx *TaskContext) FilteredVariants() []model.Product {
	if ctx.Variants == nil {
		return nil
	}
	return *ctx.Variants
}

func (ctx *TaskContext) SetProductData(productData *product.Product) {
	ctx.ProductData = productData
}

func (ctx *TaskContext) UpdateProductData(update func(*product.Product)) {
	if ctx.ProductData == nil || update == nil {
		return
	}
	update(ctx.ProductData)
}

func (ctx *TaskContext) SetCategoryTree(categoryTree *sheincategory.CategoryTreeResponse) {
	ctx.CategoryTree = categoryTree
}

func (ctx *TaskContext) SetAttributeTemplates(attributeTemplates *sheinattribute.AttributeTemplateInfo) {
	ctx.AttributeTemplates = attributeTemplates
}

func (ctx *TaskContext) SetBuildAttributeData(buildAttributeData *BuildAttributeInfo) {
	ctx.BuildAttributeData = buildAttributeData
}

func (ctx *TaskContext) SetSaleSpecResult(saleSpecResult *ResultSaleAttribute) {
	ctx.SaleSpecResult = saleSpecResult
}

func (ctx *TaskContext) SetSaleAttributeSelection(selection *SaleAttributeSelectionState) {
	ctx.SaleAttributeSelection = selection
}

func (ctx *TaskContext) SetSheinResponse(response *product.SheinResponse) {
	ctx.SheinResponse = response
}

func (ctx *TaskContext) SetSpecificationErrors(errors []PreValidResult) {
	ctx.SpecificationErrors = errors
}

func (ctx *TaskContext) SetStage(stage string) {
	ctx.CurrentStage = stage
}

func (ctx *TaskContext) GetStage() string {
	return ctx.CurrentStage
}

func (ctx *TaskContext) SetDailyQuotaReservation(date string, increment int64) {
	ctx.DailyQuotaReserved = true
	ctx.DailyQuotaDate = date
	ctx.DailyQuotaIncrement = increment
}

func (ctx *TaskContext) ClearDailyQuotaReservation() {
	ctx.DailyQuotaReserved = false
	ctx.DailyQuotaDate = ""
	ctx.DailyQuotaIncrement = 0
	ctx.DailyQuotaPauseApplied = false
}

func (ctx *TaskContext) MarkDailyQuotaPauseApplied() {
	ctx.DailyQuotaPauseApplied = true
}

func (ctx *TaskContext) SetSupplierSkuMapping(platformSKU, supplierSKU string) {
	if ctx.SupplierSkuMap == nil {
		ctx.SupplierSkuMap = make(map[string]string)
	}
	ctx.SupplierSkuMap[platformSKU] = supplierSKU
}

func (ctx *TaskContext) SetWarehouses(warehouses *warehouse.WarehouseResponse) {
	ctx.Warehouses = warehouses
}

func (ctx *TaskContext) SetSiteList(siteList []product.SiteInfo) {
	ctx.SiteList = siteList
	if ctx.ProductData != nil {
		ctx.ProductData.SiteList = siteList
	}
}

func (ctx *TaskContext) GetContext() context.Context {
	return ctx.Context
}

func (ctx *TaskContext) GetTask() *model.Task {
	return ctx.Task
}

func (ctx *TaskContext) SetVariantFiltered(asin string, filteredOut bool, reason string) {
	if ctx.VariantFilterMap == nil {
		ctx.VariantFilterMap = make(map[string]*VariantFilterInfo)
	}
	ctx.VariantFilterMap[asin] = &VariantFilterInfo{FilteredOut: filteredOut, FilterReason: reason}
}

func (ctx *TaskContext) GetVariantFilterInfo(asin string) *VariantFilterInfo {
	if ctx.VariantFilterMap == nil {
		return nil
	}
	return ctx.VariantFilterMap[asin]
}

func (ctx *TaskContext) IsVariantFiltered(asin string) bool {
	info := ctx.GetVariantFilterInfo(asin)
	return info != nil && info.FilteredOut
}

func (ctx *TaskContext) SetData(key string, value any) {
	switch key {
	case "init_error", "error":
		if err, ok := value.(error); ok {
			ctx.InitError = err
		}
	case "completed":
		if b, ok := value.(bool); ok {
			ctx.SkipSheinPipeline = b
		}
	}
}

func (ctx *TaskContext) GetData(key string) (any, bool) {
	switch key {
	case "init_error", "error":
		if ctx.InitError != nil {
			return ctx.InitError, true
		}
		return nil, false
	case "completed":
		return ctx.SkipSheinPipeline, ctx.SkipSheinPipeline
	}
	return nil, false
}

func (ctx *TaskContext) GetStringData(key string) (string, bool) {
	val, ok := ctx.GetData(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func (ctx *TaskContext) GetIntData(key string) (int, bool) {
	val, ok := ctx.GetData(key)
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}

func (ctx *TaskContext) GetBoolData(key string) (bool, bool) {
	val, ok := ctx.GetData(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

func (ctx *TaskContext) IsCompleted() bool {
	return ctx.SkipSheinPipeline
}

func (ctx *TaskContext) SetCompleted(completed bool) {
	ctx.SkipSheinPipeline = completed
}

func (ctx *TaskContext) ShouldSkipPipeline() bool {
	return ctx.SkipSheinPipeline
}

func (ctx *TaskContext) SetSkipPipeline(skip bool) {
	ctx.SkipSheinPipeline = skip
}

func (ctx *TaskContext) GetError() error {
	return ctx.InitError
}

func (ctx *TaskContext) SetError(err error) {
	ctx.InitError = err
}
