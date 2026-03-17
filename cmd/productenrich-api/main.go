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
// TODO(dev): 接入真实的 TaskRepository、RedisClient、LLMManager 实现：
//  1. 初始化 Redis 客户端，实现 productenrich.RedisClient 接口
//  2. 实现 productenrich.TaskRepository（数据库持久化）
//  3. 实现 productenrich.LLMManager（接入 OpenAI 等）
//  4. 将上述依赖注入到 ProductServiceConfig
func buildHandler(logger *logrus.Logger) (productenrich.ProductHandler, error) {
	logger.Warn("⚠️  当前 TaskRepository 与 RedisClient 未注入，服务将无法正常处理请求，请完成依赖接入后再启动")

	svc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName: "product_enrich_tasks",
		// TODO(dev): 补全以下依赖
		// TaskRepo:             infra.NewTaskRepository(db),
		// RedisClient:          infra.NewRedisClient(redisCfg),
		// LLMManager:           infra.NewLLMManager(llmCfg),
	})
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
