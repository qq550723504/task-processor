// Package client 提供TEMU平台认证暂停处理功能
package client

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// PauseHandler 暂停处理器接口
type PauseHandler interface {
	SetPauseKeyForAuthExpired(client ClientAPI, reason string) error
}

// TemuPauseHandler TEMU平台暂停处理器
type TemuPauseHandler struct {
	logger *logrus.Entry
}

// NewTemuPauseHandler 创建新的TEMU暂停处理器
func NewTemuPauseHandler(logger *logrus.Entry) *TemuPauseHandler {
	return &TemuPauseHandler{
		logger: logger,
	}
}

// SetPauseKeyForAuthExpired 设置认证过期暂停键
func (h *TemuPauseHandler) SetPauseKeyForAuthExpired(client ClientAPI, reason string) error {
	storeID := client.GetStoreID()
	h.logger.Infof("设置店铺 %d 的认证过期暂停键，原因: %s", storeID, reason)

	// 获取Cookie管理器
	cookieManagerInterface := client.GetCookieManager()
	cookieManager, ok := cookieManagerInterface.(*CookieManager)
	if !ok || cookieManager == nil {
		h.logger.Warn("Cookie管理器未初始化，无法设置暂停键")
		return fmt.Errorf("Cookie管理器未初始化")
	}

	// 获取管理客户端
	managementClient := cookieManager.GetManagementClient()
	if managementClient == nil {
		h.logger.Warn("管理客户端未初始化，无法设置暂停键")
		return fmt.Errorf("管理客户端未初始化")
	}

	// 获取店铺客户端
	storeClient := managementClient.GetStoreClient()
	if storeClient == nil {
		h.logger.Warn("店铺客户端未初始化，无法设置暂停键")
		return fmt.Errorf("店铺客户端未初始化")
	}

	// 调用管理系统API设置暂停状态
	success, err := storeClient.SetStorePauseStatus(storeID, true, "auth_expired")
	if err != nil {
		h.logger.Errorf("设置店铺 %d 的暂停状态失败: %v", storeID, err)
		return fmt.Errorf("设置暂停状态失败: %w", err)
	}

	if !success {
		h.logger.Warnf("设置店铺 %d 的暂停状态返回失败", storeID)
		return fmt.Errorf("设置暂停状态返回失败")
	}

	h.logger.Infof("✓ 成功设置店铺 %d 的认证过期暂停键", storeID)
	return nil
}

// validateClient 验证客户端及其依赖组件
func (h *TemuPauseHandler) validateClient(client ClientAPI) error {
	if client == nil {
		return fmt.Errorf("客户端为空")
	}

	cookieManagerInterface := client.GetCookieManager()
	if cookieManagerInterface == nil {
		return fmt.Errorf("Cookie管理器未初始化")
	}

	cookieManager, ok := cookieManagerInterface.(*CookieManager)
	if !ok {
		return fmt.Errorf("Cookie管理器类型错误")
	}

	if cookieManager.GetManagementClient() == nil {
		return fmt.Errorf("管理客户端未初始化")
	}

	storeClient := cookieManager.GetManagementClient().GetStoreClient()
	if storeClient == nil {
		return fmt.Errorf("店铺客户端未初始化")
	}

	return nil
}
