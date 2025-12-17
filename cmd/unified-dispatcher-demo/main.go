// Package main 提供统一任务分发系统演示程序
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"task-processor/internal/common/utils"
	"task-processor/internal/config"
	"task-processor/internal/dispatcher"
	"task-processor/internal/dispatcher/adapters"
	"task-processor/internal/model"
	"task-processor/internal/service"
	"task-processor/internal/platforms/amazon"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

var (
	platform = flag.String("platform", "all", "目标平台 (amazon, temu, shein, all)")
	count    = flag.Int("count", 3, "测试任务数量")
	verbose  = flag.Bool("verbose", false, "详细日志输出")
)

func main() {
	flag.Parse()

	// 设置日志
	logger := utils.SetupLogger()
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
		logger.Info("🔧 启用详细日志模式")
	}

	logger.Info("🚀 统一任务分发系统演示开始")

	// 加载配置
	cfg := config.LoadConfig()
	if cfg == nil {
		logger.Fatal("❌ 配置加载失败")
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 初始化系统
	system, err := initializeSystem(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("❌ 系统初始化失败")
	}
	defer system.cleanup(ctx)

	// 启动系统
	if err := system.start(ctx); err != nil {
		logger.WithError(err).Fatal("❌ 系统启动失败")
	}

	// 运行演示
	if err := runDemo(ctx, system, *platform, *count); err != nil {
		logger.WithError(err).Fatal("❌ 演示运行失败")
	}

	logger.Info("✅ 统一任务分发系统演示完成")
}

// UnifiedSystem 统一系统
type UnifiedSystem struct {
	dispatcher  dispatcher.TaskDispatcher
	taskService service.TaskService
	logger      *logrus.Logger

	// 平台处理器
	amazonProcessor *amazon.Processor
	temuProcessor   *temu.TemuProcessor
	sheinProcessor  *shein.SheinProcessor
}

// initializeSystem 初始化系统
func initializeSystem(cfg *config.Config, logger *logrus.Logger) (*UnifiedSystem, error) {
	logger.Info("🔧 初始化统一任务分发系统")

	// 创建任务分发器
	dispatcherConfig := dispatcher.DefaultDispatcherConfig()
	taskDispatcher := dispatcher.NewTaskDispatcher(logger, dispatcherConfig)

	// 创建任务服务
	taskServiceConfig := service.DefaultTaskServiceConfig()
	taskService := service.NewTaskService(taskDispatcher, nil, logger, taskServiceConfig)

	system := &UnifiedSystem{
		dispatcher:  taskDispatcher,
		taskService: taskService,
		logger:      logger,
	}

	// 初始化平台处理器
	if err := system.initializePlatforms(cfg); err != nil {
		return nil, fmt.Errorf("初始化平台处理器失败: %w", err)
	}

	logger.Info("✅ 统一任务分发系统初始化完成")
	return system, nil
}

// initializePlatforms 初始化平台处理器
func (s *UnifiedSystem) initializePlatforms(cfg *config.Config) error {
	s.logger.Info("🔧 初始化平台处理器")

	// 初始化Amazon处理器
	s.amazonProcessor = amazon.NewProcessor(cfg, s.logger)
	amazonAdapter := adapters.NewAmazonProcessorAdapter(s.amazonProcessor, s.logger)
	if err := s.dispatcher.RegisterProcessor(amazonAdapter); err != nil {
		return fmt.Errorf("注册Amazon处理器失败: %w", err)
	}
	s.logger.Info("✅ Amazon处理器注册完成")

	// 初始化TEMU处理器
	s.temuProcessor = temu.NewTemuProcessor(cfg, s.logger)
	temuAdapter := adapters.NewTemuProcessorAdapter(s.temuProcessor, s.logger)
	if err := s.dispatcher.RegisterProcessor(temuAdapter); err != nil {
		return fmt.Errorf("注册TEMU处理器失败: %w", err)
	}
	s.logger.Info("✅ TEMU处理器注册完成")

	// 初始化SHEIN处理器
	s.sheinProcessor = shein.NewSheinProcessor(cfg)
	sheinAdapter := adapters.NewSheinProcessorAdapter(s.sheinProcessor, s.logger)
	if err := s.dispatcher.RegisterProcessor(sheinAdapter); err != nil {
		return fmt.Errorf("注册SHEIN处理器失败: %w", err)
	}
	s.logger.Info("✅ SHEIN处理器注册完成")

	return nil
}

// start 启动系统
func (s *UnifiedSystem) start(ctx context.Context) error {
	s.logger.Info("🚀 启动统一任务分发系统")

	// 启动分发器
	if err := s.dispatcher.Start(ctx); err != nil {
		return fmt.Errorf("启动分发器失败: %w", err)
	}

	s.logger.Info("✅ 统一任务分发系统启动完成")
	return nil
}

