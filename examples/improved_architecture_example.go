// Package examples 提供改进后架构的使用示例
package examples

import (
	"context"
	"time"

	"task-processor/internal/container"
	"task-processor/internal/errors"
	"task-processor/internal/lifecycle"
	"task-processor/internal/monitoring"
	"task-processor/internal/task"

	"github.com/sirupsen/logrus"
)

// ExampleApplication 示例应用
type ExampleApplication struct {
	container        *container.Container
	metricsCollector *monitoring.MetricsCollector
	healthChecker    *monitoring.HealthChecker
	logger           *logrus.Logger
}

// NewExampleApplication 创建示例应用
func NewExampleApplication(logger *logrus.Logger) *ExampleApplication {
	return &ExampleApplication{
		container:        container.NewContainer(logger),
		metricsCollector: monitoring.NewMetricsCollector(logger, 30*time.Second),
		healthChecker:    monitoring.NewHealthChecker(logger, 60*time.Second),
		logger:           logger,
	}
}

// Run 运行示例应用
func (app *ExampleApplication) Run() error {
	// 1. 初始化容器
	if err := app.container.Initialize(); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "初始化容器失败")
	}

	// 2. 加载配置
	if err := app.container.LoadConfig(""); err != nil {
		return err
	}

	// 3. 初始化认证
	if err := app.container.InitializeAuth(); err != nil {
		return err
	}

	// 4. 设置监控
	if err := app.setupMonitoring(); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "设置监控失败")
	}

	// 5. 启动应用
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.container.StartAll(ctx); err != nil {
		return err
	}

	// 6. 启动监控组件
	if err := app.startMonitoring(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动监控失败")
	}

	app.logger.Info("✅ 示例应用启动完成")

	// 7. 模拟运行一段时间
	time.Sleep(10 * time.Second)

	// 8. 优雅关闭
	return app.gracefulShutdown()
}

// setupMonitoring 设置监控
func (app *ExampleApplication) setupMonitoring() error {
	// 注册健康检查
	app.healthChecker.RegisterCheck(&ConfigHealthCheck{
		container: app.container,
	})

	app.healthChecker.RegisterCheck(&DatabaseHealthCheck{})

	// 设置初始指标
	app.metricsCollector.SetGauge("app_info", 1, map[string]string{
		"version": "1.0.0",
		"env":     "development",
	}, "应用信息")

	return nil
}

// startMonitoring 启动监控
func (app *ExampleApplication) startMonitoring(ctx context.Context) error {
	// 启动指标收集器
	if err := app.metricsCollector.Start(ctx); err != nil {
		return err
	}

	// 启动健康检查器
	if err := app.healthChecker.Start(ctx); err != nil {
		return err
	}

	// 模拟业务指标更新
	go app.simulateBusinessMetrics(ctx)

	return nil
}

// simulateBusinessMetrics 模拟业务指标更新
func (app *ExampleApplication) simulateBusinessMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 模拟任务处理指标
			app.metricsCollector.IncrementCounter("tasks_processed_total",
				map[string]string{"platform": "temu"},
				"处理的任务总数")

			app.metricsCollector.IncrementCounter("tasks_processed_total",
				map[string]string{"platform": "shein"},
				"处理的任务总数")

			// 模拟错误指标
			if time.Now().Unix()%10 == 0 {
				app.metricsCollector.IncrementCounter("errors_total",
					map[string]string{"type": "network"},
					"网络错误总数")
			}

			// 模拟队列长度
			app.metricsCollector.SetGauge("queue_length", float64(time.Now().Unix()%20),
				map[string]string{"queue": "task_queue"},
				"任务队列长度")
		}
	}
}

