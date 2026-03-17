// Package pipeline 提供任务上下文具体实现
package pipeline

import (
	"context"
	"sync"
	"task-processor/internal/model"
)

// DefaultTaskContext 通用任务上下文，只持有与平台无关的基础字段。
// 各平台通过嵌入此结构体并添加自身字段来扩展。
type DefaultTaskContext struct {
	ctx       context.Context
	task      *model.Task
	data      map[string]any
	completed bool
	err       error
	mu        sync.RWMutex
}

// NewTaskContext 创建通用任务上下文
func NewTaskContext(ctx context.Context, task *model.Task) *DefaultTaskContext {
	return &DefaultTaskContext{
		ctx:  ctx,
		task: task,
		data: make(map[string]any),
	}
}

func (tc *DefaultTaskContext) GetContext() context.Context {
	return tc.ctx
}

func (tc *DefaultTaskContext) GetTask() *model.Task {
	return tc.task
}

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

var _ TaskContext = (*DefaultTaskContext)(nil)
