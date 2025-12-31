// Package updater 提供自动更新器的核心功能
package updater

import (
	"time"

	"github.com/sirupsen/logrus"
)

// Updater 自动更新器
type Updater struct {
	currentVersion     string
	updateURL          string        // 版本检查地址
	checkInterval      time.Duration // 检查间隔
	insecureSkipVerify bool          // 跳过TLS证书验证
	updateManager      *UpdateManager
}

// NewUpdater 创建更新器
func NewUpdater(currentVersion, updateURL string, checkInterval time.Duration, insecureSkipVerify bool) *Updater {
	if insecureSkipVerify {
		logrus.Info("自动更新: TLS证书验证已禁用（避免证书问题导致更新失败）")
	}

	updateManager := NewUpdateManager(currentVersion, updateURL, insecureSkipVerify)

	return &Updater{
		currentVersion:     currentVersion,
		updateURL:          updateURL,
		checkInterval:      checkInterval,
		insecureSkipVerify: insecureSkipVerify,
		updateManager:      updateManager,
	}
}

// Start 启动自动更新检查
func (u *Updater) Start() {
	logrus.Infof("自动更新器后台任务启动，当前版本: %s", u.currentVersion)

	// 检查是否刚刚更新过（避免更新循环）
	if u.updateManager.IsRecentlyUpdated() {
		logrus.Info("检测到最近刚更新过，跳过启动时的更新检查")
	} else {
		// 延迟30秒后再检查（给程序启动留出时间）
		logrus.Info("将在30秒后进行首次更新检查...")
		time.Sleep(30 * time.Second)
		u.updateManager.CheckAndUpdate()
	}

	ticker := time.NewTicker(u.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		u.updateManager.CheckAndUpdate()
	}
}
