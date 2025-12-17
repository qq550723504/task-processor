package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"task-processor/internal/common/utils"
	"task-processor/internal/service"
	internalUtils "task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// 版本信息（通过 -ldflags 在编译时注入）
var (
	appVersion = "1.0.0" // 默认版本，编译时会被覆盖
	buildTime  = "unknown"
)

func main() {
	// 初始化依赖
	deps := initializeDependencies()

	// 运行应用
	if err := deps.Run(); err != nil {
		deps.logger.Fatalf("应用启动失败: %v", err)
	}
}

// Dependencies 依赖容器
type Dependencies struct {
	logger           *logrus.Logger
	configService    *service.ConfigService
	authService      *service.AuthService
	updaterService   *service.UpdaterService
	processorService service.ProcessorService
}

// initializeDependencies 初始化依赖注入
func initializeDependencies() *Dependencies {
	// 设置日志
	logger := utils.SetupLogger()

	// 创建服务层
	configService := service.NewConfigService()
	authService := service.NewAuthService(logger)
	updaterService := service.NewUpdaterService(logger)
	processorService := service.NewProcessorService(logger)

	return &Dependencies{
		logger:           logger,
		configService:    configService,
		authService:      authService,
		updaterService:   updaterService,
		processorService: processorService,
	}
}

// Run 运行应用
func (d *Dependencies) Run() error {
	// 显示版本信息
	versionInfo := internalUtils.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	}
	internalUtils.PrintVersionInfo(d.logger, versionInfo)

	// 加载配置
	cfg := d.configService.LoadConfig("")
	if cfg == nil {
		d.logger.Fatal("配置加载失败")
	}

	// 验证配置
	if !cfg.ValidateAndLog(d.logger) {
		d.logger.Fatal("配置验证失败，请检查配置文件")
	}

	// 启动自动更新器
	d.updaterService.StartAutoUpdater(cfg, appVersion)

	// 初始化客户端凭证认证
	authClient, err := d.authService.InitializeClientCredentials(cfg)
	if err != nil {
		return err
	}

	// 启动任务处理器
	if err := d.processorService.StartProcessors(context.Background(), cfg, authClient); err != nil {
		return err
	}

	// 等待程序退出信号
	d.waitForShutdown()

	return nil
}

// waitForShutdown 等待程序退出信号
func (d *Dependencies) waitForShutdown() {
	// 导入必要的包
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	d.logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	// 停止任务处理器
	if err := d.processorService.StopProcessors(); err != nil {
		d.logger.Errorf("停止任务处理器失败: %v", err)
	}

	d.logger.Info("✅ 程序已优雅关闭")
}
