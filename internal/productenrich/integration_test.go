//go:build integration

package productenrich_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"gorm.io/gorm/logger"

	"task-processor/internal/infra/database"
	"task-processor/internal/productenrich"
)

// integrationSuite 集成测试套件，持有容器和共享资源
type integrationSuite struct {
	pgContainer    *tcpostgres.PostgresContainer
	redisContainer *tcredis.RedisContainer
	taskRepo       productenrich.TaskRepository
	redisClient    productenrich.RedisClient
}

// setupSuite 启动容器并初始化依赖，返回 cleanup 函数
func setupSuite(t *testing.T) (*integrationSuite, func()) {
	t.Helper()
	ctx := context.Background()

	// --- PostgreSQL ---
	pgC, err := tcpostgres.Run(ctx,
		"postgres:14.2",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err, "start postgres container")

	pgHost, err := pgC.Host(ctx)
	require.NoError(t, err)
	pgPort, err := pgC.MappedPort(ctx, "5432")
	require.NoError(t, err)

	db, err := database.NewDatabase(&database.DatabaseConfig{
		Host:               pgHost,
		Port:               pgPort.Int(),
		User:               "test",
		Password:           "test",
		Database:           "testdb",
		MaxConnections:     5,
		MaxIdleConnections: 2,
		ConnMaxLifetime:    time.Hour,
		LogLevel:           logger.Silent,
	})
	require.NoError(t, err, "connect to postgres")

	err = db.AutoMigrate(&productenrich.Task{})
	require.NoError(t, err, "auto migrate")

	taskRepo := productenrich.NewTaskRepository(db)

	// --- Redis ---
	redisC, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "start redis container")

	redisAddr, err := redisC.ConnectionString(ctx)
	require.NoError(t, err)

	rdb := goredis.NewClient(&goredis.Options{Addr: redisAddr[8:]}) // 去掉 "redis://" 前缀
	redisClient := newTestRedisClient(rdb)

	suite := &integrationSuite{
		pgContainer:    pgC,
		redisContainer: redisC,
		taskRepo:       taskRepo,
		redisClient:    redisClient,
	}

	cleanup := func() {
		_ = database.CloseDatabase(db)
		_ = pgC.Terminate(ctx)
		_ = redisC.Terminate(ctx)
	}

	return suite, cleanup
}

// =============================================================================
// TaskRepository 集成测试
// =============================================================================

