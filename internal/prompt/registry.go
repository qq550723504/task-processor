// Package prompt 提供 Prompt 外置功能，包括加载、缓存、热更新和模板渲染。
package prompt

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"task-processor/internal/core/logger"
	"task-processor/internal/listingkit/tenantctx"

	"github.com/sirupsen/logrus"
)

// PromptRegistry Prompt 注册表，提供线程安全的 prompt 访问。
type PromptRegistry interface {
	// Get 获取 prompt；若 key 不存在则返回空字符串，不回落到 fallback。
	Get(key string, fallback string) string
	// Render 获取并渲染 prompt 模板；若 key 不存在或渲染失败则返回错误，不回落到 fallback。
	Render(key string, vars map[string]any, fallback string) (string, error)
	// GetTenant 获取指定租户的 prompt；缺失时返回错误，不回落到全局模板或调用点 fallback。
	GetTenant(tenantID string, key string) (string, error)
	// RenderTenant 获取并渲染指定租户的 prompt；缺失或渲染失败时返回错误，不回落。
	RenderTenant(tenantID string, key string, vars map[string]any) (string, error)
	// Keys 返回当前已加载的所有 key（用于调试）。
	Keys() []string
}

// registryImpl 是 PromptRegistry 的具体实现。
type registryImpl struct {
	mu          sync.RWMutex
	rawCache    map[string]string
	cache       map[string]string
	tenantCache map[string]map[string]string
	loader      PromptLoader
	renderer    TemplateRenderer
	store       TenantPromptStore
	watcher     FileWatcher
	log         *logrus.Entry
}

func (r *registryImpl) SetTenantPromptStore(store TenantPromptStore) {
	r.mu.Lock()
	r.store = store
	r.mu.Unlock()
}

// NewRegistry 创建一个新的 PromptRegistry 实例。
func NewRegistry(log *logrus.Entry) *registryImpl {
	if log == nil {
		log = logger.GetGlobalLogger("prompt.registry")
	}
	return &registryImpl{
		rawCache:    make(map[string]string),
		cache:       make(map[string]string),
		tenantCache: make(map[string]map[string]string),
		loader:      NewPromptLoader(),
		renderer:    NewTemplateRenderer(),
		log:         log,
	}
}

// Init 加载 promptsDir 下所有 YAML 文件，hotReload=true 时启动文件监听。
func (r *registryImpl) Init(ctx context.Context, promptsDir string, hotReload bool) error {
	data, err := r.loader.LoadAll(promptsDir)
	if err != nil {
		return fmt.Errorf("加载 prompts 目录失败: %w", err)
	}

	r.mu.Lock()
	r.rawCache = data
	r.cache, r.tenantCache = splitTenantPrompts(data)
	r.mu.Unlock()

	r.log.WithField("count", len(data)).Info("Prompt 缓存已加载")

	if !hotReload {
		return nil
	}

	w, err := NewFileWatcherWithLogger(r.log)
	if err != nil {
		r.log.WithError(err).Warn("创建 FileWatcher 失败，热更新已禁用")
		return nil
	}
	r.watcher = w

	if err := w.Watch(ctx, promptsDir, r.onFileChanged); err != nil {
		r.log.WithError(err).Warn("启动文件监听失败，热更新已禁用")
		return nil
	}

	return nil
}

// onFileChanged 是文件变更时的回调，重新加载变更文件并原子替换 cache。
func (r *registryImpl) onFileChanged(changedPath string) {
	newEntries, err := r.loader.LoadFile(changedPath)
	if err != nil {
		r.log.WithError(err).WithField("file", changedPath).Error("热更新：重新加载文件失败，保留旧缓存")
		return
	}

	r.mu.Lock()
	// 合并：复制旧 cache 再覆盖变更文件的 key
	merged := make(map[string]string, len(r.rawCache)+len(newEntries))
	maps.Copy(merged, r.rawCache)
	maps.Copy(merged, newEntries)
	r.rawCache = merged
	r.cache, r.tenantCache = splitTenantPrompts(merged)
	r.mu.Unlock()

	r.log.WithField("file", changedPath).Info("热更新：缓存已更新")
}

// Get 在读锁下查缓存，命中返回值；未命中返回空字符串并记录 ERROR 日志。
func (r *registryImpl) Get(key string, fallback string) string {
	r.mu.RLock()
	v, ok := r.cache[key]
	r.mu.RUnlock()

	if ok {
		return v
	}
	r.log.WithField("key", key).Error("Prompt key 未配置")
	return ""
}

// Render 先查 prompt 再渲染；缺失或渲染失败时记录 ERROR 并返回错误，不回落。
func (r *registryImpl) Render(key string, vars map[string]any, fallback string) (string, error) {
	r.mu.RLock()
	tmpl, ok := r.cache[key]
	r.mu.RUnlock()
	if !ok {
		err := fmt.Errorf("prompt %q not configured", key)
		r.log.WithError(err).WithField("key", key).Error("Prompt key 未配置")
		return "", err
	}
	result, err := r.renderer.Render(tmpl, vars)
	if err != nil {
		r.log.WithError(err).WithField("key", key).Error("Prompt 模板渲染失败")
		return "", err
	}
	return result, nil
}

