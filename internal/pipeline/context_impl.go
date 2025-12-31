// Package pipeline 提供任务上下文具体实现
package pipeline

import (
	"context"
	"sync"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/management"
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/memory"
)

// DefaultTaskContext 默认任务上下文实现
type DefaultTaskContext struct {
	ctx              context.Context
	task             *model.Task
	data             map[string]any
	managementClient *management.ClientManager
	memoryManager    *memory.MemoryManager
	apiClient        any
	amazonProcessor  *amazon.AmazonProcessor
	amazonProduct    *model.Product
	variants         []*model.Product
	completed        bool
	err              error
	mu               sync.RWMutex
}

// NewTaskContext 创建新的任务上下文
func NewTaskContext(ctx context.Context, task *model.Task) TaskContext {
	return &DefaultTaskContext{
		ctx:  ctx,
		task: task,
		data: make(map[string]any),
	}
}

// NewTemuTaskContext 创建TEMU任务上下文
func NewTemuTaskContext(ctx context.Context, task *model.Task) TemuTaskContext {
	return &DefaultTaskContext{
		ctx:  ctx,
		task: task,
		data: make(map[string]any),
	}
}

// NewSheinTaskContext 创建SHEIN任务上下文
func NewSheinTaskContext(ctx context.Context, task *model.Task) SheinTaskContext {
	return &DefaultTaskContext{
		ctx:  ctx,
		task: task,
		data: make(map[string]any),
	}
}

// 基础方法实现
func (tc *DefaultTaskContext) GetContext() context.Context {
	return tc.ctx
}

func (tc *DefaultTaskContext) GetTask() *model.Task {
	return tc.task
}

// 数据存储方法实现
func (tc *DefaultTaskContext) SetData(key string, value any) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.data[key] = value
}

func (tc *DefaultTaskContext) GetData(key string) (any, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	val, ok := tc.data[key]
	return val, ok
}

func (tc *DefaultTaskContext) GetStringData(key string) (string, bool) {
	val, ok := tc.GetData(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func (tc *DefaultTaskContext) GetIntData(key string) (int, bool) {
	val, ok := tc.GetData(key)
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}

func (tc *DefaultTaskContext) GetBoolData(key string) (bool, bool) {
	val, ok := tc.GetData(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// 状态管理方法实现
func (tc *DefaultTaskContext) IsCompleted() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.completed
}

func (tc *DefaultTaskContext) SetCompleted(completed bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.completed = completed
}

func (tc *DefaultTaskContext) GetError() error {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.err
}

func (tc *DefaultTaskContext) SetError(err error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.err = err
}

// 管理系统上下文方法实现
func (tc *DefaultTaskContext) GetManagementClient() *management.ClientManager {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.managementClient
}

func (tc *DefaultTaskContext) SetManagementClient(client *management.ClientManager) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.managementClient = client
}

func (tc *DefaultTaskContext) GetMemoryManager() *memory.MemoryManager {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.memoryManager
}

func (tc *DefaultTaskContext) SetMemoryManager(manager *memory.MemoryManager) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.memoryManager = manager
}

// API上下文方法实现
func (tc *DefaultTaskContext) GetAPIClient() any {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.apiClient
}

func (tc *DefaultTaskContext) SetAPIClient(client any) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.apiClient = client
}

// Amazon上下文方法实现
func (tc *DefaultTaskContext) GetAmazonProcessor() *amazon.AmazonProcessor {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.amazonProcessor
}

func (tc *DefaultTaskContext) SetAmazonProcessor(processor *amazon.AmazonProcessor) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.amazonProcessor = processor
}

func (tc *DefaultTaskContext) GetAmazonProduct() *model.Product {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.amazonProduct
}

func (tc *DefaultTaskContext) SetAmazonProduct(product *model.Product) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.amazonProduct = product
}

func (tc *DefaultTaskContext) GetVariants() []*model.Product {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.variants
}

func (tc *DefaultTaskContext) SetVariants(variants []*model.Product) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.variants = variants
}

func (tc *DefaultTaskContext) AddVariant(variant *model.Product) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.variants = append(tc.variants, variant)
}
