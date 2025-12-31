package modules

import (
	"context"
	"task-processor/internal/domain/model"
	types "task-processor/internal/domain/model"
	"task-processor/internal/infra/memory"
	"task-processor/internal/pkg/management"
	management_api "task-processor/internal/pkg/management/api"
	shein_api "task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/api/category"
	"task-processor/internal/platforms/shein/api/other"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/api/warehouse"
)

// StepHandler 任务处理步骤接口
type StepHandler interface {
	Name() string
	Handle(ctx *TaskContext) error
}

// Task 使用公共的Task类型
type Task = types.Task

// VariantFilterInfo 变体过滤信息
type VariantFilterInfo struct {
	FilteredOut  bool
	FilterReason string
}

// TaskContext 任务处理上下文
type TaskContext struct {
	Context             context.Context
	Task                *Task
	MemoryManager       *memory.MemoryManager // 内存管理器
	StoreInfo           *management_api.StoreRespDTO
	SupplierInfo        *other.SupplierOperateInfo
	SpuLimitCount       *other.SpuLimitCountInfo
	AmazonProduct       *model.Product
	Variants            *[]model.Product
	UnFilteredVariants  *[]model.Product
	VariantFilterMap    map[string]*VariantFilterInfo // ASIN到过滤信息的映射
	AsinSkuMap          map[string]string             // ASIN与SKU的对应关系
	SupplierSkuMap      map[string]string
	ProductData         *product.Product
	ShopClient          shein_api.APIClient               // 店铺API客户端
	FilterRule          *management_api.FilterRuleRespDTO // 筛选规则
	ProfitRule          *management_api.ProfitRuleRespDTO // 利润规则
	Warehouses          *warehouse.WarehouseResponse      // 仓库信息
	SiteList            []product.SiteInfo                // 站点信息
	CategoryTree        *category.CategoryTreeResponse    // 分类树
	AttributeTemplates  *attribute.AttributeTemplateInfo  // 属性模板信息
	BuildAttributeData  *BuildAttributeInfo               // 构建属性数据
	GenerateAttribute   *AttributeData                    // 生成属性数据
	SaleSpecResult      *ResultSaleAttribute              // 结果销售规格
	ManagementClientMgr *management.ClientManager         // 管理客户端管理器
	SheinResponse       *product.SheinResponse
	// 错误信息相关字段
	ValidationErrors    []PreValidResult // 验证错误信息
	SpecificationErrors []PreValidResult // 规格配置错误信息
	// 敏感词处理相关字段
	SensitiveWordRetryCount int             // 敏感词重试计数器
	ProcessedSensitiveWords map[string]bool // 已处理的敏感词记录
	// 扩展字段，用于存储额外的上下文信息
	Extra map[string]interface{} // 扩展字段
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
		Extra:                   make(map[string]interface{}),
	}
}

// 实现pipeline.TaskContext接口的方法

// GetContext 获取上下文
func (ctx *TaskContext) GetContext() context.Context {
	return ctx.Context
}

// GetTask 获取任务信息
func (ctx *TaskContext) GetTask() *types.Task {
	// 需要将SHEIN的Task转换为通用的Task类型
	// 这里暂时返回nil，需要根据实际情况实现转换逻辑
	return nil
}

// SetData 设置数据
func (ctx *TaskContext) SetData(key string, value any) {
	if ctx.Extra == nil {
		ctx.Extra = make(map[string]interface{})
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
