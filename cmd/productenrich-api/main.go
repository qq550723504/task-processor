// Package main 提供商品信息增强（productenrich）HTTP API 服务入口
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
	"task-processor/internal/productenrich"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "配置文件路径")
	logLevel   = flag.String("log-level", "info", "日志级别")
	port       = flag.Int("port", 8085, "API 服务端口")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)

	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("🚀 启动商品信息增强 API 服务...")
	logger.Infof("📋 配置文件路径: %s", *configPath)
	logger.Infof("🌐 API 端口: %d", *port)

	if err := run(logger); err != nil {
		logger.Fatalf("❌ 服务启动失败: %v", err)
	}
}

func run(logger *logrus.Logger) error {
	handler, err := buildHandler(logger)
	if err != nil {
		return fmt.Errorf("构建 handler 失败: %w", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	registerRoutes(router, handler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: router,
	}

	go func() {
		logger.Infof("✅ 商品信息增强 API 服务已启动，监听端口 %d", *port)
		logger.Info("📊 API 端点:")
		logger.Info("   - POST /api/v1/products/generate       - 提交商品生成任务")
		logger.Info("   - GET  /api/v1/products/tasks/:task_id - 查询任务结果")
		logger.Info("   - GET  /health                         - 健康检查")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("❌ HTTP 服务异常退出: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("优雅关闭失败: %w", err)
	}

	logger.Info("✅ 服务已优雅关闭")
	return nil
}

// buildHandler 组装 productenrich 依赖并返回 ProductHandler。
// - TaskRepository：内存实现（如需持久化，替换为 productenrich.NewTaskRepository(db)）
// - RedisClient：内存实现（如需真实 Redis，替换为 newRedisClient(redisCfg)）
// - LLMManager：接入配置文件中的 OpenAI 配置
func buildHandler(logger *logrus.Logger) (productenrich.ProductHandler, error) {
	cfg := config.LoadConfigFromFile(*configPath)

	// LLM Manager（接入 OpenAI）
	llmMgr, err := newLLMManager(cfg.OpenAI.APIKey, cfg.OpenAI.Model, cfg.OpenAI.BaseURL, cfg.OpenAI.Timeout)
	if err != nil {
		return nil, fmt.Errorf("创建 LLMManager 失败: %w", err)
	}
	logger.Info("✅ OpenAI LLMManager 已初始化")

	// TaskRepository（内存实现）
	// TODO(dev): 替换为数据库实现：productenrich.NewTaskRepository(db)
	taskRepo := newMemTaskRepository()
	logger.Warn("⚠️  TaskRepository 使用内存实现，重启后数据丢失，生产环境请替换为数据库实现")

	// RedisClient（内存实现）
	// TODO(dev): 替换为真实 Redis 实现（需在 go.mod 引入 github.com/redis/go-redis/v9）
	redisClient := newMemRedisClient()
	logger.Warn("⚠️  RedisClient 使用内存实现，生产环境请替换为真实 Redis 实现")

	svc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:   "product_enrich_tasks",
		TaskRepo:    taskRepo,
		RedisClient: redisClient,
		// LLMManager 通过 JSONGenerator 等组件间接使用，此处暂不注入
		// 如需接入，请在 productenrich.ProductServiceConfig 中添加 LLMManager 字段
	})
	_ = llmMgr // 已初始化，待 ProductServiceConfig 支持后注入
	if err != nil {
		return nil, fmt.Errorf("创建 ProductService 失败: %w", err)
	}

	handler, err := productenrich.NewProductHandler(svc)
	if err != nil {
		return nil, fmt.Errorf("创建 ProductHandler 失败: %w", err)
	}
	return handler, nil
}

// registerRoutes 注册所有 API 路由
func registerRoutes(r *gin.Engine, h productenrich.ProductHandler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1/products")
	{
		v1.POST("/generate", h.GenerateProduct)
		v1.GET("/tasks/:task_id", h.GetTaskResult)
	}
}
