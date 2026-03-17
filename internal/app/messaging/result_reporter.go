// Package messaging 提供任务结果上报功能
package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/model"
	"task-processor/internal/pkg/httpclient"

	"github.com/sirupsen/logrus"
)

// TaskResult 任务结果
type TaskResult struct {
	TaskID       int64          `json:"taskId"`
	Status       string         `json:"status"`       // success, failed, retry
	Message      string         `json:"message"`      // 结果消息
	Data         map[string]any `json:"data"`         // 结果数据
	ProcessTime  int64          `json:"processTime"`  // 处理时间（毫秒）
	ErrorCode    string         `json:"errorCode"`    // 错误代码
	ErrorMessage string         `json:"errorMessage"` // 错误消息
	RetryCount   int            `json:"retryCount"`   // 重试次数
	NodeID       string         `json:"nodeId"`       // 节点ID
	Timestamp    int64          `json:"timestamp"`    // 时间戳
}

// ResultReporter 结果上报器
type ResultReporter struct {
	httpClient  *http.Client
	reportURL   string
	retryConfig *config.RetryConfig
	logger      *logrus.Logger
	nodeID      string

	// 异步上报
	resultChan chan *TaskResult
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	stopped    bool       // 添加停止标志
	stopMutex  sync.Mutex // 保护停止标志的互斥锁

	// 统计信息
	stats      ReporterStats
	statsMutex sync.RWMutex
}

// ReporterStats 上报器统计信息
type ReporterStats struct {
	TotalReports   int64 `json:"total_reports"`
	SuccessReports int64 `json:"success_reports"`
	FailedReports  int64 `json:"failed_reports"`
	RetryReports   int64 `json:"retry_reports"`
}

// ReporterConfig 上报器配置
type ReporterConfig struct {
	ReportURL   string              `yaml:"report_url"`
	NodeID      string              `yaml:"node_id"`
	Timeout     time.Duration       `yaml:"timeout"`
	BufferSize  int                 `yaml:"buffer_size"`
	RetryConfig *config.RetryConfig `yaml:"retry"`
}

// NewResultReporter 创建结果上报器
func NewResultReporter(cfg ReporterConfig, logger *logrus.Logger) *ResultReporter {
	// 设置默认值
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1000
	}

	// 初始化重试配置
	if cfg.RetryConfig == nil {
		cfg.RetryConfig = config.DefaultRetryConfig()
	} else {
		// 设置重试配置的默认值
		if cfg.RetryConfig.MaxRetries == 0 {
			cfg.RetryConfig.MaxRetries = 3
		}
		if cfg.RetryConfig.InitialDelay == 0 {
			cfg.RetryConfig.InitialDelay = 2 * time.Second
		}
		if cfg.RetryConfig.MaxDelay == 0 {
			cfg.RetryConfig.MaxDelay = 30 * time.Second
		}
		if cfg.RetryConfig.BackoffFactor == 0 {
			cfg.RetryConfig.BackoffFactor = 2.0
		}
	}

	if cfg.NodeID == "" {
		cfg.NodeID = fmt.Sprintf("node-%d", time.Now().Unix())
	}

	// 创建HTTP客户端（使用自定义Transport配置）
	httpClient := httpclient.NewWithTransport(
		cfg.Timeout,
		&http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     30 * time.Second,
		},
	)

	return &ResultReporter{
		httpClient:  httpClient,
		reportURL:   cfg.ReportURL,
		retryConfig: cfg.RetryConfig,
		logger:      logger,
		nodeID:      cfg.NodeID,
		resultChan:  make(chan *TaskResult, cfg.BufferSize),
	}
}

// Start 启动结果上报器
func (rr *ResultReporter) Start(ctx context.Context) error {
	rr.ctx, rr.cancel = context.WithCancel(ctx)

	// 启动异步上报goroutine
	rr.wg.Add(1)
	go rr.reportWorker()

	rr.logger.Info("结果上报器启动完成")
	return nil
}

// Stop 停止结果上报器
func (rr *ResultReporter) Stop(ctx context.Context) error {
	rr.stopMutex.Lock()
	defer rr.stopMutex.Unlock()

	// 检查是否已经停止
	if rr.stopped {
		rr.logger.Info("结果上报器已经停止，跳过重复停止")
		return nil
	}

	rr.logger.Info("停止结果上报器...")
	rr.stopped = true

	// 取消上下文
	if rr.cancel != nil {
		rr.cancel()
	}

	// 关闭结果通道
	close(rr.resultChan)

	// 等待worker完成
	done := make(chan struct{})
	go func() {
		defer close(done)
		rr.wg.Wait()
	}()

	select {
	case <-done:
		rr.logger.Info("结果上报器停止完成")
	case <-ctx.Done():
		rr.logger.Warn("等待结果上报器停止超时")
		return fmt.Errorf("停止结果上报器超时")
	}

	return nil
}

// ReportSuccess 上报成功结果
func (rr *ResultReporter) ReportSuccess(task *model.Task, data map[string]any, processTime time.Duration) error {
	result := &TaskResult{
		TaskID:      task.ID,
		Status:      "success",
		Message:     "任务处理成功",
		Data:        data,
		ProcessTime: processTime.Milliseconds(),
		RetryCount:  task.RetryCount,
		NodeID:      rr.nodeID,
		Timestamp:   time.Now().UnixMilli(),
	}

	return rr.reportAsync(result)
}

