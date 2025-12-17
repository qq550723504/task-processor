// Package examples 展示日志系统和goroutine管理器的使用示例
package examples

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/goroutine"
	"task-processor/internal/logger"
	"task-processor/internal/scheduler"

	"github.com/sirupsen/logrus"
)

// LoggerGoroutineExample 日志和goroutine管理器使用示例
type LoggerGoroutineExample struct {
	logManager       *logger.LogManager
	goroutineManager *goroutine.GoroutineManager
	safeScheduler    *scheduler.SafeScheduler
	logger           *logrus.Entry
}

// NewLoggerGoroutineExample 创建示例
func NewLoggerGoroutineExample() *LoggerGoroutineExample {
	// 1. 初始化日志管理器
	logConfig := &logger.LogConfig{
		Level:      "info",
		Format:     "json",
		OutputFile: "logs/example.log",
		Console:    true,
	}
	logManager := logger.NewLogManager(logConfig)

	// 2. 获取组件日志记录器
	componentLogger := logManager.GetLogger("example")

	// 3. 创建上下文
	ctx := context.Background()

	// 4. 创建goroutine管理器
	goroutineManager := goroutine.NewGoroutineManager(ctx, componentLogger)

	// 5. 创建安全调度器
	safeScheduler := scheduler.NewSafeScheduler(ctx)

	return &LoggerGoroutineExample{
		logManager:       logManager,
		goroutineManager: goroutineManager,
		safeScheduler:    safeScheduler,
		logger:           componentLogger,
	}
}

// RunBasicExample 运行基础示例
func (e *LoggerGoroutineExample) RunBasicExample() {
	e.logger.Info("开始运行基础示例")

	// 示例1: 基础日志使用
	e.demonstrateLogging()

	// 示例2: goroutine管理
	e.demonstrateGoroutineManagement()

	// 示例3: 调度器使用
	e.demonstrateScheduler()

	e.logger.Info("基础示例运行完成")
}

// demonstrateLogging 演示日志使用
func (e *LoggerGoroutineExample) demonstrateLogging() {
	e.logger.Info("=== 日志系统示例 ===")

	// 基础日志
	e.logger.Debug("这是调试信息")
	e.logger.Info("这是普通信息")
	e.logger.Warn("这是警告信息")

	// 结构化日志
	e.logger.WithFields(logrus.Fields{
		"user_id":    12345,
		"action":     "login",
		"ip_address": "192.168.1.100",
		"timestamp":  time.Now(),
	}).Info("用户登录事件")

	// 错误日志
	err := fmt.Errorf("模拟错误")
	e.logger.WithError(err).Error("处理请求时发生错误")

	// 动态调整日志级别
	e.logger.Info("当前日志级别: " + e.logManager.GetLevel())
	e.logManager.SetLevel("debug")
	e.logger.Info("日志级别已调整为: " + e.logManager.GetLevel())
	e.logger.Debug("现在可以看到调试信息了")
}

// demonstrateGoroutineManagement 演示goroutine管理
func (e *LoggerGoroutineExample) demonstrateGoroutineManagement() {
	e.logger.Info("=== Goroutine管理示例 ===")

	// 启动一个简单的goroutine
	id1 := e.goroutineManager.Start("simple_task", func(ctx context.Context) error {
		e.logger.Info("执行简单任务")
		time.Sleep(2 * time.Second)
		e.logger.Info("简单任务完成")
		return nil
	})

	// 启动一个可重试的goroutine
	id2 := e.goroutineManager.StartWithRetry("retry_task", func(ctx context.Context, retryCount int) error {
		e.logger.WithField("retry_count", retryCount).Info("执行可重试任务")

		if retryCount < 2 {
			return fmt.Errorf("模拟失败，重试次数: %d", retryCount)
		}

		e.logger.Info("可重试任务成功完成")
		return nil
	})

	// 启动周期性goroutine
	id3 := e.goroutineManager.StartPeriodic("periodic_task", 3*time.Second, func(ctx context.Context) error {
		e.logger.Info("执行周期性任务")
		return nil
	})

	// 等待一段时间观察执行
	time.Sleep(10 * time.Second)

	// 获取状态
	status := e.goroutineManager.GetStatus()
	e.logger.WithField("goroutine_status", status).Info("Goroutine状态")

	// 停止特定goroutine
	e.goroutineManager.Stop(id3)
	e.logger.WithField("stopped_id", id3).Info("停止周期性任务")

	// 记录运行中的goroutine数量
	runningCount := e.goroutineManager.GetRunningCount()
	e.logger.WithField("running_count", runningCount).Info("当前运行中的goroutine数量")

	// 等待所有任务完成
	e.logger.Info("等待所有goroutine完成...")
	e.goroutineManager.WaitWithTimeout(15 * time.Second)

	e.logger.WithFields(logrus.Fields{
		"task1_id": id1,
		"task2_id": id2,
		"task3_id": id3,
	}).Info("Goroutine管理示例完成")
}

