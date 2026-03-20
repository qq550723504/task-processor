// package productenrich 提供 infra 层适配器：LLM、内存存储、1688爬虫。
// 供 cmd/productenrich-api 和 cmd/test-productenrich 共同使用。
package productenrich

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/infra/clients/openai"
)

// =============================================================================
// LLM 适配器
// =============================================================================

// LLMClientAdapter 将 openai.Client 适配为 LLMClient 接口。
type LLMClientAdapter struct {
	client *openai.Client
}

// Generate 实现 LLMClient.Generate。
func (a *LLMClientAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	return a.client.Generate(ctx, prompt)
}

// AnalyzeImage 实现 LLMClient.AnalyzeImage。
func (a *LLMClientAdapter) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return a.client.AnalyzeImage(ctx, imageURL, prompt)
}

// LLMManagerAdapter 将 openai.Manager 适配为 LLMManager 接口。
type LLMManagerAdapter struct {
	manager *openai.Manager
}

// GetClient 实现 LLMManager.GetClient。
func (a *LLMManagerAdapter) GetClient(clientName string) (LLMClient, error) {
	c, err := a.manager.GetClient(clientName)
	if err != nil {
		return nil, err
	}
	return &LLMClientAdapter{client: c}, nil
}

// GetDefaultClient 实现 LLMManager.GetDefaultClient。
func (a *LLMManagerAdapter) GetDefaultClient() LLMClient {
	return &LLMClientAdapter{client: a.manager.GetDefaultClient()}
}

// NewLLMManagerAdapter 根据 OpenAIConfig 创建 LLMManager。
func NewLLMManagerAdapter(cfg config.OpenAIConfig) (LLMManager, error) {
	clientCfgs := map[string]*openai.ClientConfig{
		"default": openai.NewClientConfig(cfg.APIKey, cfg.Model, cfg.BaseURL, cfg.Timeout),
	}
	for name, c := range cfg.Clients {
		apiKey := c.APIKey
		if apiKey == "" {
			apiKey = cfg.APIKey
		}
		baseURL := c.BaseURL
		if baseURL == "" {
			baseURL = cfg.BaseURL
		}
		timeout := c.Timeout
		if timeout == 0 {
			timeout = cfg.Timeout
		}
		clientCfgs[name] = openai.NewClientConfig(apiKey, c.Model, baseURL, timeout)
	}
	mgr, err := openai.NewManager(&openai.ManagerConfig{
		Clients:       clientCfgs,
		DefaultClient: "default",
	})
	if err != nil {
		return nil, fmt.Errorf("创建 OpenAI Manager 失败: %w", err)
	}
	return &LLMManagerAdapter{manager: mgr}, nil
}

// =============================================================================
// 内存实现（无需外部依赖，适合测试和降级场景）
// =============================================================================

type memRedisEntry struct {
	value     string
	expiresAt time.Time
}

// MemRedisClient 基于内存的 RedisClient 实现。
type MemRedisClient struct {
	mu    sync.RWMutex
	store map[string]memRedisEntry
	lists map[string][]string
}

// NewMemRedisClient 创建内存 RedisClient。
func NewMemRedisClient() RedisClient {
	return &MemRedisClient{
		store: make(map[string]memRedisEntry),
		lists: make(map[string][]string),
	}
}

// Push 实现 RedisClient.Push。
func (r *MemRedisClient) Push(_ context.Context, key string, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lists[key] = append(r.lists[key], value)
	return nil
}

// Get 实现 RedisClient.Get。
func (r *MemRedisClient) Get(_ context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.store[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return "", fmt.Errorf("key expired: %s", key)
	}
	return entry.value, nil
}

// Set 实现 RedisClient.Set。
func (r *MemRedisClient) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := memRedisEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	r.store[key] = entry
	return nil
}

// Delete 实现 RedisClient.Delete。
func (r *MemRedisClient) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, key)
	delete(r.lists, key)
	return nil
}

// MemTaskRepository 基于内存的 TaskRepository 实现。
type MemTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewMemTaskRepository 创建内存 TaskRepository。
func NewMemTaskRepository() TaskRepository {
	return &MemTaskRepository{tasks: make(map[string]*Task)}
}

// CreateTask 实现 TaskRepository.CreateTask。
func (r *MemTaskRepository) CreateTask(_ context.Context, task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return nil
}

// GetTask 实现 TaskRepository.GetTask。
func (r *MemTaskRepository) GetTask(_ context.Context, taskID string) (*Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	cp := *task
	return &cp, nil
}

// UpdateTaskStatus 实现 TaskRepository.UpdateTaskStatus。
func (r *MemTaskRepository) UpdateTaskStatus(_ context.Context, taskID string, status TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = status
	return nil
}

// UpdateTaskError 实现 TaskRepository.UpdateTaskError。
func (r *MemTaskRepository) UpdateTaskError(_ context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = TaskStatusFailed
	task.Error = errorMsg
	return nil
}

// SaveTaskResult 实现 TaskRepository.SaveTaskResult。
func (r *MemTaskRepository) SaveTaskResult(_ context.Context, taskID string, result *ProductJSON) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = TaskStatusCompleted
	task.Result = result
	return nil
}

// IncrementRetryCount 实现 TaskRepository.IncrementRetryCount。
func (r *MemTaskRepository) IncrementRetryCount(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.RetryCount++
	return nil
}

// ResetForRetry 实现 TaskRepository.ResetForRetry。
func (r *MemTaskRepository) ResetForRetry(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = TaskStatusPending
	return nil
}

// =============================================================================
// WebScraper 适配器（1688 爬虫）
// =============================================================================

// Crawler1688Adapter 将 alibaba1688.Alibaba1688Processor 适配为 WebScraper 接口。
type Crawler1688Adapter struct {
	processor *alibaba1688.Alibaba1688Processor
}

// NewCrawler1688Adapter 创建基于 1688 爬虫的 WebScraper。
func NewCrawler1688Adapter(cfg *config.Config) WebScraper {
	return &Crawler1688Adapter{
		processor: alibaba1688.NewAlibaba1688Processor(cfg),
	}
}

// Scrape 实现 WebScraper.Scrape。
func (a *Crawler1688Adapter) Scrape(_ context.Context, url string) (*ScrapedData, error) {
	product, err := a.processor.Process(url)
	if err != nil {
		return nil, fmt.Errorf("1688 scrape failed: %w", err)
	}

	specs := make(map[string]string, len(product.Specifications))
	for _, s := range product.Specifications {
		specs[s.Name] = s.Value
	}

	return &ScrapedData{
		Title:       product.Title,
		Description: buildCrawler1688Description(product),
		Images:      product.Images,
		Price:       product.MinPrice,
		Specs:       specs,
	}, nil
}

func buildCrawler1688Description(product *alibaba1688model.Product1688) string {
	if len(product.ProductDetails) == 0 {
		return product.Title
	}
	var sb strings.Builder
	for _, d := range product.ProductDetails {
		if d.Content != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(d.Content)
		}
	}
	if sb.Len() == 0 {
		return product.Title
	}
	return sb.String()
}
