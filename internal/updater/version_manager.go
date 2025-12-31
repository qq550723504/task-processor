// Package updater 提供自动更新器的版本管理功能
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"task-processor/internal/utils"
	"time"

	"github.com/sirupsen/logrus"
)

// VersionManager 版本管理器
type VersionManager struct {
	currentVersion string
	updateURL      string
}

// NewVersionManager 创建版本管理器
func NewVersionManager(currentVersion, updateURL string) *VersionManager {
	return &VersionManager{
		currentVersion: currentVersion,
		updateURL:      updateURL,
	}
}

// FetchLatestVersion 获取最新版本信息
func (vm *VersionManager) FetchLatestVersion() (*VersionInfo, error) {
	// 使用统一的HTTP客户端工厂
	client := utils.CreateSimpleHTTPClientWithTimeout(30 * time.Second)

	resp, err := client.Get(vm.updateURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	var versionInfo VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %w", err)
	}

	return &versionInfo, nil
}

// CompareVersions 比较版本号
func (vm *VersionManager) CompareVersions(remoteVersion, localVersion string) int {
	return CompareVersion(remoteVersion, localVersion)
}

// IsUpdateAvailable 检查是否有更新可用
func (vm *VersionManager) IsUpdateAvailable(remoteVersion *VersionInfo) bool {
	cmp := CompareVersion(remoteVersion.Version, vm.currentVersion)
	if cmp <= 0 {
		logrus.Infof("当前已是最新版本 (本地: %s, 远程: %s)", vm.currentVersion, remoteVersion.Version)
		return false
	}

	logrus.Infof("发现新版本: %s -> %s", vm.currentVersion, remoteVersion.Version)
	logrus.Infof("更新日志: %s", remoteVersion.Changelog)
	return true
}

// CompareVersion 比较版本号 - 语义化版本比较
// 返回值: 1表示v1>v2, 0表示v1=v2, -1表示v1<v2
func CompareVersion(v1, v2 string) int {
	// 移除版本号前缀（如 "v1.0.0" -> "1.0.0"）
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// 分割版本号
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// 补齐版本号长度
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for len(parts1) < maxLen {
		parts1 = append(parts1, "0")
	}
	for len(parts2) < maxLen {
		parts2 = append(parts2, "0")
	}

	// 逐个比较版本号部分
	for i := 0; i < maxLen; i++ {
		num1, err1 := strconv.Atoi(parts1[i])
		num2, err2 := strconv.Atoi(parts2[i])

		// 如果解析失败，按字符串比较
		if err1 != nil || err2 != nil {
			if parts1[i] > parts2[i] {
				return 1
			} else if parts1[i] < parts2[i] {
				return -1
			}
			continue
		}

		if num1 > num2 {
			return 1
		} else if num1 < num2 {
			return -1
		}
	}

	return 0
}
