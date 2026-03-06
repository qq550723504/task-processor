// Package utils 提供并行任务处理工具
package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ParallelProcessor 并行处理器
type ParallelProcessor struct {
	maxWorkers int
	timeout    time.Duration
	logger     *logrus.Entry
}

// NewParallelProcessor 创建并行处理器
func NewParallelProcessor(maxWorkers int, timeout time.Duration, logger *logrus.Entry) *ParallelProcessor {
	if maxWorkers <= 0 {
		maxWorkers = 3 // 默认3个并发
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute // 默认2分钟超时
	}

	return &ParallelProcessor{
		maxWorkers: maxWorkers,
		timeout:    timeout,
		logger:     logger,
	}
}

// ProcessResult 处理结果
type ProcessResult struct {
	Index   int
	Data    interface{}
	Error   error
	Success bool
}

// ProcessTask 处理任务
type ProcessTask struct {
	Index int
	ID    string
	Data  interface{}
}

// ProcessFunc 处理函数类型
type ProcessFunc func(ctx context.Context, task *ProcessTask) (interface{}, error)

// ProcessParallel 并行处理任务
func (pp *ParallelProcessor) ProcessParallel(ctx context.Context, tasks []*ProcessTask, processFunc ProcessFunc) []*ProcessResult {
	if len(tasks) == 0 {
		return []*ProcessResult{}
	}

	// 创建性能跟踪器
	tracker := NewPerformanceTracker(fmt.Sprintf("并行处理-%d个任务", len(tasks)), pp.logger)
	defer tracker.Finish()

	tracker.StartStep("初始化并行处理")

	// 创建工作池
	workerCount := pp.maxWorkers
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}

	pool := NewWorkerPool(workerCount, len(tasks), pp.logger)

	// 包装处理函数以添加超时和日志
	wrappedFunc := pp.wrapProcessFunc(processFunc)
	pool.Start(ctx, wrappedFunc)

	tracker.EndStep()
	tracker.StartStep("分发任务")

	// 分发任务
	for _, task := range tasks {
		pool.Submit(task)
	}
	pool.Close()

	tracker.EndStep()
	tracker.StartStep("等待结果")

	// 等待所有工作完成
	go pool.Wait()

	// 收集结果
	results := make([]*ProcessResult, 0, len(tasks))
	for result := range pool.Results() {
		results = append(results, result)
	}

	tracker.EndStep()

	// 统计结果
	pp.logStatistics(results, len(tasks), workerCount)

	return results
}

// wrapProcessFunc 包装处理函数以添加超时和日志
func (pp *ParallelProcessor) wrapProcessFunc(processFunc ProcessFunc) ProcessFunc {
	return func(ctx context.Context, task *ProcessTask) (interface{}, error) {
		// 创建带超时的上下文
		taskCtx, cancel := context.WithTimeout(ctx, pp.timeout)
		defer cancel()

		start := time.Now()

		if pp.logger != nil {
			pp.logger.WithFields(logrus.Fields{
				"task_id": task.ID,
				"index":   task.Index,
			}).Info("📦 开始处理任务")
		}

		// 执行处理函数
		data, err := processFunc(taskCtx, task)
		duration := time.Since(start)

		// 记录结果
		if pp.logger != nil {
			fields := logrus.Fields{
				"task_id":     task.ID,
				"index":       task.Index,
				"duration":    duration.String(),
				"duration_ms": duration.Milliseconds(),
				"success":     err == nil,
			}

			if err != nil {
				fields["error"] = err.Error()
			}

			level := logrus.InfoLevel
			if err != nil {
				level = logrus.WarnLevel
			}

			pp.logger.WithFields(fields).Log(level, "✅ 任务处理完成")
		}

		return data, err
	}
}

// logStatistics 记录统计信息
func (pp *ParallelProcessor) logStatistics(results []*ProcessResult, totalTasks, workerCount int) {
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	if pp.logger != nil {
		pp.logger.WithFields(logrus.Fields{
			"total_tasks":   totalTasks,
			"success_count": successCount,
			"failed_count":  totalTasks - successCount,
			"success_rate":  fmt.Sprintf("%.1f%%", float64(successCount)/float64(totalTasks)*100),
			"worker_count":  workerCount,
		}).Info("🎉 并行处理完成")
	}
}

// BatchProcessor 批量处理器
type BatchProcessor struct {
	batchSize  int
	maxWorkers int
	timeout    time.Duration
	logger     *logrus.Entry
}

// NewBatchProcessor 创建批量处理器
func NewBatchProcessor(batchSize, maxWorkers int, timeout time.Duration, logger *logrus.Entry) *BatchProcessor {
	if batchSize <= 0 {
		batchSize = 10
	}
	if maxWorkers <= 0 {
		maxWorkers = 3
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	return &BatchProcessor{
		batchSize:  batchSize,
		maxWorkers: maxWorkers,
		timeout:    timeout,
		logger:     logger,
	}
}

// ProcessInBatches 分批并行处理
func (bp *BatchProcessor) ProcessInBatches(ctx context.Context, items []interface{}, processFunc func(ctx context.Context, item interface{}) (interface{}, error)) []interface{} {
	if len(items) == 0 {
		return []interface{}{}
	}

	tracker := NewPerformanceTracker(fmt.Sprintf("分批处理-%d个项目", len(items)), bp.logger)
	defer tracker.Finish()

	var allResults []interface{}

	// 分批处理
	for i := 0; i < len(items); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		tracker.StartStep(fmt.Sprintf("处理批次-%d (项目%d-%d)", i/bp.batchSize+1, i+1, end))

		// 创建任务
		tasks := make([]*ProcessTask, len(batch))
		for j, item := range batch {
			tasks[j] = &ProcessTask{
				Index: i + j,
				ID:    fmt.Sprintf("item-%d", i+j),
				Data:  item,
			}
		}

		// 并行处理批次
		processor := NewParallelProcessor(bp.maxWorkers, bp.timeout, bp.logger)
		results := processor.ProcessParallel(ctx, tasks, func(ctx context.Context, task *ProcessTask) (interface{}, error) {
			return processFunc(ctx, task.Data)
		})

		// 收集成功的结果
		for _, result := range results {
			if result.Success {
				allResults = append(allResults, result.Data)
			}
		}

		tracker.EndStep()
	}

	return allResults
}
