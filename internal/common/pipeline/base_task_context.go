// Package pipeline 提供通用的任务上下文基础实现
package pipeline

import (
	"context"
	"sync"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/management"
	"task-processor/internal/common/memory"
	"task-processor/internal/common/types"
)

// BaseTaskContext 通用任务上下文基础实现
// 包含所有平台共有的字段和功能
type BaseTaskContext struct {
	// 基础字段
	ctx  context.Context
	Task *types.Task

	// 通用服务（所有平台都可能用到）
	ManagementClient *management.ClientManager
	MemoryManager    *memory.MemoryManager

	// 通用产品数据
	AmazonProduct *model.Product
	Variants      []*model.Product

	// 通用数据存储
	data map[string]interface{}

	// 状态管理
	completed bool
	err       error
	mutex     sync.RWMutex
}

// NewBaseTaskContext 创建基础任务上下文
func NewBaseTaskContext(ctx context.Context, task *types.Task) *BaseTaskContext {
	return &BaseTaskContext{
		ctx:      ctx,
		Task:     task,
		data:     make(map[string]interface{}),
		Variants: make([]*model.Product, 0),
	}
}

// GetContext 获取上下文
func (btc *BaseTaskContext) GetContext() context.Context {
	return btc.ctx
}

// GetTask 获取任务信息
func (btc *BaseTaskContext) GetTask() *types.Task {
	return btc.Task
}

// SetData 设置数据（线程安全）
func (btc *BaseTaskContext) SetData(key string, value interface{}) {
	btc.mutex.Lock()
	defer btc.mutex.Unlock()
	btc.data[key] = value
}

// GetData 获取数据（线程安全）
func (btc *BaseTaskContext) GetData(key string) (interface{}, bool) {
	btc.mutex.RLock()
	defer btc.mutex.RUnlock()
	value, exists := btc.data[key]
	return value, exists
}

// GetStringData 获取字符串数据
func (btc *BaseTaskContext) GetStringData(key string) (string, bool) {
	if value, exists := btc.GetData(key); exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetIntData 获取整数数据
func (btc *BaseTaskContext) GetIntData(key string) (int, bool) {
	if value, exists := btc.GetData(key); exists {
		if i, ok := value.(int); ok {
			return i, true
		}
	}
	return 0, false
}

// GetBoolData 获取布尔数据
func (btc *BaseTaskContext) GetBoolData(key string) (bool, bool) {
	if value, exists := btc.GetData(key); exists {
		if b, ok := value.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// SetManagementClient 设置管理客户端
func (btc *BaseTaskContext) SetManagementClient(client *management.ClientManager) {
	btc.ManagementClient = client
}

// GetManagementClient 获取管理客户端
func (btc *BaseTaskContext) GetManagementClient() *management.ClientManager {
	return btc.ManagementClient
}

// SetMemoryManager 设置内存管理器
func (btc *BaseTaskContext) SetMemoryManager(manager *memory.MemoryManager) {
	btc.MemoryManager = manager
}

// GetMemoryManager 获取内存管理器
func (btc *BaseTaskContext) GetMemoryManager() *memory.MemoryManager {
	return btc.MemoryManager
}

// SetAmazonProduct 设置Amazon产品数据
func (btc *BaseTaskContext) SetAmazonProduct(product *model.Product) {
	btc.AmazonProduct = product
}

// GetAmazonProduct 获取Amazon产品数据
func (btc *BaseTaskContext) GetAmazonProduct() *model.Product {
	return btc.AmazonProduct
}

// SetVariants 设置变体数据
func (btc *BaseTaskContext) SetVariants(variants []*model.Product) {
	btc.Variants = variants
}

// GetVariants 获取变体数据
func (btc *BaseTaskContext) GetVariants() []*model.Product {
	return btc.Variants
}

// AddVariant 添加变体
func (btc *BaseTaskContext) AddVariant(variant *model.Product) {
	btc.Variants = append(btc.Variants, variant)
}

// IsCompleted 检查是否完成
func (btc *BaseTaskContext) IsCompleted() bool {
	btc.mutex.RLock()
	defer btc.mutex.RUnlock()
	return btc.completed
}

// SetCompleted 设置完成状态
func (btc *BaseTaskContext) SetCompleted(completed bool) {
	btc.mutex.Lock()
	defer btc.mutex.Unlock()
	btc.completed = completed
}

// GetError 获取错误
func (btc *BaseTaskContext) GetError() error {
	btc.mutex.RLock()
	defer btc.mutex.RUnlock()
	return btc.err
}

// SetError 设置错误
func (btc *BaseTaskContext) SetError(err error) {
	btc.mutex.Lock()
	defer btc.mutex.Unlock()
	btc.err = err
}

// 确保BaseTaskContext实现UnifiedTaskContextInterface接口
var _ UnifiedTaskContextInterface = (*BaseTaskContext)(nil)