// ReportFailure 上报失败结果
func (rr *ResultReporter) ReportFailure(task *model.Task, err error, processTime time.Duration) error {
	result := &TaskResult{
		TaskID:       task.ID,
		Status:       "failed",
		Message:      "任务处理失败",
		ProcessTime:  processTime.Milliseconds(),
		ErrorCode:    "TASK_FAILED",
		ErrorMessage: err.Error(),
		RetryCount:   task.RetryCount,
		NodeID:       rr.nodeID,
		Timestamp:    time.Now().UnixMilli(),
	}

	return rr.reportAsync(result)
}

// ReportRetry 上报重试结果
func (rr *ResultReporter) ReportRetry(task *model.Task, err error, processTime time.Duration) error {
	result := &TaskResult{
		TaskID:       task.ID,
		Status:       "retry",
		Message:      "任务需要重试",
		ProcessTime:  processTime.Milliseconds(),
		ErrorCode:    "TASK_RETRY",
		ErrorMessage: err.Error(),
		RetryCount:   task.RetryCount,
		NodeID:       rr.nodeID,
		Timestamp:    time.Now().UnixMilli(),
	}

	return rr.reportAsync(result)
}

// reportAsync 异步上报
func (rr *ResultReporter) reportAsync(result *TaskResult) error {
	select {
	case rr.resultChan <- result:
		rr.updateStats("total")
		return nil
	case <-rr.ctx.Done():
		return fmt.Errorf("上报器已停止")
	default:
		rr.logger.Warnf("结果上报缓冲区已满，丢弃任务结果: TaskID=%d", result.TaskID)
		return fmt.Errorf("上报缓冲区已满")
	}
}

// reportWorker 上报工作器
func (rr *ResultReporter) reportWorker() {
	defer rr.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			rr.logger.Errorf("结果上报器发生panic: %v", r)
		}
	}()

	rr.logger.Info("结果上报工作器启动")

	for {
		select {
		case <-rr.ctx.Done():
			rr.logger.Info("结果上报工作器停止")
			return
		case result, ok := <-rr.resultChan:
			if !ok {
				rr.logger.Info("结果通道已关闭，工作器退出")
				return
			}

			rr.processResult(result)
		}
	}
}

// processResult 处理结果
func (rr *ResultReporter) processResult(result *TaskResult) {
	defer func() {
		if r := recover(); r != nil {
			rr.logger.Errorf("处理结果发生panic: TaskID=%d, Panic=%v", result.TaskID, r)
		}
	}()

	rr.logger.Debugf("上报任务结果: TaskID=%d, Status=%s", result.TaskID, result.Status)

	// 带重试的上报
	err := rr.reportWithRetry(result)
	if err != nil {
		rr.logger.Errorf("上报任务结果失败: TaskID=%d, Error=%v", result.TaskID, err)
		rr.updateStats("failed")
	} else {
		rr.logger.Debugf("上报任务结果成功: TaskID=%d", result.TaskID)
		rr.updateStats("success")
	}
}

// reportWithRetry 带重试的上报
func (rr *ResultReporter) reportWithRetry(result *TaskResult) error {
	var lastErr error
	delay := rr.retryConfig.InitialDelay

	for attempt := 0; attempt <= rr.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			rr.logger.Debugf("重试上报任务结果: TaskID=%d, Attempt=%d/%d",
				result.TaskID, attempt, rr.retryConfig.MaxRetries)
			rr.updateStats("retry")

			// 等待重试延迟
			select {
			case <-rr.ctx.Done():
				return fmt.Errorf("上报被取消")
			case <-time.After(delay):
			}

			// 计算下次延迟（指数退避）
			delay = time.Duration(float64(delay) * rr.retryConfig.BackoffFactor)
			if delay > rr.retryConfig.MaxDelay {
				delay = rr.retryConfig.MaxDelay
			}
		}

		// 尝试上报
		err := rr.doReport(result)
		if err == nil {
			return nil
		}

		lastErr = err
		rr.logger.Debugf("上报尝试失败: TaskID=%d, Attempt=%d, Error=%v",
			result.TaskID, attempt+1, err)
	}

	return fmt.Errorf("上报失败，已达到最大重试次数: %w", lastErr)
}

// doReport 执行上报
func (rr *ResultReporter) doReport(result *TaskResult) error {
	// 序列化结果
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("序列化结果失败: %w", err)
	}

	// 创建请求上下文（使用默认10秒超时）
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(rr.ctx, timeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", rr.reportURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "task-processor-rabbitmq")
	req.Header.Set("X-Node-ID", rr.nodeID)

	// 发送请求
	resp, err := rr.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("上报失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	return nil
}

// updateStats 更新统计信息
func (rr *ResultReporter) updateStats(statType string) {
	rr.statsMutex.Lock()
	defer rr.statsMutex.Unlock()

	switch statType {
	case "total":
		rr.stats.TotalReports++
	case "success":
		rr.stats.SuccessReports++
	case "failed":
		rr.stats.FailedReports++
	case "retry":
		rr.stats.RetryReports++
	}
}

// GetStats 获取统计信息
func (rr *ResultReporter) GetStats() ReporterStats {
	rr.statsMutex.RLock()
	defer rr.statsMutex.RUnlock()

	return rr.stats
}

// GetNodeID 获取节点ID
func (rr *ResultReporter) GetNodeID() string {
	return rr.nodeID
}