// cleanup 清理资源
func (s *UnifiedSystem) cleanup(ctx context.Context) {
	s.logger.Info("🧹 清理系统资源")

	if s.dispatcher != nil {
		if err := s.dispatcher.Stop(ctx); err != nil {
			s.logger.Errorf("停止分发器失败: %v", err)
		}
	}

	s.logger.Info("✅ 系统资源清理完成")
}

// runDemo 运行演示
func runDemo(ctx context.Context, system *UnifiedSystem, targetPlatform string, taskCount int) error {
	system.logger.Info("📦 开始任务分发演示")

	// 显示系统状态
	showSystemStatus(system)

	// 创建测试任务
	tasks := createTestTasks(targetPlatform, taskCount)
	system.logger.Infof("📋 创建了 %d 个测试任务", len(tasks))

	// 分发任务
	startTime := time.Now()
	for i, task := range tasks {
		system.logger.Infof("📤 分发任务 %d/%d: ID=%s, Platform=%s",
			i+1, len(tasks), task.ID, task.TargetPlatform)

		if err := system.taskService.SubmitTask(ctx, task); err != nil {
			system.logger.Errorf("❌ 任务分发失败: %v", err)
			continue
		}

		// 添加小延迟避免过快分发
		time.Sleep(100 * time.Millisecond)
	}

	duration := time.Since(startTime)
	system.logger.Infof("⏱️  任务分发完成，耗时: %v", duration)

	// 等待任务处理完成
	system.logger.Info("⏳ 等待任务处理完成...")
	time.Sleep(5 * time.Second)

	// 显示最终状态
	showFinalStatus(system)

	return nil
}

// createTestTasks 创建测试任务
func createTestTasks(targetPlatform string, count int) []*model.UnifiedTask {
	var tasks []*model.UnifiedTask

	platforms := []string{"amazon", "temu", "shein"}
	if targetPlatform != "all" {
		platforms = []string{targetPlatform}
	}

	taskID := 1
	for _, platform := range platforms {
		for i := 0; i < count; i++ {
			task := &model.UnifiedTask{
				ID:             fmt.Sprintf("demo-task-%03d", taskID),
				TenantID:       1,
				ProductID:      fmt.Sprintf("product-%s-%03d", platform, i+1),
				Platform:       "1688",
				TargetPlatform: platform,
				StoreID:        1001,
				CategoryID:     100,
				RawJSONData:    createTestProductData(platform, i+1),
				SourcePlatform: "1688",
				MarketplaceID:  "ATVPDKIKX0DER",
				LanguageTag:    "en_US",
				Currency:       "USD",
				Creator:        "demo-system",
				Priority:       5,
			}
			tasks = append(tasks, task)
			taskID++
		}
	}

	return tasks
}

// createTestProductData 创建测试产品数据
func createTestProductData(platform string, index int) string {
	return fmt.Sprintf(`{
		"title": "%s测试产品 %d - 韩版修身显瘦长袖连衣裙女装春秋新款",
		"brand": "时尚女装",
		"description": "优雅的韩版修身连衣裙，采用高品质面料，显瘦效果佳，适合春秋季节穿着。",
		"price": "199.00",
		"currency": "CNY",
		"color": "黑色",
		"size": "M",
		"material": "棉混纺",
		"category": "女装/连衣裙",
		"images": [
			"https://example.com/image1.jpg",
			"https://example.com/image2.jpg"
		]
	}`, platform, index)
}

// showSystemStatus 显示系统状态
func showSystemStatus(system *UnifiedSystem) {
	system.logger.Info("📊 系统状态概览:")

	statuses := system.dispatcher.GetAllProcessorStatus()
	for platform, status := range statuses {
		system.logger.Infof("  🔧 %s: 状态=%s, 可用槽位=%d",
			platform, status.Status, status.AvailableSlots)
	}

	platforms := system.dispatcher.GetSupportedPlatforms()
	system.logger.Infof("  🌐 支持的平台: %v", platforms)
}

// showFinalStatus 显示最终状态
func showFinalStatus(system *UnifiedSystem) {
	system.logger.Info("📈 最终处理状态:")

	statuses := system.dispatcher.GetAllProcessorStatus()
	for platform, status := range statuses {
		successRate := 0.0
		if status.TasksProcessed > 0 {
			successRate = float64(status.TasksSucceeded) / float64(status.TasksProcessed) * 100.0
		}

		system.logger.Infof("  📊 %s: 处理=%d, 成功=%d, 失败=%d, 成功率=%.1f%%",
			platform, status.TasksProcessed, status.TasksSucceeded,
			status.TasksFailed, successRate)
	}
}
