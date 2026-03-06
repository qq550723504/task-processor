// Package utils 提供工作池管理功能
package utils

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

// WorkerPool 工作池
type WorkerPool struct {
	maxWorkers int
	taskChan   chan *ProcessTask
	resultChan chan *ProcessResult
	wg         sync.WaitGroup
	logger     *logrus.Entry
}

// NewWorkerPool 创建工作池
func NewWorkerPool(maxWorkers, bufferSize int, logger *logrus.Entry) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 3
	}
	if bufferSize <= 0 {
		bufferSize = 100
	}

	return &WorkerPool{
		maxWorkers: maxWorkers,
		taskChan:   make(chan *ProcessTask, bufferSize),
		resultChan: make(chan *ProcessResult, bufferSize),
		logger:     logger,
	}
}

// Start 启动工作池
func (wp *WorkerPool) Start(ctx context.Context, processFunc ProcessFunc) {
	workerCount := wp.maxWorkers

	for i := 0; i < workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i, processFunc)
	}

	if wp.logger != nil {
		wp.logger.Debugf("🔧 工作池启动，工作协程数: %d", workerCount)
	}
}

// Submit 提交任务
func (wp *WorkerPool) Submit(task *ProcessTask) {
	wp.taskChan <- task
}

// Close 关闭任务通道
func (wp *WorkerPool) Close() {
	close(wp.taskChan)
}

// Wait 等待所有工作完成
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
	close(wp.resultChan)
}

// Results 获取结果通道
func (wp *WorkerPool) Results() <-chan *ProcessResult {
	return wp.resultChan
}

// worker 工作协程
func (wp *WorkerPool) worker(ctx context.Context, workerID int, processFunc ProcessFunc) {
	defer wp.wg.Done()

	if wp.logger != nil {
		wp.logger.WithField("worker_id", workerID).Debug("🔧 工作协程启动")
	}

	for task := range wp.taskChan {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			wp.resultChan <- &ProcessResult{
				Index:   task.Index,
				Error:   ctx.Err(),
				Success: false,
			}
			continue
		default:
		}

		// 执行任务
		data, err := processFunc(ctx, task)
		wp.resultChan <- &ProcessResult{
			Index:   task.Index,
			Data:    data,
			Error:   err,
			Success: err == nil,
		}
	}

	if wp.logger != nil {
		wp.logger.WithField("worker_id", workerID).Debug("🔧 工作协程结束")
	}
}
