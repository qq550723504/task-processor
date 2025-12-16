// Package pipeline 提供统一的任务处理接口定义
package pipeline

import (
	"context"
	"task-processor/common/amazon/model"
	"task-processor/common/management"
	"task-processor/common/memory"
	"task-processor/common/types"
)

// UnifiedTaskContextInterface 统一的任务上下文接口
// 所有平台的TaskContext都应该实现这个接口
type UnifiedTaskContextInterface interface {
	// 基础方法
	GetContext() context.Context
	GetTask() *types.Task
	SetData(key string, value any)
	GetData(key string) (any, bool)
	GetStringData(key string) (string, bool)
	GetIntData(key string) (int, bool)
	GetBoolData(key string) (bool, bool)

	// 通用服务访问
	GetManagementClient() *management.ClientManager
	SetManagementClient(client *management.ClientManager)
	GetMemoryManager() *memory.MemoryManager
	SetMemoryManager(manager *memory.MemoryManager)

	// 通用产品数据访问
	GetAmazonProduct() *model.Product
	SetAmazonProduct(product *model.Product)
	GetVariants() []*model.Product
	SetVariants(variants []*model.Product)
	AddVariant(variant *model.Product)

	// 状态管理
	IsCompleted() bool
	SetCompleted(completed bool)
	GetError() error
	SetError(err error)
}

// UnifiedStepHandler 统一的步骤处理器接口
type UnifiedStepHandler interface {
	Name() string
	Handle(ctx UnifiedTaskContextInterface) error
}

// PlatformSpecificHandler 平台特定处理器接口
// 用于需要访问平台特定功能的处理器
type PlatformSpecificHandler interface {
	Name() string
	// 平台特定的Handle方法由各平台自己定义
}

// HandlerAdapter 处理器适配器接口
// 用于将平台特定的处理器适配为统一接口
type HandlerAdapter interface {
	UnifiedStepHandler
	// AdaptHandler 适配平台特定的处理器
	AdaptHandler(handler PlatformSpecificHandler) UnifiedStepHandler
}
