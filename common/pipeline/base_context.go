// Package pipeline 提供基础的任务上下文实现
package pipeline

import (
	"context"
	"sync"
	"task-processor/common/types"
)

// BaseTaskContext 基础任务上下文实现
type BaseTaskContext struct {
	ctx       context.Context
	Task      *types.Task // 公开字段，供处理器直接访问
	data      map[string]interface{}
	completed bool
	err       error
	mutex     sync.RWMutex
}

// NewBaseTaskContext 创建基础任务上下文
func NewBaseTaskContext(ctx context.Context, task *types.Task) *BaseTaskContext {
	return &BaseTaskContext{
		ctx:  ctx,
		Task: task,
		data: make(map[string]interface{}),
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
