// Package browser 提供浏览器实例管理功能
package browser

import (
	"fmt"
	sharedbrowser "task-processor/internal/crawler/shared/browser"
	"time"

	"github.com/sirupsen/logrus"
)

// InstanceManager 实例管理器
type InstanceManager struct {
	pool *BrowserPool
}

// NewInstanceManager 创建实例管理器
func NewInstanceManager(pool *BrowserPool) *InstanceManager {
	return &InstanceManager{
		pool: pool,
	}
}

// CreateInstance 创建浏览器实例
func (im *InstanceManager) CreateInstance(id int) (*BrowserInstance, error) {
	// 根据池配置决定使用的策略
	strategy := im.pool.poolConfig.FingerprintStrategy
	presetName := im.pool.poolConfig.PresetName

	// 使用新的配置管理器创建管理器
	manager := NewBrowserManagerWithConfig(im.pool.GetConfig(), strategy, presetName, id)

	// 如果启用随机指纹，为每个实例生成指纹
	if im.pool.UseRandomFingerprint() && im.pool.GetFingerprintGenerator() != nil {
		var fingerprint *sharedbrowser.FingerprintConfig

		// 根据策略决定指纹生成方式
		switch strategy {
		case "random":
			fingerprint = im.pool.GetFingerprintGenerator().GenerateRandomFingerprint("")
			logrus.Infof("实例 %d 使用随机指纹", id)
		case "stable":
			userID := fmt.Sprintf("instance_%d", id)
			fingerprint = im.pool.GetFingerprintGenerator().GenerateStableFingerprint(userID)
			logrus.Infof("实例 %d 使用稳定指纹 (用户ID: %s)", id, userID)
		default:
			// 默认使用随机指纹
			fingerprint = im.pool.GetFingerprintGenerator().GenerateRandomFingerprint("")
			logrus.Infof("实例 %d 使用默认随机指纹", id)
		}

		manager.SetFingerprint(fingerprint)

	}

	if err := manager.Install(); err != nil {
		return nil, fmt.Errorf("初始化playwright失败: %w", err)
	}

	if err := manager.Launch(); err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}

	page, err := manager.NewPage()
	if err != nil {
		manager.Close()
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	return &BrowserInstance{
		ID:      id,
		Manager: manager,
		Page:    page,
		InUse:   false,
	}, nil
}

// RecreateInstanceSync 同步重新创建浏览器实例（用于任务内重试）
func (im *InstanceManager) RecreateInstanceSync(oldInstance *BrowserInstance) *BrowserInstance {
	logrus.Infof("开始同步重新创建浏览器实例 %d", oldInstance.ID)

	// 先关闭旧实例
	if oldInstance.Manager != nil {
		oldInstance.Manager.Close()
		logrus.Infof("已关闭出现问题的浏览器实例 %d", oldInstance.ID)
	}

	// 等待短暂时间，让资源释放
	time.Sleep(2 * time.Second)

	// 创建新实例
	newInstance, err := im.CreateInstance(oldInstance.ID)
	if err != nil {
		logrus.Errorf("同步重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)

		// 如果创建失败，等待更长时间后再次尝试
		logrus.Infof("等待5秒后进行第二次创建尝试...")
		time.Sleep(5 * time.Second)

		newInstance, err = im.CreateInstance(oldInstance.ID)
		if err != nil {
			logrus.Errorf("第二次同步重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)
			// 返回nil，让调用方处理
			return nil
		}
	}

	// 更新实例列表
	im.pool.UpdateInstance(oldInstance, newInstance)

	logrus.Infof("✅ 成功同步重新创建浏览器实例 %d", oldInstance.ID)
	return newInstance
}

// RecreateInstanceAsync 异步重新创建浏览器实例（用于后台健康检查）
func (im *InstanceManager) RecreateInstanceAsync(oldInstance *BrowserInstance) {
	logrus.Infof("开始异步重新创建浏览器实例 %d", oldInstance.ID)

	// 异步重新创建实例，避免阻塞 - 添加panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("重新创建浏览器实例goroutine panic (实例ID: %d): %v", oldInstance.ID, r)
			}
		}()

		// 先关闭旧实例
		if oldInstance.Manager != nil {
			oldInstance.Manager.Close()
			logrus.Infof("已关闭被风控的浏览器实例 %d", oldInstance.ID)
		}

		// 等待一段时间再重新创建，避免立即重试
		time.Sleep(5 * time.Second)

		// 创建新实例
		newInstance, err := im.CreateInstance(oldInstance.ID)
		if err != nil {
			logrus.Infof("重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)

			// 如果创建失败，等待更长时间后再次尝试
			time.Sleep(30 * time.Second)
			newInstance, err = im.CreateInstance(oldInstance.ID)
			if err != nil {
				logrus.Infof("第二次重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)
				// 如果还是失败，可以考虑通知管理员或采取其他措施
				return
			}
		}

		// 更新实例列表
		im.pool.UpdateInstance(oldInstance, newInstance)

		// 将新实例放回可用池
		im.pool.GetAvailableChannel() <- newInstance
		logrus.Infof("✅ 成功异步重新创建浏览器实例 %d", oldInstance.ID)
	}()
}

// CloseInstance 关闭浏览器实例
func (im *InstanceManager) CloseInstance(instance *BrowserInstance) {
	if instance == nil {
		return
	}

	if instance.Manager != nil {
		instance.Manager.Close()
		logrus.Infof("已关闭浏览器实例 %d", instance.ID)
	}
}

// ValidateInstance 验证浏览器实例
func (im *InstanceManager) ValidateInstance(instance *BrowserInstance) bool {
	if instance == nil {
		return false
	}

	if instance.Manager == nil {
		logrus.Infof("浏览器实例 %d 管理器为空", instance.ID)
		return false
	}

	if instance.Page == nil {
		logrus.Infof("浏览器实例 %d 页面为空", instance.ID)
		return false
	}

	return true
}

// GetInstanceInfo 获取实例信息
func (im *InstanceManager) GetInstanceInfo(instance *BrowserInstance) map[string]any {
	if instance == nil {
		return map[string]any{
			"valid": false,
			"error": "instance is nil",
		}
	}

	info := map[string]any{
		"id":              instance.ID,
		"in_use":          instance.InUse,
		"current_zipcode": instance.CurrentZipcode,
		"valid":           im.ValidateInstance(instance),
	}

	if instance.Manager != nil {
		info["manager_exists"] = true
	} else {
		info["manager_exists"] = false
	}

	if instance.Page != nil {
		info["page_exists"] = true
	} else {
		info["page_exists"] = false
	}

	return info
}

// RestartInstance 重启浏览器实例
func (im *InstanceManager) RestartInstance(instance *BrowserInstance) (*BrowserInstance, error) {
	if instance == nil {
		return nil, fmt.Errorf("实例为空")
	}

	logrus.Infof("重启浏览器实例 %d", instance.ID)

	// 关闭旧实例
	im.CloseInstance(instance)

	// 等待资源释放
	time.Sleep(1 * time.Second)

	// 创建新实例
	newInstance, err := im.CreateInstance(instance.ID)
	if err != nil {
		return nil, fmt.Errorf("重启实例失败: %w", err)
	}

	logrus.Infof("✅ 成功重启浏览器实例 %d", instance.ID)
	return newInstance, nil
}