func (r *registryImpl) GetTenant(tenantID string, key string) (string, error) {
	normalizedTenantID := tenantctx.NormalizeTenantID(tenantID)
	r.mu.RLock()
	tenantPrompts := r.tenantCache[normalizedTenantID]
	value, ok := tenantPrompts[key]
	r.mu.RUnlock()
	if ok {
		return value, nil
	}
	return "", fmt.Errorf("prompt %q not configured for tenant %q", key, normalizedTenantID)
}

func (r *registryImpl) RenderTenant(tenantID string, key string, vars map[string]any) (string, error) {
	tmpl, err := r.GetTenant(tenantID, key)
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{"tenant_id": tenantctx.NormalizeTenantID(tenantID), "key": key}).Error("租户 Prompt key 未配置")
		return "", err
	}
	result, err := r.renderer.Render(tmpl, vars)
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{"tenant_id": tenantctx.NormalizeTenantID(tenantID), "key": key}).Error("租户 Prompt 模板渲染失败")
		return "", err
	}
	return result, nil
}

func (r *registryImpl) GetTenantFromContext(ctx context.Context, key string) (string, error) {
	tenantID := tenantctx.TenantIDFromContext(ctx)
	r.mu.RLock()
	store := r.store
	r.mu.RUnlock()
	if store != nil {
		tmpl, err := store.GetEnabled(ctx, tenantID, key)
		if err != nil {
			return "", err
		}
		return tmpl.Content, nil
	}
	return r.GetTenant(tenantID, key)
}

func (r *registryImpl) RenderTenantFromContext(ctx context.Context, key string, vars map[string]any) (string, error) {
	tmpl, err := r.GetTenantFromContext(ctx, key)
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{"tenant_id": tenantctx.TenantIDFromContext(ctx), "key": key}).Error("租户 Prompt key 未配置")
		return "", err
	}
	result, err := r.renderer.Render(tmpl, vars)
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{"tenant_id": tenantctx.TenantIDFromContext(ctx), "key": key}).Error("租户 Prompt 模板渲染失败")
		return "", err
	}
	return result, nil
}

// Keys 在读锁下返回所有已加载的 key 切片（用于调试）。
func (r *registryImpl) Keys() []string {
	r.mu.RLock()
	keys := make([]string, 0, len(r.cache))
	for k := range r.cache {
		keys = append(keys, k)
	}
	r.mu.RUnlock()
	return keys
}

func splitTenantPrompts(data map[string]string) (map[string]string, map[string]map[string]string) {
	global := make(map[string]string)
	tenants := make(map[string]map[string]string)
	for key, value := range data {
		tenantID, promptKey, ok := parseTenantPromptKey(key)
		if !ok {
			global[key] = value
			continue
		}
		if tenants[tenantID] == nil {
			tenants[tenantID] = make(map[string]string)
		}
		tenants[tenantID][promptKey] = value
	}
	return global, tenants
}

func parseTenantPromptKey(key string) (string, string, bool) {
	const prefix = "tenants."
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return "", "", false
	}
	rest := key[len(prefix):]
	for idx, ch := range rest {
		if ch != '.' {
			continue
		}
		tenantID := tenantctx.NormalizeTenantID(rest[:idx])
		promptKey := rest[idx+1:]
		if promptKey == "" {
			return "", "", false
		}
		return tenantID, promptKey, true
	}
	return "", "", false
}

var ErrGlobalPromptRegistryNotInitialized = errors.New("prompt registry is not initialized")

func GetTenantFromContext(ctx context.Context, key string) (string, error) {
	if GlobalRegistry == nil {
		return "", ErrGlobalPromptRegistryNotInitialized
	}
	if registry, ok := GlobalRegistry.(interface {
		GetTenantFromContext(context.Context, string) (string, error)
	}); ok {
		return registry.GetTenantFromContext(ctx, key)
	}
	return GlobalRegistry.GetTenant(tenantctx.TenantIDFromContext(ctx), key)
}

func RenderTenantFromContext(ctx context.Context, key string, vars map[string]any) (string, error) {
	if GlobalRegistry == nil {
		return "", ErrGlobalPromptRegistryNotInitialized
	}
	if registry, ok := GlobalRegistry.(interface {
		RenderTenantFromContext(context.Context, string, map[string]any) (string, error)
	}); ok {
		return registry.RenderTenantFromContext(ctx, key, vars)
	}
	return GlobalRegistry.RenderTenant(tenantctx.TenantIDFromContext(ctx), key, vars)
}

func SetTenantPromptStore(store TenantPromptStore) error {
	if GlobalRegistry == nil {
		return ErrGlobalPromptRegistryNotInitialized
	}
	registry, ok := GlobalRegistry.(interface {
		SetTenantPromptStore(TenantPromptStore)
	})
	if !ok {
		return fmt.Errorf("global prompt registry does not support tenant prompt store")
	}
	registry.SetTenantPromptStore(store)
	return nil
}

// GlobalRegistry 是全局单例 PromptRegistry，由 InitGlobal 初始化。
var GlobalRegistry PromptRegistry

// InitGlobal 初始化全局单例 GlobalRegistry。
// log 为 nil 时使用默认 logger。
func InitGlobal(ctx context.Context, promptsDir string, hotReload bool, log *logrus.Entry) error {
	r := NewRegistry(log)
	if err := r.Init(ctx, promptsDir, hotReload); err != nil {
		return err
	}
	GlobalRegistry = r
	return nil
}