// gracefulShutdown 优雅关闭
func (app *ExampleApplication) gracefulShutdown() error {
	app.logger.Info("开始优雅关闭...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止监控组件
	if err := app.metricsCollector.Stop(ctx); err != nil {
		app.logger.Errorf("停止指标收集器失败: %v", err)
	}

	if err := app.healthChecker.Stop(ctx); err != nil {
		app.logger.Errorf("停止健康检查器失败: %v", err)
	}

	// 停止容器中的所有组件
	if err := app.container.StopAll(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "停止容器组件失败")
	}

	app.logger.Info("✅ 优雅关闭完成")
	return nil
}

// ConfigHealthCheck 配置健康检查
type ConfigHealthCheck struct {
	container *container.Container
}

func (c *ConfigHealthCheck) Name() string {
	return "config"
}

func (c *ConfigHealthCheck) Check(ctx context.Context) error {
	cfg := c.container.GetConfig()
	if cfg == nil {
		return errors.New(errors.ErrCodeConfig, "配置未加载")
	}

	// 验证关键配置项
	if cfg.Worker.Concurrency <= 0 {
		return errors.New(errors.ErrCodeConfig, "工作池并发数配置无效")
	}

	if cfg.Management.BaseURL == "" {
		return errors.New(errors.ErrCodeConfig, "管理系统URL未配置")
	}

	return nil
}

// DatabaseHealthCheck 数据库健康检查示例
type DatabaseHealthCheck struct{}

func (d *DatabaseHealthCheck) Name() string {
	return "database"
}

func (d *DatabaseHealthCheck) Check(ctx context.Context) error {
	// 模拟数据库连接检查
	// 在实际应用中，这里应该是真实的数据库ping操作
	select {
	case <-time.After(100 * time.Millisecond):
		return nil // 模拟成功
	case <-ctx.Done():
		return errors.New(errors.ErrCodeTimeout, "数据库健康检查超时")
	}
}

// ExampleCustomComponent 自定义组件示例
type ExampleCustomComponent struct {
	*lifecycle.BaseComponent
	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// NewExampleCustomComponent 创建自定义组件
func NewExampleCustomComponent(logger *logrus.Logger) *ExampleCustomComponent {
	return &ExampleCustomComponent{
		BaseComponent: lifecycle.NewBaseComponent("ExampleCustomComponent"),
		logger:        logger,
	}
}

// Start 启动组件
func (c *ExampleCustomComponent) Start(ctx context.Context) error {
	if c.IsRunning() {
		return errors.New(errors.ErrCodeSystem, "ExampleCustomComponent已在运行")
	}

	c.logger.Info("启动自定义组件...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	// 启动后台任务
	go c.backgroundTask()

	c.SetRunning(true)
	c.logger.Info("自定义组件启动完成")
	return nil
}

// Stop 停止组件
func (c *ExampleCustomComponent) Stop(ctx context.Context) error {
	if !c.IsRunning() {
		return nil
	}

	c.logger.Info("停止自定义组件...")

	if c.cancel != nil {
		c.cancel()
	}

	c.SetRunning(false)
	c.logger.Info("自定义组件停止完成")
	return nil
}

// backgroundTask 后台任务
func (c *ExampleCustomComponent) backgroundTask() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("自定义组件后台任务停止")
			return
		case <-ticker.C:
			c.logger.Debug("自定义组件执行后台任务")
		}
	}
}

// ExampleTaskDeduplication 任务去重示例
func ExampleTaskDeduplication(logger *logrus.Logger) {
	// 创建去重管理器
	deduplicationManager := task.NewDeduplicationManager(logger, 24*time.Hour)

	// 启动去重管理器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deduplicationManager.Start(ctx)

	// 模拟任务提交
	taskID := "example_task_001"

	// 检查任务是否可以提交
	canSubmit, err := deduplicationManager.CanSubmitTask(taskID)
	if err != nil {
		logger.Errorf("检查任务失败: %v", err)
		return
	}

	if canSubmit {
		// 标记任务为处理中
		err := deduplicationManager.MarkTaskAsProcessing(taskID, "temu", 3)
		if err != nil {
			logger.Errorf("标记任务失败: %v", err)
			return
		}

		logger.Infof("任务 %s 已标记为处理中", taskID)

		// 模拟任务处理
		time.Sleep(2 * time.Second)

		// 标记任务完成
		deduplicationManager.MarkTaskAsCompleted(taskID)
		logger.Infof("任务 %s 已完成", taskID)
	} else {
		logger.Infof("任务 %s 不能提交", taskID)
	}

	// 获取任务统计
	stats := deduplicationManager.GetTaskStats()
	logger.Infof("任务统计: %+v", stats)

	// 停止去重管理器
	deduplicationManager.Stop()
}

