// Package pipeline 提供任务上下文接口定义
package pipeline

import (
	"context"
	"task-processor/internal/model"
)

// TaskContext 核心任务上下文接口
type TaskContext interface {
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

// AmazonContext 携带 Amazon 抓取结果的上下文接口。
// 由各平台 context 按需实现，pipeline 包内的通用 handler 通过类型断言使用。
type AmazonContext interface {
	TaskContext
	GetAmazonProduct() *model.Product
	SetAmazonProduct(product *model.Product)
	GetVariants() []*model.Product
	SetVariants(variants []*model.Product)
	AddVariant(variant *model.Product)
}