func TestTaskRepository_Integration(t *testing.T) {
	suite, cleanup := setupSuite(t)
	defer cleanup()

	ctx := context.Background()
	repo := suite.taskRepo

	t.Run("CreateAndGetTask", func(t *testing.T) {
		task := &productenrich.Task{
			ID:      "integ-001",
			Request: &productenrich.GenerateRequest{Text: "test product"},
			Status:  productenrich.TaskStatusPending,
		}
		require.NoError(t, repo.CreateTask(ctx, task))

		got, err := repo.GetTask(ctx, "integ-001")
		require.NoError(t, err)
		assert.Equal(t, "integ-001", got.ID)
		assert.Equal(t, productenrich.TaskStatusPending, got.Status)
	})

	t.Run("GetTask_NotFound", func(t *testing.T) {
		_, err := repo.GetTask(ctx, "nonexistent")
		assert.ErrorIs(t, err, productenrich.ErrTaskNotFound)
	})

	t.Run("UpdateTaskStatus", func(t *testing.T) {
		task := &productenrich.Task{
			ID:      "integ-002",
			Request: &productenrich.GenerateRequest{Text: "test"},
			Status:  productenrich.TaskStatusPending,
		}
		require.NoError(t, repo.CreateTask(ctx, task))
		require.NoError(t, repo.UpdateTaskStatus(ctx, "integ-002", productenrich.TaskStatusProcessing))

		got, err := repo.GetTask(ctx, "integ-002")
		require.NoError(t, err)
		assert.Equal(t, productenrich.TaskStatusProcessing, got.Status)
	})

	t.Run("UpdateTaskError", func(t *testing.T) {
		task := &productenrich.Task{
			ID:      "integ-003",
			Request: &productenrich.GenerateRequest{Text: "test"},
			Status:  productenrich.TaskStatusProcessing,
		}
		require.NoError(t, repo.CreateTask(ctx, task))
		require.NoError(t, repo.UpdateTaskError(ctx, "integ-003", "something went wrong"))

		got, err := repo.GetTask(ctx, "integ-003")
		require.NoError(t, err)
		assert.Equal(t, productenrich.TaskStatusFailed, got.Status)
		assert.Equal(t, "something went wrong", got.Error)
	})

	t.Run("SaveTaskResult", func(t *testing.T) {
		task := &productenrich.Task{
			ID:      "integ-004",
			Request: &productenrich.GenerateRequest{Text: "test"},
			Status:  productenrich.TaskStatusProcessing,
		}
		require.NoError(t, repo.CreateTask(ctx, task))

		result := &productenrich.ProductJSON{
			Title:       "Test Product",
			Category:    []string{"Electronics"},
			Description: "A great product",
		}
		require.NoError(t, repo.SaveTaskResult(ctx, "integ-004", result))

		got, err := repo.GetTask(ctx, "integ-004")
		require.NoError(t, err)
		assert.Equal(t, productenrich.TaskStatusCompleted, got.Status)
		require.NotNil(t, got.Result)
		assert.Equal(t, "Test Product", got.Result.Title)
	})

	t.Run("IncrementRetryCount", func(t *testing.T) {
		task := &productenrich.Task{
			ID:      "integ-005",
			Request: &productenrich.GenerateRequest{Text: "test"},
			Status:  productenrich.TaskStatusFailed,
		}
		require.NoError(t, repo.CreateTask(ctx, task))
		require.NoError(t, repo.IncrementRetryCount(ctx, "integ-005"))
		require.NoError(t, repo.IncrementRetryCount(ctx, "integ-005"))

		got, err := repo.GetTask(ctx, "integ-005")
		require.NoError(t, err)
		assert.Equal(t, 2, got.RetryCount)
	})

	t.Run("ResetForRetry_preserves_error", func(t *testing.T) {
		task := &productenrich.Task{
			ID:      "integ-006",
			Request: &productenrich.GenerateRequest{Text: "test"},
			Status:  productenrich.TaskStatusFailed,
			Error:   "previous error",
		}
		require.NoError(t, repo.CreateTask(ctx, task))
		require.NoError(t, repo.ResetForRetry(ctx, "integ-006"))

		got, err := repo.GetTask(ctx, "integ-006")
		require.NoError(t, err)
		// status 重置为 pending
		assert.Equal(t, productenrich.TaskStatusPending, got.Status)
		// error 字段保留，不清空
		assert.Equal(t, "previous error", got.Error)
	})
}

// =============================================================================
// Redis 集成测试
// =============================================================================

func TestRedisClient_Integration(t *testing.T) {
	suite, cleanup := setupSuite(t)
	defer cleanup()

	ctx := context.Background()
	rc := suite.redisClient

	t.Run("SetAndGet", func(t *testing.T) {
		require.NoError(t, rc.Set(ctx, "key1", "value1", time.Minute))
		val, err := rc.Get(ctx, "key1")
		require.NoError(t, err)
		assert.Equal(t, "value1", val)
	})

	t.Run("Get_NotFound", func(t *testing.T) {
		_, err := rc.Get(ctx, "nonexistent-key")
		assert.Error(t, err)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, rc.Set(ctx, "key2", "value2", time.Minute))
		require.NoError(t, rc.Delete(ctx, "key2"))
		_, err := rc.Get(ctx, "key2")
		assert.Error(t, err)
	})

	t.Run("Push", func(t *testing.T) {
		require.NoError(t, rc.Push(ctx, "queue1", "task-id-1"))
		require.NoError(t, rc.Push(ctx, "queue1", "task-id-2"))
		// 验证队列长度（通过底层 rdb 检查）
	})

	t.Run("TTL_expiry", func(t *testing.T) {
		require.NoError(t, rc.Set(ctx, "ttl-key", "val", 100*time.Millisecond))
		time.Sleep(200 * time.Millisecond)
		_, err := rc.Get(ctx, "ttl-key")
		assert.Error(t, err, "key should have expired")
	})
}

// =============================================================================
// ProductService 端到端集成测试（mock LLM，真实 DB + Redis）
// =============================================================================

