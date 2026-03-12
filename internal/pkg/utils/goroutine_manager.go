// Package utils 提供工具方法
package utils

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskInfo 任务信息
type TaskInfo struct {
	Name      string
	StartedAt time.Time
}

// GoroutineManager goroutine 管理器
type GoroutineManager struct {
	ctx     context.Context
	logger  *logrus.Entry
	tasks   map[string]*TaskInfo
	mutex   sync.RWMutex
	wg      sync.WaitGroup
	running int32
}

// NewGoroutineManager 创建 goroutine 管理器
func NewGoroutineManager(ctx context.Context, logger *logrus.Entry) *GoroutineManager {
	return &GoroutineManager{
		ctx:    ctx,
		logger: logger,
		tasks:  make(map[string]*TaskInfo),
	}
}

// Start 启动一个带名称的 goroutine
func (gm *GoroutineManager) Start(name string, fn func(ctx context.Context) error) {
	gm.mutex.Lock()
	gm.tasks[name] = &TaskInfo{
		Name:      name,
		StartedAt: time.Now(),
	}
	gm.running++
	gm.mutex.Unlock()

	gm.wg.Add(1)

	go func() {
		defer gm.wg.Done()
		defer func() {
			gm.mutex.Lock()
			gm.running--
			delete(gm.tasks, name)
			gm.mutex.Unlock()
		}()

		if gm.logger != nil {
			gm.logger.WithField("task", name).Info("任务启动")
		}

		if err := fn(gm.ctx); err != nil {
			if gm.logger != nil {
				gm.logger.WithError(err).WithField("task", name).Error("任务执行出错")
			}
		}
	}()
}

// Wait 等待所有任务完成
func (gm *GoroutineManager) Wait() {
	gm.wg.Wait()
}

// WaitWithTimeout 等待所有任务完成，带超时
func (gm *GoroutineManager) WaitWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		gm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

// GetRunningCount 获取运行中的任务数量
func (gm *GoroutineManager) GetRunningCount() int {
	return int(gm.running)
}

// GetStatus 获取任务状态
func (gm *GoroutineManager) GetStatus() map[string]interface{} {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	tasks := make(map[string]interface{})
	for name, info := range gm.tasks {
		tasks[name] = map[string]interface{}{
			"name":       info.Name,
			"started_at": info.StartedAt,
		}
	}

	return map[string]interface{}{
		"running_count": gm.running,
		"tasks":         tasks,
	}
}
