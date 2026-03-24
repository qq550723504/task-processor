// Package context 提供 SHEIN 任务处理上下文定义
package context

import (
	"context"

	"task-processor/internal/app/state"
	"task-processor/internal/infra/clients/management"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/shein/aicache"
	shein_attribute "task-processor/internal/shein/api/attribute"
	shein_category "task-processor/internal/shein/api/category"
	shein_image "task-processor/internal/shein/api/image"
	shein_marketing "task-processor/internal/shein/api/marketing"
	"task-processor/internal/shein/api/other"
	shein_pricing "task-processor/internal/shein/api/pricing"
	"task-processor/internal/shein/api/product"
	shein_translate "task-processor/internal/shein/api/translate"
	"task-processor/internal/shein/api/warehouse"
)

// StepHandler 任务处理步骤接口
type StepHandler interface {
	Name() string
	Handle(ctx *TaskContext) error
}

// VariantFilterInfo 变体过滤信息
type VariantFilterInfo struct {
	FilteredOut  bool
	FilterReason string
}

// PreValidResult 预验证结果
type PreValidResult struct {
	Form                    string                     `json:"form"`
	FormName                string                     `json:"form_name"`
	Messages                []string                   `json:"messages"`
	Module                  string                     `json:"module"`
	OtherLanguageMessageMap map[string][]string        `json:"other_language_message_map"`
	SkcErrorMessageMap      map[string]SkcErrorMessage `json:"skc_error_message_map"`
}

// SkcErrorMessage SKC错误信息
type SkcErrorMessage struct {
	Messages                []string            `json:"messages"`
	OtherLanguageMessageMap map[string][]string `json:"otherLanguageMessageMap"`
}

// APIClients 封装所有 SHEIN API 客户端
type APIClients struct {
	ProductAPI   *product.Client
	CategoryAPI  *shein_category.Client
	AttributeAPI *shein_attribute.Client
	WarehouseAPI *warehouse.Client
	TranslateAPI *shein_translate.Client
	PricingAPI   *shein_pricing.Client
	ImageAPI     *shein_image.Client
	OtherAPI     *other.Client
	MarketingAPI *shein_marketing.Client
}

// TaskState 封装任务运行时中间状态
type TaskState struct {
	SheinResponse       *product.SheinResponse
	ValidationErrors    []PreValidResult
	SpecificationErrors []PreValidResult
	// 敏感词处理
	SensitiveWordRetryCount int
	ProcessedSensitiveWords map[string]bool
	// 平台路由标记
	Platform          string
	SkipSheinPipeline bool
	// 初始化错误（替代 Extra["init_error"]）
	InitError error
}

// TaskContext 任务处理上下文
type TaskContext struct {
	Context             context.Context
	Task                *model.Task
	MemoryManager       *state.MemoryManager
	ManagementClientMgr *management.ClientManager
	AICache             *aicache.Cache
	StoreInfo           *management_api.StoreRespDTO
	// 产品数据
	SupplierInfo       *other.SupplierOperateInfo
	SpuLimitCount      *other.SpuLimitCountInfo
	ShelfQuotaInfo     *other.ShelfQuotaInfo
	AmazonProduct      *model.Product
	Variants           *[]model.Product
	UnFilteredVariants *[]model.Product
	VariantFilterMap   map[string]*VariantFilterInfo
	AsinSkuMap         map[string]string
	SupplierSkuMap     map[string]string
	ProductData        *product.Product
	FilterRule         *management_api.FilterRuleRespDTO
	ProfitRule         *management_api.ProfitRuleRespDTO
	Warehouses         *warehouse.WarehouseResponse
	SiteList           []product.SiteInfo
	CategoryTree       *shein_category.CategoryTreeResponse
	AttributeTemplates *shein_attribute.AttributeTemplateInfo
	BuildAttributeData *BuildAttributeInfo
	GenerateAttribute  *AttributeData
	SaleSpecResult     *ResultSaleAttribute
	// 嵌入 API 客户端和运行时状态
	APIClients
	TaskState
}

// NewTaskContext 创建新的任务上下文
func NewTaskContext(ctx context.Context, task *model.Task) *TaskContext {
	return &TaskContext{
		Context:          ctx,
		Task:             task,
		VariantFilterMap: make(map[string]*VariantFilterInfo),
		AsinSkuMap:       make(map[string]string),
		SupplierSkuMap:   make(map[string]string),
		TaskState: TaskState{
			ProcessedSensitiveWords: make(map[string]bool),
		},
	}
}

// GetContext 获取上下文
func (ctx *TaskContext) GetContext() context.Context {
	return ctx.Context
}

// GetTask 获取任务信息
func (ctx *TaskContext) GetTask() *model.Task {
	return ctx.Task
}

// SetVariantFiltered 设置变体过滤状态
func (ctx *TaskContext) SetVariantFiltered(asin string, filteredOut bool, reason string) {
	if ctx.VariantFilterMap == nil {
		ctx.VariantFilterMap = make(map[string]*VariantFilterInfo)
	}
	ctx.VariantFilterMap[asin] = &VariantFilterInfo{
		FilteredOut:  filteredOut,
		FilterReason: reason,
	}
}

// GetVariantFilterInfo 获取变体过滤信息
func (ctx *TaskContext) GetVariantFilterInfo(asin string) *VariantFilterInfo {
	if ctx.VariantFilterMap == nil {
		return nil
	}
	return ctx.VariantFilterMap[asin]
}

// IsVariantFiltered 检查变体是否被过滤
func (ctx *TaskContext) IsVariantFiltered(asin string) bool {
	info := ctx.GetVariantFilterInfo(asin)
	return info != nil && info.FilteredOut
}

// SetData 设置扩展数据（保留以满足 pipeline.TaskContext 接口）
func (ctx *TaskContext) SetData(key string, value any) {
	switch key {
	case "init_error":
		if err, ok := value.(error); ok {
			ctx.InitError = err
		}
	case "completed":
		if b, ok := value.(bool); ok {
			ctx.SkipSheinPipeline = b
		}
	case "error":
		if err, ok := value.(error); ok {
			ctx.InitError = err
		}
	}
}

// GetData 获取扩展数据（保留以满足 pipeline.TaskContext 接口）
func (ctx *TaskContext) GetData(key string) (any, bool) {
	switch key {
	case "init_error":
		if ctx.InitError != nil {
			return ctx.InitError, true
		}
		return nil, false
	case "error":
		if ctx.InitError != nil {
			return ctx.InitError, true
		}
		return nil, false
	case "completed":
		return ctx.SkipSheinPipeline, ctx.SkipSheinPipeline
	}
	return nil, false
}

// GetStringData 获取字符串数据
func (ctx *TaskContext) GetStringData(key string) (string, bool) {
	val, ok := ctx.GetData(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetIntData 获取整数数据
func (ctx *TaskContext) GetIntData(key string) (int, bool) {
	val, ok := ctx.GetData(key)
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}

// GetBoolData 获取布尔数据
func (ctx *TaskContext) GetBoolData(key string) (bool, bool) {
	val, ok := ctx.GetData(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// IsCompleted 检查是否完成
func (ctx *TaskContext) IsCompleted() bool {
	return false
}

// SetCompleted 设置完成状态
func (ctx *TaskContext) SetCompleted(completed bool) {
	ctx.SkipSheinPipeline = completed
}

// GetError 获取错误
func (ctx *TaskContext) GetError() error {
	return ctx.InitError
}

// SetError 设置错误
func (ctx *TaskContext) SetError(err error) {
	ctx.InitError = err
}