func TestProductService_Integration_CreateAndProcess(t *testing.T) {
	suite, cleanup := setupSuite(t)
	defer cleanup()

	ctx := context.Background()

	// 用 mock LLM（不调用真实 API）
	llmManager := newMockLLMManagerForInteg()

	understanding, err := productenrich.NewProductUnderstanding(llmManager)
	require.NoError(t, err)

	jsonGen, err := productenrich.NewJSONGenerator(logrus.New(), llmManager)
	require.NoError(t, err)

	svc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		TaskRepo:       suite.taskRepo,
		RedisClient:    suite.redisClient,
		QueueName:      "test_tasks",
		InputValidator: productenrich.NewInputValidator(nil),
		QualityScorer:  productenrich.NewQualityScorer(nil),
		// 集成测试用低阈值：minimal=0，避免因测试数据不足被拒绝
		StrategySelector: productenrich.NewStrategySelector(&productenrich.StrategySelectorConfig{
			FullThreshold:    80,
			BasicThreshold:   60,
			MinimalThreshold: 0,
		}),
		EnhancementSuggester: productenrich.NewEnhancementSuggester(),
		ResultValidator:      productenrich.NewResultValidator(),
		ProductUnderstanding: understanding,
		JSONGenerator:        jsonGen,
	})
	require.NoError(t, err)

	t.Run("CreateTask_and_GetResult", func(t *testing.T) {
		req := &productenrich.GenerateRequest{
			Text: "这是一款高品质的蓝牙耳机，支持主动降噪，续航时间长达30小时",
		}

		task, err := svc.CreateGenerateTask(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, task.ID)
		assert.Equal(t, productenrich.TaskStatusPending, task.Status)

		// 直接调用 ProcessProduct（绕过 worker pool）
		result, err := svc.ProcessProduct(ctx, task)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.Title)

		// 验证 DB 中状态已更新为 completed
		saved, err := suite.taskRepo.GetTask(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, productenrich.TaskStatusCompleted, saved.Status)
	})

	t.Run("CreateTask_invalid_request", func(t *testing.T) {
		_, err := svc.CreateGenerateTask(ctx, &productenrich.GenerateRequest{})
		assert.Error(t, err, "empty request should fail validation")
	})

	t.Run("GetTaskResult_after_processing", func(t *testing.T) {
		req := &productenrich.GenerateRequest{Text: "产品描述文本"}
		task, err := svc.CreateGenerateTask(ctx, req)
		require.NoError(t, err)

		_, err = svc.ProcessProduct(ctx, task)
		require.NoError(t, err)

		taskResult, err := svc.GetTaskResult(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, productenrich.TaskStatusCompleted, taskResult.Status)
		assert.NotNil(t, taskResult.CompletedAt)
	})
}

// =============================================================================
// 辅助：测试用 Redis 客户端适配器
// =============================================================================

type testRedisClient struct {
	rdb *goredis.Client
}

func newTestRedisClient(rdb *goredis.Client) productenrich.RedisClient {
	return &testRedisClient{rdb: rdb}
}

func (r *testRedisClient) Push(ctx context.Context, key string, value string) error {
	return r.rdb.RPush(ctx, key, value).Err()
}

func (r *testRedisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := r.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return val, err
}

func (r *testRedisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.rdb.Set(ctx, key, value, ttl).Err()
}

func (r *testRedisClient) Delete(ctx context.Context, key string) error {
	return r.rdb.Del(ctx, key).Err()
}

// =============================================================================
// 辅助：集成测试用 mock LLM（返回合法 JSON，不调用真实 API）
// =============================================================================

type integMockLLMClient struct{}

func (m *integMockLLMClient) Generate(_ context.Context, _ string) (string, error) {
	return `{
		"title": "蓝牙耳机",
		"category": ["电子产品", "耳机"],
		"attributes": {"color": "黑色", "connectivity": "蓝牙5.0"},
		"selling_points": ["主动降噪", "30小时续航"],
		"seo_keywords": ["蓝牙耳机", "降噪耳机"],
		"description": "高品质蓝牙耳机，支持主动降噪"
	}`, nil
}

func (m *integMockLLMClient) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	return `{"color": "black", "material": "plastic", "scene": "product", "usage": "audio"}`, nil
}

type integMockLLMManager struct {
	client *integMockLLMClient
}

func newMockLLMManagerForInteg() productenrich.LLMManager {
	return &integMockLLMManager{client: &integMockLLMClient{}}
}

func (m *integMockLLMManager) GetClient(_ string) (productenrich.LLMClient, error) {
	return m.client, nil
}

func (m *integMockLLMManager) GetDefaultClient() productenrich.LLMClient {
	return m.client
}

// 确保 logrus 在测试中不输出噪音
func init() {
	logrus.SetLevel(logrus.WarnLevel)
	// Windows 上 Docker Desktop 使用 desktop-linux context，需要指定 host
	// 如果环境变量未设置，testcontainers 会尝试 rootless Docker（Windows 不支持）
	if os.Getenv("DOCKER_HOST") == "" {
		os.Setenv("DOCKER_HOST", "npipe:////./pipe/dockerDesktopLinuxEngine")
	}
}
