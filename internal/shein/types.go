package shein

import (
	"context"
	"task-processor/internal/app/state"
	"task-processor/internal/model"
	"task-processor/internal/infra/clients/management"
	management_api "task-processor/internal/infra/clients/management/api"
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

// Task 使用公共的Task类型
type Task = model.Task

// VariantFilterInfo 变体过滤信息
type VariantFilterInfo struct {
	FilteredOut  bool
	FilterReason string
}

// TaskContext 任务处理上下文
type TaskContext struct {
	Context            context.Context
	Task               *Task
	MemoryManager      *state.MemoryManager // 状态管理器
	StoreInfo          *management_api.StoreRespDTO
	SupplierInfo       *other.SupplierOperateInfo
	SpuLimitCount      *other.SpuLimitCountInfo
	ShelfQuotaInfo     *other.ShelfQuotaInfo // SKC上架额度信息
	AmazonProduct      *model.Product
	Variants           *[]model.Product
	UnFilteredVariants *[]model.Product
	VariantFilterMap   map[string]*VariantFilterInfo // ASIN到过滤信息的映射
	AsinSkuMap         map[string]string             // ASIN与SKU的对应关系
	SupplierSkuMap     map[string]string
	ProductData        *product.Product
	// API客户端（拆分为具体的API实例，避免巨大接口）
	ProductAPI          *product.Client
	CategoryAPI         *shein_category.Client
	AttributeAPI        *shein_attribute.Client
	WarehouseAPI        *warehouse.Client
	TranslateAPI        *shein_translate.Client
	PricingAPI          *shein_pricing.Client
	ImageAPI            *shein_image.Client
	OtherAPI            *other.Client
	MarketingAPI        *shein_marketing.Client
	FilterRule          *management_api.FilterRuleRespDTO      // 筛选规则
	ProfitRule          *management_api.ProfitRuleRespDTO      // 利润规则
	Warehouses          *warehouse.WarehouseResponse           // 仓库信息
	SiteList            []product.SiteInfo                     // 站点信息
	CategoryTree        *shein_category.CategoryTreeResponse   // 分类树
	AttributeTemplates  *shein_attribute.AttributeTemplateInfo // 属性模板信息
	BuildAttributeData  *BuildAttributeInfo                    // 构建属性数据
	GenerateAttribute   *AttributeData                         // 生成属性数据
	SaleSpecResult      *ResultSaleAttribute                   // 结果销售规格
	ManagementClientMgr *management.ClientManager              // 管理客户端管理器
	SheinResponse       *product.SheinResponse
	// 错误信息相关字段
	ValidationErrors    []PreValidResult // 验证错误信息
	SpecificationErrors []PreValidResult // 规格配置错误信息
	// 敏感词处理相关字段
	SensitiveWordRetryCount int             // 敏感词重试计数器
	ProcessedSensitiveWords map[string]bool // 已处理的敏感词记录
	// 扩展字段，用于存储额外的上下文信息
	Extra map[string]any // 扩展字段
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

// NewTaskContext 创建新的任务上下文
func NewTaskContext(ctx context.Context, task *Task) *TaskContext {
	return &TaskContext{
		Context:                 ctx,
		Task:                    task,
		VariantFilterMap:        make(map[string]*VariantFilterInfo),
		AsinSkuMap:              make(map[string]string),
		SupplierSkuMap:          make(map[string]string),
		ProcessedSensitiveWords: make(map[string]bool),
		Extra:                   make(map[string]any),
	}
}

// 实现pipeline.TaskContext接口的方法

// GetContext 获取上下文
func (ctx *TaskContext) GetContext() context.Context {
	return ctx.Context
}

// GetTask 获取任务信息
func (ctx *TaskContext) GetTask() *model.Task {
	return ctx.Task
}

// SetData 设置数据
func (ctx *TaskContext) SetData(key string, value any) {
	if ctx.Extra == nil {
		ctx.Extra = make(map[string]any)
	}
	ctx.Extra[key] = value
}

// GetData 获取数据
func (ctx *TaskContext) GetData(key string) (any, bool) {
	if ctx.Extra == nil {
		return nil, false
	}
	val, ok := ctx.Extra[key]
	return val, ok
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
	// 根据实际业务逻辑实现
	return false
}

// SetCompleted 设置完成状态
func (ctx *TaskContext) SetCompleted(completed bool) {
	ctx.SetData("completed", completed)
}

// GetError 获取错误
func (ctx *TaskContext) GetError() error {
	val, ok := ctx.GetData("error")
	if !ok {
		return nil
	}
	err, ok := val.(error)
	if !ok {
		return nil
	}
	return err
}

// SetError 设置错误
func (ctx *TaskContext) SetError(err error) {
	ctx.SetData("error", err)
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