// ExampleMonitoringUsage 监控功能使用示例
func ExampleMonitoringUsage(logger *logrus.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建指标收集器
	metricsCollector := monitoring.NewMetricsCollector(logger, 5*time.Second)

	// 创建健康检查器
	healthChecker := monitoring.NewHealthChecker(logger, 10*time.Second)

	// 启动监控组件
	if err := metricsCollector.Start(ctx); err != nil {
		logger.Errorf("启动指标收集器失败: %v", err)
		return
	}

	if err := healthChecker.Start(ctx); err != nil {
		logger.Errorf("启动健康检查器失败: %v", err)
		return
	}

	// 注册健康检查
	healthChecker.RegisterCheck(&ExampleHealthCheck{})

	// 模拟业务指标更新
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 更新指标
				metricsCollector.IncrementCounter("requests_total",
					map[string]string{"method": "GET", "status": "200"},
					"HTTP请求总数")

				metricsCollector.SetGauge("active_connections", float64(i*10),
					nil, "活跃连接数")

				logger.Infof("已更新指标，循环 %d", i+1)
			}
		}
	}()

	// 运行一段时间
	time.Sleep(25 * time.Second)

	// 获取最终指标
	metrics := metricsCollector.GetMetrics()
	logger.Infof("最终指标数量: %d", len(metrics))
	for name, metric := range metrics {
		logger.Infof("指标 %s: %f", name, metric.Value)
	}

	// 停止监控组件
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := metricsCollector.Stop(stopCtx); err != nil {
		logger.Errorf("停止指标收集器失败: %v", err)
	}

	if err := healthChecker.Stop(stopCtx); err != nil {
		logger.Errorf("停止健康检查器失败: %v", err)
	}

	logger.Info("监控功能示例完成")
}

// ExampleHealthCheck 示例健康检查
type ExampleHealthCheck struct{}

func (e *ExampleHealthCheck) Name() string {
	return "example_service"
}

func (e *ExampleHealthCheck) Check(ctx context.Context) error {
	// 模拟健康检查逻辑
	// 90%的概率返回健康状态
	if time.Now().Unix()%10 < 9 {
		return nil
	}
	return errors.New(errors.ErrCodeExternalAPI, "示例服务不可用")
}

// ExampleErrorHandling 错误处理示例
func ExampleErrorHandling(logger *logrus.Logger) {
	// 创建不同类型的错误
	configErr := errors.New(errors.ErrCodeConfig, "配置文件不存在")
	networkErr := errors.Wrap(configErr, errors.ErrCodeNetwork, "网络连接失败")

	// 检查错误类型
	if errors.IsCode(networkErr, errors.ErrCodeNetwork) {
		logger.Info("这是一个网络错误")
	}

	// 检查是否可重试
	if errors.IsRetryable(networkErr) {
		logger.Info("这个错误可以重试")
	}

	// 检查是否为关键错误
	if errors.IsCritical(configErr) {
		logger.Error("这是一个关键错误")
	}

	// 获取错误码
	code := errors.GetCode(networkErr)
	logger.Infof("错误码: %s", code)

	// 创建带详情的错误
	detailedErr := errors.New(errors.ErrCodeValidation, "数据验证失败").
		WithDetails("字段 'name' 不能为空").
		WithStack()

	logger.Errorf("详细错误: %+v", detailedErr)
}
