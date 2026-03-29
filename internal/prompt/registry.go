// Package prompt 提供 Prompt 外置功能，包括加载、缓存、热更新和模板渲染。
package prompt

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// PromptRegistry Prompt 注册表，提供线程安全的 prompt 访问。
type PromptRegistry interface {
	// Get 获取 prompt，若 key 不存在则返回 fallback。
	Get(key string, fallback string) string
	// Render 获取并渲染 prompt 模板，若 key 不存在则渲染 fallback；渲染失败时降级返回 fallback。
	Render(key string, vars map[string]any, fallback string) (string, error)
	// Keys 返回当前已加载的所有 key（用于调试）。
	Keys() []string
}

// registryImpl 是 PromptRegistry 的具体实现。
type registryImpl struct {
	mu       sync.RWMutex
	cache    map[string]string
	loader   PromptLoader
	renderer TemplateRenderer
	watcher  FileWatcher
	log      *logrus.Entry
}

// NewRegistry 创建一个新的 PromptRegistry 实例。
func NewRegistry(log *logrus.Entry) *registryImpl {
	if log == nil {
		log = logger.GetGlobalLogger("prompt.registry")
	}
	return &registryImpl{
		cache:    make(map[string]string),
		loader:   NewPromptLoader(),
		renderer: NewTemplateRenderer(),
		log:      log,
	}
}

// Init 加载 promptsDir 下所有 YAML 文件，hotReload=true 时启动文件监听。
func (r *registryImpl) Init(ctx context.Context, promptsDir string, hotReload bool) error {
	data, err := r.loader.LoadAll(promptsDir)
	if err != nil {
		return fmt.Errorf("加载 prompts 目录失败: %w", err)
	}

	r.mu.Lock()
	r.cache = data
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
	merged := make(map[string]string, len(r.cache)+len(newEntries))
	maps.Copy(merged, r.cache)
	maps.Copy(merged, newEntries)
	r.cache = merged
	r.mu.Unlock()

	r.log.WithField("file", changedPath).Info("热更新：缓存已更新")
}

// Get 在读锁下查缓存，命中返回值，未命中返回 fallback 并记录 DEBUG 日志。
func (r *registryImpl) Get(key string, fallback string) string {
	r.mu.RLock()
	v, ok := r.cache[key]
	r.mu.RUnlock()

	if ok {
		return v
	}
	r.log.WithField("key", key).Debug("Prompt key 未命中，使用 fallback")
	return fallback
}

// Render 先调 Get 拿到 prompt，再调 renderer.Render()；渲染失败时记录 ERROR 并降级返回 fallback。
func (r *registryImpl) Render(key string, vars map[string]any, fallback string) (string, error) {
	tmpl := r.Get(key, fallback)
	result, err := r.renderer.Render(tmpl, vars)
	if err != nil {
		r.log.WithError(err).WithField("key", key).Error("Prompt 模板渲染失败，降级返回 fallback")
		return fallback, err
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
