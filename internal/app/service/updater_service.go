// Package service 提供业务逻辑层
package service

import (
	"time"

	"task-processor/internal/app/updater"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// UpdaterService 更新器服务
type UpdaterService struct {
	logger *logrus.Logger
}

// NewUpdaterService 创建更新器服务实例
func NewUpdaterService(logger *logrus.Logger) *UpdaterService {
	return &UpdaterService{
		logger: logger,
	}
}

// StartAutoUpdater 启动自动更新器
func (s *UpdaterService) StartAutoUpdater(cfg *config.Config, currentVersion string) {
	if !cfg.Updater.Enabled {
		s.logger.Info("自动更新功能已禁用")
		return
	}

	s.logger.Info("启动自动更新器...")

	// 设置更新URL
	updateURL := s.getUpdateURL(cfg)

	// 设置检查间隔
	checkInterval := s.getCheckInterval(cfg)

	// 创建更新器
	autoUpdater := updater.NewUpdater(
		currentVersion,
		updateURL,
		checkInterval,
		cfg.Updater.InsecureSkipVerify,
	)

	// 在后台启动更新检查
	go autoUpdater.Start()

	s.logger.Infof("自动更新器已启动 (当前版本: %s, 检查间隔: %v)", currentVersion, checkInterval)
}

// getUpdateURL 获取更新URL
func (s *UpdaterService) getUpdateURL(cfg *config.Config) string {
	updateURL := cfg.Updater.UpdateURL
	if updateURL == "" {
		updateURL = "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json"
		s.logger.Infof("使用默认更新地址: %s", updateURL)
	}
	return updateURL
}

// getCheckInterval 获取检查间隔
func (s *UpdaterService) getCheckInterval(cfg *config.Config) time.Duration {
	checkInterval := time.Duration(cfg.Updater.CheckInterval) * time.Second
	if checkInterval <= 0 {
		checkInterval = 5 * time.Minute
		s.logger.Info("使用默认检查间隔: 5分钟")
	}
	return checkInterval
}
