package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/cmd/temu-web/server"
	"task-processor/common/auth"
	"task-processor/common/config"
	"task-processor/common/utils"
	"task-processor/updater"

	"github.com/sirupsen/logrus"
)

// 版本信息（通过 -ldflags 在编译时注入）
var (
	appVersion = "1.0.0" // 默认版本，编译时会被覆盖
	buildTime  = "unknown"

	clientCredentialsClient *auth.ClientCredentialsAuthClient
)

func main() {
	// Setup logger
	logger := utils.SetupLogger()

	// 显示版本信息
	logger.Infof("========================================")
	logger.Infof("Task Processor 启动")
	logger.Infof("版本: %s", appVersion)
	logger.Infof("构建时间: %s", buildTime)
	logger.Infof("========================================")

	// Load configuration
	cfg := config.LoadConfig()
	if cfg == nil {
		logger.Fatal("配置加载失败")
	}

	// 验证配置
	if !cfg.ValidateAndLog(logger) {
		logger.Fatal("配置验证失败，请检查配置文件")
	}

	// Start auto updater if enabled
	if cfg.Updater.Enabled {
		startAutoUpdater(cfg, logger, appVersion)
	} else {
		logger.Info("自动更新功能已禁用")
	}

	// Initialize client credentials auth
	initializeClientCredentialsAuth(cfg, logger)

	// Create server instance
	srv := server.New(cfg, logger)

	// Initialize server components with client credentials
	if err := srv.InitializeWithClientCredentials(clientCredentialsClient); err != nil {
		logger.Fatalf("服务器初始化失败: %v", err)
	}

	// Keep the program running
	logger.Info("✅ 任务处理器正在运行中...")
	logger.Info("按 Ctrl+C 停止程序")

	// 设置优雅关闭
	setupGracefulShutdown(srv, logger)
}

// setupGracefulShutdown 设置优雅关闭
func setupGracefulShutdown(srv *server.Server, logger *logrus.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	// 停止任务处理器
	if err := srv.StopProcessor(); err != nil {
		logger.Errorf("停止任务处理器失败: %v", err)
	}

	logger.Info("✅ 程序已优雅关闭")
	os.Exit(0)
}

func initializeClientCredentialsAuth(cfg *config.Config, logger *logrus.Logger) {
	logger.Info("初始化客户端凭证授权...")

	// 从配置中获取租户ID
	tenantID := cfg.Management.TenantID
	if tenantID == "" {
		tenantID = "1"
	}

	clientCredentialsClient = auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		tenantID,
	)

	// 立即获取一次token，验证配置是否正确
	_, err := clientCredentialsClient.GetAccessToken()
	if err != nil {
		logger.Fatalf("获取访问令牌失败: %v", err)
	}

}

func startAutoUpdater(cfg *config.Config, logger *logrus.Logger, currentVersion string) {
	logger.Info("启动自动更新器...")

	// 设置更新URL（如果配置中没有设置，使用默认值）
	updateURL := cfg.Updater.UpdateURL
	if updateURL == "" {
		updateURL = "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json"
		logger.Infof("使用默认更新地址: %s", updateURL)
	}

	// 设置检查间隔（如果配置中没有设置，使用默认值5分钟）
	checkInterval := time.Duration(cfg.Updater.CheckInterval) * time.Second
	if checkInterval <= 0 {
		checkInterval = 5 * time.Minute
		logger.Info("使用默认检查间隔: 5分钟")
	}

	// 创建更新器
	autoUpdater := updater.NewUpdater(
		currentVersion,
		updateURL,
		checkInterval,
		cfg.Updater.InsecureSkipVerify,
	)

	// 在后台启动更新检查
	go autoUpdater.Start()

	logger.Infof("自动更新器已启动 (当前版本: %s, 检查间隔: %v)", currentVersion, checkInterval)
}
