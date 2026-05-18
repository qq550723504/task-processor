// Package updater 提供自动更新器的更新逻辑管理功能
package updater

import (
	"fmt"
	"task-processor/internal/core/logger"
	"time"
)

// UpdateManager 更新逻辑管理器
type UpdateManager struct {
	currentVersion string
	adapter        AutoUpdateAdapter
	restartDelay   time.Duration
}

// NewUpdateManager 创建更新逻辑管理器
func NewUpdateManager(currentVersion, updateURL string, insecureSkipVerify bool) *UpdateManager {
	versionManager := NewVersionManager(currentVersion, updateURL)
	fileDownloader := NewFileDownloader(insecureSkipVerify)
	fileManager := NewFileManager()

	return &UpdateManager{
		currentVersion: currentVersion,
		adapter:        NewDefaultAutoUpdateAdapter(versionManager, fileDownloader, fileManager),
		restartDelay:   1 * time.Second,
	}
}

// CheckAndUpdate 检查并执行更新
func (um *UpdateManager) CheckAndUpdate() {
	logger.GetGlobalLogger("app/updater").Infof("检查更新... (当前版本: %s)", um.currentVersion)

	// 获取最新版本信息
	latestVersion, err := um.adapter.FetchLatestVersion()
	if err != nil {
		logger.GetGlobalLogger("app/updater").Errorf("获取版本信息失败: %v", err)
		um.adapter.SaveUpdateError(um.currentVersion, "获取版本信息失败", err)
		return
	}

	logger.GetGlobalLogger("app/updater").Infof("远程版本: %s", latestVersion.Version)

	// 检查是否有更新可用
	if !um.adapter.IsUpdateAvailable(latestVersion) {
		return
	}

	// 检查是否已经更新过这个版本（防止重复更新）
	if um.adapter.IsAlreadyUpdated(latestVersion.Version) {
		logger.GetGlobalLogger("app/updater").Infof("版本 %s 已经更新过，跳过", latestVersion.Version)
		return
	}

	// 下载新版本
	if err := um.adapter.DownloadAndStage(latestVersion); err != nil {
		logger.GetGlobalLogger("app/updater").Errorf("更新失败: %v", err)
		um.adapter.SaveUpdateError(um.currentVersion, fmt.Sprintf("更新到版本 %s 失败", latestVersion.Version), err)
		return
	}

	logger.GetGlobalLogger("app/updater").Info("更新成功，准备重启...")

	// 在重启前标记已更新（重要：必须在重启前创建标记文件）
	um.adapter.MarkApplied(latestVersion.Version)

	// 等待1秒确保文件写入完成
	time.Sleep(um.restartDelay)

	um.adapter.Restart()
}

// IsRecentlyUpdated 检查是否最近刚更新过
func (um *UpdateManager) IsRecentlyUpdated() bool {
	return um.adapter.IsRecentlyUpdated()
}
