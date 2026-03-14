// Package goroutine 提供 goroutine 管理与并行处理工具
package goroutine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskInfo 任务信息
type TaskInfo struct {
	Name      string
	StartedAt time.Time
}

// Manager goroutine 管理器
type Manager struct {
	ctx     context.Context
	logger  *logrus.Entry
	tasks   map[string]*TaskInfo
	mutex   sync.RWMutex
	wg      sync.WaitGroup
	running int32
}

// NewManager 创建 goroutine 管理器
func NewManager(ctx context.Context, logger *logrus.Entry) *Manager {
	return &Manager{
		ctx:    ctx,
		logger: logger,
		tasks:  make(map[string]*TaskInfo),
	}
}

// Start 启动一个带名称的 goroutine
func (gm *Manager) Start(name string, fn func(ctx context.Context) error) {
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
func (gm *Manager) Wait() {
	gm.wg.Wait()
}

// WaitWithTimeout 等待所有任务完成，带超时
func (gm *Manager) WaitWithTimeout(timeout time.Duration) error {
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
func (gm *Manager) GetRunningCount() int {
	return int(gm.running)
}

// GetStatus 获取任务状态
func (gm *Manager) GetStatus() map[string]interface{} {
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

// Task 处理任务
type Task struct {
	Index int
	ID    string
	Data  interface{}
}

// Result 处理结果
type Result struct {
	Index   int
	ID      string
	Data    interface{}
	Error   error
	Success bool
}

// ProcessFunc 处理函数类型
type ProcessFunc func(ctx context.Context, task *Task) (interface{}, error)

// Processor 并行处理器
type Processor struct {
	maxWorkers int
	timeout    time.Duration
	logger     *logrus.Entry
}

// NewProcessor 创建并行处理器
func NewProcessor(maxWorkers int, timeout time.Duration, logger *logrus.Entry) *Processor {
	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	return &Processor{
		maxWorkers: maxWorkers,
		timeout:    timeout,
		logger:     logger,
	}
}

// ProcessParallel 并行处理任务
func (p *Processor) ProcessParallel(ctx context.Context, tasks []*Task, processFunc ProcessFunc) []*Result {
	if len(tasks) == 0 {
		return []*Result{}
	}

	results := make([]*Result, len(tasks))
	for i := range results {
		results[i] = &Result{Index: i, Success: false}
	}

	taskChan := make(chan *Task, len(tasks))
	resultChan := make(chan *Result, len(tasks))

	var wg sync.WaitGroup
	for i := 0; i < p.maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.worker(ctx, workerID, taskChan, resultChan, processFunc)
		}(i)
	}

	go func() {
		for _, task := range tasks {
			taskChan <- task
		}
		close(taskChan)
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Index >= 0 && result.Index < len(results) {
			results[result.Index] = result
		}
	}

	return results
}

func (p *Processor) worker(ctx context.Context, workerID int, taskChan <-chan *Task, resultChan chan<- *Result, processFunc ProcessFunc) {
	for task := range taskChan {
		result := &Result{Index: task.Index, ID: task.ID, Success: false}

		taskCtx, cancel := context.WithTimeout(ctx, p.timeout)
		data, err := p.executeTask(taskCtx, task, processFunc)
		cancel()

		if err != nil {
			result.Error = err
			if p.logger != nil {
				p.logger.WithError(err).Warnf("Worker[%d] 任务[%s]处理失败", workerID, task.ID)
			}
		} else {
			result.Data = data
			result.Success = true
			if p.logger != nil {
				p.logger.Debugf("Worker[%d] 任务[%s]处理成功", workerID, task.ID)
			}
		}

		resultChan <- result
	}
}

func (p *Processor) executeTask(ctx context.Context, task *Task, processFunc ProcessFunc) (interface{}, error) {
	type res struct {
		data interface{}
		err  error
	}

	ch := make(chan res, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- res{err: fmt.Errorf("任务执行panic: %v", r)}
			}
		}()
		data, err := processFunc(ctx, task)
		ch <- res{data: data, err: err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("任务执行超时: %w", ctx.Err())
	}
}
