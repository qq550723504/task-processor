// Package pipeline 提供任务上下文接口定义
package pipeline

import (
	"context"
	"task-processor/internal/app/state"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/management"
)

// TaskContext 核心任务上下文接口
type TaskContext interface {
	// 基础方法
	GetContext() context.Context
	GetTask() *model.Task

	// 通用数据存储
	SetData(key string, value any)
	GetData(key string) (any, bool)
	GetStringData(key string) (string, bool)
	GetIntData(key string) (int, bool)
	GetBoolData(key string) (bool, bool)

	// 状态管理
	IsCompleted() bool
	SetCompleted(completed bool)
	GetError() error
	SetError(err error)
}

// ManagementContext 管理系统上下文接口
type ManagementContext interface {
	TaskContext
	GetManagementClient() *management.ClientManager
	SetManagementClient(client *management.ClientManager)
	GetMemoryManager() *state.MemoryManager
	SetMemoryManager(manager *state.MemoryManager)
}

// APIContext API客户端上下文接口
type APIContext interface {
	TaskContext
	GetAPIClient() any
	SetAPIClient(client any)
}

// AmazonContext Amazon处理器上下文接口
type AmazonContext interface {
	TaskContext
	GetAmazonProcessor() *amazon.AmazonProcessor
	SetAmazonProcessor(processor *amazon.AmazonProcessor)
	GetAmazonProduct() *model.Product
	SetAmazonProduct(product *model.Product)
	GetVariants() []*model.Product
	SetVariants(variants []*model.Product)
	AddVariant(variant *model.Product)
}

// TemuTaskContext TEMU平台特定任务上下文接口
type TemuTaskContext interface {
	ManagementContext
	APIContext
	AmazonContext
}

// SheinTaskContext SHEIN平台特定任务上下文接口
type SheinTaskContext interface {
	ManagementContext
	APIContext
	AmazonContext
}