// demonstrateScheduler 演示调度器使用
func (e *LoggerGoroutineExample) demonstrateScheduler() {
	e.logger.Info("=== 安全调度器示例 ===")

	// 添加调度任务
	task1 := &scheduler.ScheduledTask{
		ID:       "data_sync",
		Name:     "数据同步任务",
		Interval: 5 * time.Second,
		Enabled:  true,
		Fn: func(ctx context.Context) error {
			e.logger.Info("执行数据同步")
			// 模拟数据同步工作
			time.Sleep(1 * time.Second)
			return nil
		},
	}

	task2 := &scheduler.ScheduledTask{
		ID:       "health_check",
		Name:     "健康检查任务",
		Interval: 3 * time.Second,
		Enabled:  true,
		Fn: func(ctx context.Context) error {
			e.logger.Info("执行健康检查")
			// 模拟健康检查
			return nil
		},
	}

	task3 := &scheduler.ScheduledTask{
		ID:       "cleanup",
		Name:     "清理任务",
		Interval: 10 * time.Second,
		Enabled:  false, // 初始禁用
		Fn: func(ctx context.Context) error {
			e.logger.Info("执行清理任务")
			return nil
		},
	}

	// 添加任务到调度器
	e.safeScheduler.AddTask(task1)
	e.safeScheduler.AddTask(task2)
	e.safeScheduler.AddTask(task3)

	// 启动调度器
	if err := e.safeScheduler.Start(); err != nil {
		e.logger.WithError(err).Error("启动调度器失败")
		return
	}

	// 运行一段时间
	time.Sleep(12 * time.Second)

	// 启用清理任务
	e.logger.Info("启用清理任务")
	e.safeScheduler.EnableTask("cleanup")

	// 再运行一段时间
	time.Sleep(8 * time.Second)

	// 获取调度器状态
	status := e.safeScheduler.GetStatus()
	e.logger.WithField("scheduler_status", status).Info("调度器状态")

	// 禁用数据同步任务
	e.logger.Info("禁用数据同步任务")
	e.safeScheduler.DisableTask("data_sync")

	// 再运行一段时间观察
	time.Sleep(5 * time.Second)

	// 停止调度器
	e.logger.Info("停止调度器")
	if err := e.safeScheduler.Stop(); err != nil {
		e.logger.WithError(err).Error("停止调度器失败")
	}

	e.logger.Info("调度器示例完成")
}

// RunAdvancedExample 运行高级示例
func (e *LoggerGoroutineExample) RunAdvancedExample() {
	e.logger.Info("开始运行高级示例")

	// 模拟复杂的业务场景
	e.simulateComplexScenario()

	e.logger.Info("高级示例运行完成")
}

// simulateComplexScenario 模拟复杂业务场景
func (e *LoggerGoroutineExample) simulateComplexScenario() {
	e.logger.Info("=== 复杂业务场景模拟 ===")

	// 模拟多个并发任务
	tasks := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{
			name: "user_data_processor",
			fn: func(ctx context.Context) error {
				e.logger.WithField("processor", "user_data").Info("处理用户数据")
				time.Sleep(2 * time.Second)
				return nil
			},
		},
		{
			name: "order_processor",
			fn: func(ctx context.Context) error {
				e.logger.WithField("processor", "order").Info("处理订单数据")
				time.Sleep(1 * time.Second)
				return nil
			},
		},
		{
			name: "notification_sender",
			fn: func(ctx context.Context) error {
				e.logger.WithField("processor", "notification").Info("发送通知")
				time.Sleep(500 * time.Millisecond)
				return nil
			},
		},
	}

	// 启动所有任务
	var taskIDs []string
	for _, task := range tasks {
		id := e.goroutineManager.Start(task.name, task.fn)
		taskIDs = append(taskIDs, id)
		e.logger.WithFields(logrus.Fields{
			"task_name": task.name,
			"task_id":   id,
		}).Info("启动业务处理任务")
	}

	// 等待所有任务完成
	e.logger.Info("等待所有业务任务完成...")
	e.goroutineManager.WaitWithTimeout(10 * time.Second)

	// 记录最终状态
	finalStatus := e.goroutineManager.GetStatus()
	e.logger.WithFields(logrus.Fields{
		"completed_tasks": taskIDs,
		"final_status":    finalStatus,
	}).Info("复杂业务场景处理完成")
}

// Cleanup 清理资源
func (e *LoggerGoroutineExample) Cleanup() {
	e.logger.Info("开始清理资源")

	// 停止所有goroutine
	e.goroutineManager.StopAll()
	e.goroutineManager.WaitWithTimeout(5 * time.Second)

	// 停止调度器
	e.safeScheduler.Stop()

	// 关闭日志管理器
	if err := e.logManager.Close(); err != nil {
		logrus.WithError(err).Error("关闭日志管理器失败")
	}

	e.logger.Info("资源清理完成")
}
