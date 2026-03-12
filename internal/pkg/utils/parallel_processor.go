package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProcessTask 处理任务
type ProcessTask struct {
	Index int         // 任务索引
	ID    string      // 任务ID
	Data  interface{} // 任务数据
}

// ProcessResult 处理结果
type ProcessResult struct {
	Index   int         // 任务索引
	ID      string      // 任务ID
	Data    interface{} // 结果数据
	Error   error       // 错误信息
	Success bool        // 是否成功
}

// ProcessFunc 处理函数类型
type ProcessFunc func(ctx context.Context, task *ProcessTask) (interface{}, error)

// ParallelProcessor 并行处理器
type ParallelProcessor struct {
	maxWorkers int
	timeout    time.Duration
	logger     *logrus.Entry
}

// NewParallelProcessor 创建并行处理器
func NewParallelProcessor(maxWorkers int, timeout time.Duration, logger *logrus.Entry) *ParallelProcessor {
	if maxWorkers <= 0 {
		maxWorkers = 5 // 默认5个并发
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute // 默认5分钟超时
	}

	return &ParallelProcessor{
		maxWorkers: maxWorkers,
		timeout:    timeout,
		logger:     logger,
	}
}

// ProcessParallel 并行处理任务
func (p *ParallelProcessor) ProcessParallel(ctx context.Context, tasks []*ProcessTask, processFunc ProcessFunc) []*ProcessResult {
	if len(tasks) == 0 {
		return []*ProcessResult{}
	}

	// 创建结果切片
	results := make([]*ProcessResult, len(tasks))
	for i := range results {
		results[i] = &ProcessResult{
			Index:   i,
			Success: false,
		}
	}

	// 创建任务通道
	taskChan := make(chan *ProcessTask, len(tasks))
	resultChan := make(chan *ProcessResult, len(tasks))

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < p.maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.worker(ctx, workerID, taskChan, resultChan, processFunc)
		}(i)
	}

	// 发送任务
	go func() {
		for _, task := range tasks {
			taskChan <- task
		}
		close(taskChan)
	}()

	// 等待所有工作协程完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	for result := range resultChan {
		if result.Index >= 0 && result.Index < len(results) {
			results[result.Index] = result
		}
	}

	return results
}

// worker 工作协程
func (p *ParallelProcessor) worker(ctx context.Context, workerID int, taskChan <-chan *ProcessTask, resultChan chan<- *ProcessResult, processFunc ProcessFunc) {
	for task := range taskChan {
		result := &ProcessResult{
			Index:   task.Index,
			ID:      task.ID,
			Success: false,
		}

		// 创建带超时的上下文
		taskCtx, cancel := context.WithTimeout(ctx, p.timeout)

		// 执行任务
		data, err := p.executeTask(taskCtx, task, processFunc)
		cancel()

		if err != nil {
			result.Error = err
			result.Success = false
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

// executeTask 执行任务
func (p *ParallelProcessor) executeTask(ctx context.Context, task *ProcessTask, processFunc ProcessFunc) (interface{}, error) {
	// 使用通道来接收结果
	type result struct {
		data interface{}
		err  error
	}

	resultChan := make(chan result, 1)

	// 在新的goroutine中执行任务
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- result{
					data: nil,
					err:  fmt.Errorf("任务执行panic: %v", r),
				}
			}
		}()

		data, err := processFunc(ctx, task)
		resultChan <- result{data: data, err: err}
	}()

	// 等待结果或超时
	select {
	case res := <-resultChan:
		return res.data, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("任务执行超时: %w", ctx.Err())
	}
}
