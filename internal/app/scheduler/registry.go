// Package scheduler 提供任务注册表功能
package scheduler

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// Registry 任务工厂注册表
type Registry struct {
	factories map[string]TaskFactory // key: platform
	mutex     sync.RWMutex
	logger    *logrus.Entry
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]TaskFactory),
		logger: logrus.WithFields(logrus.Fields{
			"component": "TaskRegistry",
		}),
	}
}

// Register 注册任务工厂
func (r *Registry) Register(factory TaskFactory) error {
	if factory == nil {
		return fmt.Errorf("factory不能为空")
	}

	platform := factory.SupportedPlatform()
	if platform == "" {
		return fmt.Errorf("平台名称不能为空")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.factories[platform]; exists {
		return fmt.Errorf("平台 %s 的工厂已注册", platform)
	}

	r.factories[platform] = factory
	r.logger.Infof("成功注册平台 %s 的任务工厂，支持任务类型: %v",
		platform, factory.SupportedTaskTypes())

	return nil
}

// GetFactory 获取任务工厂
func (r *Registry) GetFactory(platform string) (TaskFactory, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	factory, exists := r.factories[platform]
	if !exists {
		return nil, fmt.Errorf("未找到平台 %s 的任务工厂", platform)
	}

	return factory, nil
}

// ListPlatforms 列出所有已注册的平台
func (r *Registry) ListPlatforms() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	platforms := make([]string, 0, len(r.factories))
	for platform := range r.factories {
		platforms = append(platforms, platform)
	}

	return platforms
}

// GetSupportedTaskTypes 获取平台支持的任务类型
func (r *Registry) GetSupportedTaskTypes(platform string) ([]TaskType, error) {
	factory, err := r.GetFactory(platform)
	if err != nil {
		return nil, err
	}

	return factory.SupportedTaskTypes(), nil
}
