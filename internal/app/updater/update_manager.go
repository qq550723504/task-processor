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
	versionManager *VersionManager
	fileDownloader *FileDownloader
	fileManager    *FileManager
}

// NewUpdateManager 创建更新逻辑管理器
func NewUpdateManager(currentVersion, updateURL string, insecureSkipVerify bool) *UpdateManager {
	return &UpdateManager{
		currentVersion: currentVersion,
		versionManager: NewVersionManager(currentVersion, updateURL),
		fileDownloader: NewFileDownloader(insecureSkipVerify),
		fileManager:    NewFileManager(),
	}
}

// CheckAndUpdate 检查并执行更新
func (um *UpdateManager) CheckAndUpdate() {
	logger.GetGlobalLogger("app/updater").Infof("检查更新... (当前版本: %s)", um.currentVersion)

	// 获取最新版本信息
	latestVersion, err := um.versionManager.FetchLatestVersion()
	if err != nil {
		logger.GetGlobalLogger("app/updater").Errorf("获取版本信息失败: %v", err)
		um.fileManager.SaveUpdateError(um.currentVersion, "获取版本信息失败", err)
		return
	}

	logger.GetGlobalLogger("app/updater").Infof("远程版本: %s", latestVersion.Version)

	// 检查是否有更新可用
	if !um.versionManager.IsUpdateAvailable(latestVersion) {
		return
	}

	// 检查是否已经更新过这个版本（防止重复更新）
	if um.fileManager.IsAlreadyUpdated(latestVersion.Version) {
		logger.GetGlobalLogger("app/updater").Infof("版本 %s 已经更新过，跳过", latestVersion.Version)
		return
	}

	// 下载新版本
	if err := um.downloadAndUpdate(latestVersion); err != nil {
		logger.GetGlobalLogger("app/updater").Errorf("更新失败: %v", err)
		um.fileManager.SaveUpdateError(um.currentVersion, fmt.Sprintf("更新到版本 %s 失败", latestVersion.Version), err)
		return
	}

	logger.GetGlobalLogger("app/updater").Info("更新成功，准备重启...")

	// 在重启前标记已更新（重要：必须在重启前创建标记文件）
	um.fileManager.MarkAsUpdated(latestVersion.Version)

	// 等待1秒确保文件写入完成
	time.Sleep(1 * time.Second)

	um.fileManager.RestartProgram()
}

// downloadAndUpdate 下载并更新程序
func (um *UpdateManager) downloadAndUpdate(version *VersionInfo) error {
	tmpFile := um.fileDownloader.GetTempFilePath()

	// 重试最多3次下载
	maxRetries := 3
	err := um.fileDownloader.DownloadWithRetry(version.DownloadURL, tmpFile, version.SHA256, maxRetries)
	if err != nil {
		return err
	}

	// 替换可执行文件
	return um.fileManager.ReplaceExecutable(tmpFile, version.Version)
}

// IsRecentlyUpdated 检查是否最近刚更新过
func (um *UpdateManager) IsRecentlyUpdated() bool {
	return um.fileManager.IsRecentlyUpdated()
}
