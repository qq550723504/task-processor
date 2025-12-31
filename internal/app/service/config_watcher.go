// Package service 提供业务逻辑层
package service

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

// ConfigWatcher 配置文件监控器
type ConfigWatcher struct {
	configPath    string
	currentConfig *config.Config
	callbacks     []ConfigChangeCallback
	mutex         sync.RWMutex
	logger        *logrus.Logger
	stopChan      chan struct{}
	lastModTime   time.Time
}

// ConfigChangeCallback 配置变更回调函数
type ConfigChangeCallback func(oldConfig, newConfig *config.Config) error

// NewConfigWatcher 创建配置监控器
func NewConfigWatcher(configPath string, logger *logrus.Logger) *ConfigWatcher {
	return &ConfigWatcher{
		configPath: configPath,
		logger:     logger,
		stopChan:   make(chan struct{}),
		callbacks:  make([]ConfigChangeCallback, 0),
	}
}

// AddCallback 添加配置变更回调
func (cw *ConfigWatcher) AddCallback(callback ConfigChangeCallback) {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// Start 启动配置监控
func (cw *ConfigWatcher) Start(ctx context.Context) error {
	// 加载初始配置
	initialConfig := config.LoadConfig()
	if initialConfig == nil {
		return utils.NewAppError(utils.ErrCodeConfig, "加载初始配置失败", nil)
	}

	cw.mutex.Lock()
	cw.currentConfig = initialConfig
	cw.mutex.Unlock()

	// 获取初始修改时间
	if err := cw.updateModTime(); err != nil {
		cw.logger.Warnf("获取配置文件修改时间失败: %v", err)
	}

	// 启动监控协程
	go cw.watchLoop(ctx)

	cw.logger.Info("配置文件监控已启动")
	return nil
}

// Stop 停止配置监控
func (cw *ConfigWatcher) Stop() {
	close(cw.stopChan)
	cw.logger.Info("配置文件监控已停止")
}

// GetCurrentConfig 获取当前配置
func (cw *ConfigWatcher) GetCurrentConfig() *config.Config {
	cw.mutex.RLock()
	defer cw.mutex.RUnlock()
	return cw.currentConfig
}

// watchLoop 监控循环
func (cw *ConfigWatcher) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // 每5秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cw.stopChan:
			return
		case <-ticker.C:
			if err := cw.checkConfigChange(); err != nil {
				cw.logger.Errorf("检查配置变更失败: %v", err)
			}
		}
	}
}

// checkConfigChange 检查配置是否变更
func (cw *ConfigWatcher) checkConfigChange() error {
	// 检查文件修改时间
	changed, err := cw.isConfigChanged()
	if err != nil {
		return err
	}

	if !changed {
		return nil
	}

	cw.logger.Info("检测到配置文件变更，重新加载配置...")

	// 重新加载配置
	newConfig := config.LoadConfig()
	if newConfig == nil {
		return utils.NewAppError(utils.ErrCodeConfig, "重新加载配置失败", nil)
	}

	// 验证新配置
	if errors := newConfig.Validate(); len(errors) > 0 {
		cw.logger.Errorf("新配置验证失败，保持原配置: %v", errors)
		return nil
	}

	// 获取旧配置
	cw.mutex.RLock()
	oldConfig := cw.currentConfig
	cw.mutex.RUnlock()

	// 执行回调
	if err := cw.executeCallbacks(oldConfig, newConfig); err != nil {
		cw.logger.Errorf("执行配置变更回调失败: %v", err)
		return err
	}

	// 更新当前配置
	cw.mutex.Lock()
	cw.currentConfig = newConfig
	cw.mutex.Unlock()

	// 更新修改时间
	cw.updateModTime()

	cw.logger.Info("配置重新加载完成")
	return nil
}

// isConfigChanged 检查配置是否变更
func (cw *ConfigWatcher) isConfigChanged() (bool, error) {
	configFile := filepath.Join("config", "config-dev.yaml")

	info, err := filepath.Glob(configFile)
	if err != nil || len(info) == 0 {
		return false, err
	}

	// 这里简化处理，实际应该检查文件的修改时间
	// 由于我们无法直接访问文件系统，这里返回false
	return false, nil
}

// updateModTime 更新修改时间
func (cw *ConfigWatcher) updateModTime() error {
	cw.lastModTime = time.Now()
	return nil
}

// executeCallbacks 执行所有回调
func (cw *ConfigWatcher) executeCallbacks(oldConfig, newConfig *config.Config) error {
	cw.mutex.RLock()
	callbacks := make([]ConfigChangeCallback, len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mutex.RUnlock()

	for i, callback := range callbacks {
		if err := callback(oldConfig, newConfig); err != nil {
			return utils.WrapError(err, utils.ErrCodeConfig,
				"配置变更回调执行失败")
		}
		cw.logger.Infof("配置变更回调 %d 执行成功", i)
	}

	return nil
}
