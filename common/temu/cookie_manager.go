package temu

import (
	"fmt"
	"net/http"
	"strings"
	"task-processor/common/management"

	"github.com/sirupsen/logrus"
)

// CookieManager Cookie管理器
type CookieManager struct {
	storeID          int64
	managementClient *management.Client
	logger           *logrus.Entry
}

// NewCookieManager 创建Cookie管理器
func NewCookieManager(storeID int64, managementClient *management.Client) *CookieManager {
	logger := logrus.WithFields(logrus.Fields{
		"component": "CookieManager",
		"storeID":   storeID,
	})

	return &CookieManager{
		storeID:          storeID,
		managementClient: managementClient,
		logger:           logger,
	}
}

// LoadCookies 从管理系统加载Cookie
func (cm *CookieManager) LoadCookies() ([]*http.Cookie, error) {
	cm.logger.WithField("storeID", cm.storeID).Debug("尝试从管理系统加载Cookie")

	// 检查管理系统客户端是否可用
	if cm.managementClient == nil {
		cm.logger.Error("管理系统客户端为空")
		return nil, fmt.Errorf("管理系统客户端为空")
	}

	// 从管理系统获取Cookie字符串
	cookieStr, err := cm.managementClient.GetStoreCookie(cm.storeID)
	if err != nil {
		cm.logger.WithError(err).WithField("storeID", cm.storeID).Error("从管理系统获取Cookie失败")
		// 检查是否是认证错误
		if strings.Contains(err.Error(), "访问令牌为空") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			return nil, fmt.Errorf("认证失败，无法获取Cookie: %w", err)
		}
		return nil, fmt.Errorf("从管理系统获取Cookie失败: %w", err)
	}

	if cookieStr == "" {
		cm.logger.WithField("storeID", cm.storeID).Info("未找到Cookie数据")
		return nil, nil
	}

	cm.logger.WithFields(logrus.Fields{
		"storeID":   cm.storeID,
		"cookieLen": len(cookieStr),
		"cookiePreview": func() string {
			if len(cookieStr) > 50 {
				return cookieStr[:50] + "..."
			}
			return cookieStr
		}(),
	}).Debug("获取到Cookie字符串")

	// 解析Cookie字符串
	cookies, err := cm.parseCookieString(cookieStr)
	if err != nil {
		cm.logger.WithError(err).Error("解析Cookie字符串失败")
		return nil, fmt.Errorf("解析Cookie字符串失败: %w", err)
	}

	cm.logger.WithFields(logrus.Fields{
		"storeID":   cm.storeID,
		"cookieNum": len(cookies),
	}).Info("成功从管理系统加载Cookie数据")

	return cookies, nil
}

// parseCookieString 解析Cookie字符串为http.Cookie对象
func (cm *CookieManager) parseCookieString(cookieStr string) ([]*http.Cookie, error) {
	if cookieStr == "" {
		return nil, nil
	}

	var cookies []*http.Cookie

	// 按分号分割cookie字符串
	cookiePairs := strings.Split(cookieStr, ";")

	for _, pair := range cookiePairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// 分割name=value
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			cm.logger.Warnf("跳过无效的Cookie格式: %s", pair)
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if name == "" {
			continue
		}

		cookie := &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: ".temu.com",
			Path:   "/",
		}

		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

// RefreshCookies 刷新Cookie（重新从管理系统获取）
func (cm *CookieManager) RefreshCookies() ([]*http.Cookie, error) {
	cm.logger.Info("刷新Cookie")
	return cm.LoadCookies()
}

// TestConnection 测试管理系统连接和认证状态
func (cm *CookieManager) TestConnection() error {
	cm.logger.Debug("测试管理系统连接")

	if cm.managementClient == nil {
		return fmt.Errorf("管理系统客户端为空")
	}

	// 尝试获取store信息来测试连接
	_, err := cm.managementClient.GetStore(cm.storeID)
	if err != nil {
		cm.logger.WithError(err).Error("管理系统连接测试失败")
		return fmt.Errorf("管理系统连接测试失败: %w", err)
	}

	cm.logger.Debug("管理系统连接测试成功")
	return nil
}
