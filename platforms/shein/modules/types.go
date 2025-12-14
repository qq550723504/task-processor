package modules

import (
	"context"
	"task-processor/common/amazon/model"
	"task-processor/common/management"
	management_api "task-processor/common/management/api"
	"task-processor/common/memory"
	shops "task-processor/common/shein"
	shein_api "task-processor/common/shein/api"
	"task-processor/common/shein/api/attribute"
	"task-processor/common/shein/api/category"
	"task-processor/common/shein/api/other"
	"task-processor/common/shein/api/product"
	"task-processor/common/shein/api/warehouse"
	"task-processor/common/types"
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
	ShopClientMgr       *shops.ClientManager  // 店铺客户端管理器
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
