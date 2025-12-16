// Package dispatcher 提供处理器注册管理功能
package dispatcher

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// processorRegistry 处理器注册表实现
type processorRegistry struct {
	processors map[string]PlatformProcessor
	mutex      sync.RWMutex
	logger     *logrus.Logger
}

// NewProcessorRegistry 创建处理器注册表
func NewProcessorRegistry(logger *logrus.Logger) ProcessorRegistry {
	return &processorRegistry{
		processors: make(map[string]PlatformProcessor),
		logger:     logger,
	}
}

// Register 注册处理器
func (r *processorRegistry) Register(processor PlatformProcessor) error {
	if processor == nil {
		return fmt.Errorf("处理器不能为空")
	}

	platform := processor.GetPlatformName()
	if platform == "" {
		return fmt.Errorf("平台名称不能为空")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 检查是否已存在
	if _, exists := r.processors[platform]; exists {
		return fmt.Errorf("平台 %s 的处理器已存在", platform)
	}

	r.processors[platform] = processor
	r.logger.Infof("[Registry] 注册平台处理器: %s", platform)

	return nil
}

// Unregister 注销处理器
func (r *processorRegistry) Unregister(platform string) error {
	if platform == "" {
		return fmt.Errorf("平台名称不能为空")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	processor, exists := r.processors[platform]
	if !exists {
		return fmt.Errorf("平台 %s 的处理器不存在", platform)
	}

	// 停止处理器
	if status := processor.GetStatus(); status.Status == "running" {
		r.logger.Warnf("[Registry] 处理器 %s 仍在运行，建议先停止", platform)
	}

	delete(r.processors, platform)
	r.logger.Infof("[Registry] 注销平台处理器: %s", platform)

	return nil
}

// Get 获取处理器
func (r *processorRegistry) Get(platform string) (PlatformProcessor, error) {
	if platform == "" {
		return nil, fmt.Errorf("平台名称不能为空")
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	processor, exists := r.processors[platform]
	if !exists {
		return nil, fmt.Errorf("平台 %s 的处理器不存在", platform)
	}

	return processor, nil
}

// GetAll 获取所有处理器
func (r *processorRegistry) GetAll() map[string]PlatformProcessor {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 创建副本避免并发问题
	result := make(map[string]PlatformProcessor)
	for platform, processor := range r.processors {
		result[platform] = processor
	}

	return result
}

// List 列出所有平台名称
func (r *processorRegistry) List() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	platforms := make([]string, 0, len(r.processors))
	for platform := range r.processors {
		platforms = append(platforms, platform)
	}

	return platforms
}

// Count 获取处理器数量
func (r *processorRegistry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.processors)
}

// GetAvailableProcessors 获取可用的处理器
func (r *processorRegistry) GetAvailableProcessors() map[string]PlatformProcessor {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	available := make(map[string]PlatformProcessor)
	for platform, processor := range r.processors {
		status := processor.GetStatus()
		if status.Status == "running" && status.AvailableSlots > 0 {
			available[platform] = processor
		}
	}

	return available
}

// GetProcessorsByStatus 根据状态获取处理器
func (r *processorRegistry) GetProcessorsByStatus(status string) map[string]PlatformProcessor {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]PlatformProcessor)
	for platform, processor := range r.processors {
		if processor.GetStatus().Status == status {
			result[platform] = processor
		}
	}

	return result
}
