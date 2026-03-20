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
	"task-processor/internal/infra/worker"
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
	handler, pool, closers, err := buildHandler(logger)
	if err != nil {
		return fmt.Errorf("构建 handler 失败: %w", err)
	}
	defer func() {
		for _, close := range closers {
			if err := close(); err != nil {
				logger.Warnf("关闭资源失败: %v", err)
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 Worker Pool
	pool.Start(ctx)
	logger.Info("✅ Worker Pool 已启动")

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 先停止接收新请求
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("优雅关闭失败: %w", err)
	}

	// 再停止 Worker Pool，等待在途任务完成
	cancel()
	pool.Stop(shutdownCtx)
	logger.Info("✅ 服务已优雅关闭")
	return nil
}

// buildHandler 组装 productenrich 依赖并返回 ProductHandler、WorkerPool 和资源关闭函数列表。
// 优先使用配置文件中的 database/redis 配置；若未配置则回退到内存实现。
func buildHandler(logger *logrus.Logger) (productenrich.ProductHandler, worker.WorkerPool, []func() error, error) {
	cfg := config.LoadConfigFromFile(*configPath)
	var closers []func() error

	// LLM Manager（接入 OpenAI）
	llmMgr, err := newLLMManager(cfg.OpenAI)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 LLMManager 失败: %w", err)
	}
	logger.Info("✅ OpenAI LLMManager 已初始化")

	// 基于 LLMManager 构建各 LLM 依赖组件
	productUnderstanding, err := productenrich.NewProductUnderstanding(llmMgr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 ProductUnderstanding 失败: %w", err)
	}

	jsonGenerator, err := productenrich.NewJSONGenerator(logger, llmMgr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 JSONGenerator 失败: %w", err)
	}

	variantGenerator, err := productenrich.NewVariantGenerator(llmMgr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 VariantGenerator 失败: %w", err)
	}

	llmScorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{
		LLMManager: llmMgr,
	})
	qualityScorer := productenrich.NewQualityScorer(&productenrich.QualityScorerConfig{
		ImageWeight:   0.4,
		TextWeight:    0.3,
		ScrapedWeight: 0.3,
		LLMScorer:     llmScorer,
		EnableLLM:     true,
	})
	strategySelector := productenrich.NewStrategySelector(nil) // 使用默认阈值
	resultValidator := productenrich.NewResultValidator()
	enhancementSuggester := productenrich.NewEnhancementSuggester()
	inputValidator := productenrich.NewInputValidator(&productenrich.InputValidatorConfig{
		HTTPTimeout: 5 * time.Second,
		MaxWorkers:  10,
	})
	logger.Info("✅ LLM 相关组件已初始化")

	// TaskRepository
	var taskRepo productenrich.TaskRepository
	if cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, closers, fmt.Errorf("创建 TaskRepository 失败: %w", err)
		}
		taskRepo = repo
		closers = append(closers, closer)
	} else {
		logger.Warn("⚠️  未配置 database，TaskRepository 使用内存实现（重启后数据丢失）")
		taskRepo = newMemTaskRepository()
	}

	// RedisClient（仅作降级备用，主路径走 WorkerPool）
	var redisC productenrich.RedisClient
	if cfg.Redis != nil && cfg.Redis.Host != "" {
		rc, err := newRedisClient(cfg.Redis, logger)
		if err != nil {
			return nil, nil, closers, fmt.Errorf("创建 RedisClient 失败: %w", err)
		}
		redisC = rc
	} else {
		logger.Warn("⚠️  未配置 redis，RedisClient 使用内存实现")
		redisC = newMemRedisClient()
	}

	// WebScraper + InputParser（接入 1688 爬虫）
	webScraper := newWebScraper(cfg)
	inputParser, err := productenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, webScraper)
	if err != nil {
		return nil, nil, closers, fmt.Errorf("创建 InputParser 失败: %w", err)
	}
	logger.Info("✅ InputParser（1688爬虫）已初始化")

	// 先创建 service（不含 Pool，后续注入）
	svc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:            "product_enrich_tasks",
		TaskRepo:             taskRepo,
		RedisClient:          redisC,
		InputParser:          inputParser,
		ProductUnderstanding: productUnderstanding,
		JSONGenerator:        jsonGenerator,
		VariantGenerator:     variantGenerator,
		QualityScorer:        qualityScorer,
		StrategySelector:     strategySelector,
		ResultValidator:      resultValidator,
		EnhancementSuggester: enhancementSuggester,
		InputValidator:       inputValidator,
	})
	if err != nil {
		return nil, nil, closers, fmt.Errorf("创建 ProductService 失败: %w", err)
	}

	// 创建 Processor 并用 infra/worker.Pool 驱动
	proc, err := productenrich.NewProcessor(svc, taskRepo, logger, 3)
	if err != nil {
		return nil, nil, closers, fmt.Errorf("创建 Processor 失败: %w", err)
	}
	pool := worker.NewPoolWithConfig(proc, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * 60 * 1e9, // 15 分钟
		EnableMetrics:   true,
		ShutdownTimeout: 30 * 1e9,
	})
	logger.Infof("✅ Worker Pool 已创建（concurrency=%d）", cfg.Worker.Concurrency)

	// 将 Pool 注入 service，使 CreateGenerateTask 能直接 Submit
	svc.SetWorkerPool(pool)
	// 将 Pool 注入 proc，使重试时能重新入队
	proc.SetWorkerPool(pool)
	handler, err := productenrich.NewProductHandler(svc)
	if err != nil {
		return nil, nil, closers, fmt.Errorf("创建 ProductHandler 失败: %w", err)
	}
	return handler, pool, closers, nil
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
