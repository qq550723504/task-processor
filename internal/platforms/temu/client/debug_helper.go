package client

import (
	"task-processor/internal/common/management"

	"github.com/sirupsen/logrus"
)

// DebugCookieIssue 调试Cookie问题的辅助函数
func DebugCookieIssue(storeID int64, managementClient *management.ClientManager) {
	logger := logrus.WithField("component", "CookieDebug")

	logger.Infof("开始调试StoreID=%d的Cookie问题", storeID)

	// 1. 检查管理系统客户端
	if managementClient == nil {
		logger.Error("❌ 管理系统客户端为空")
		return
	}
	logger.Info("✅ 管理系统客户端已初始化")

	// 2. 测试获取store信息
	storeClient := managementClient.GetStoreClient()
	store, err := storeClient.GetStore(storeID)
	if err != nil {
		logger.WithError(err).Error("❌ 获取store信息失败")
		return
	}
	logger.Infof("✅ 成功获取store信息: %+v", store)

	// 3. 测试获取cookie
	cookieStr, err := storeClient.GetStoreCookie(storeID)
	if err != nil {
		logger.WithError(err).Error("❌ 获取Cookie失败")
		return
	}

	if cookieStr == "" {
		logger.Warn("⚠️ Cookie字符串为空")
		return
	}

	logger.Infof("✅ 成功获取Cookie: 长度=%d, 内容预览=%s",
		len(cookieStr),
		func() string {
			if len(cookieStr) > 100 {
				return cookieStr[:100] + "..."
			}
			return cookieStr
		}())

	// 4. 测试解析cookie
	cm := NewCookieManager(storeID, managementClient)
	cookies, err := cm.parseCookieString(cookieStr)
	if err != nil {
		logger.WithError(err).Error("❌ 解析Cookie失败")
		return
	}

	logger.Infof("✅ 成功解析Cookie: 数量=%d", len(cookies))
	for i, cookie := range cookies {
		logger.Infof("  Cookie[%d]: Name=%s, Value=%s, Domain=%s, Path=%s",
			i, cookie.Name,
			func() string {
				if len(cookie.Value) > 20 {
					return cookie.Value[:20] + "..."
				}
				return cookie.Value
			}(),
			cookie.Domain, cookie.Path)
	}

	logger.Info("🎉 Cookie调试完成，一切正常")
}

// LogAPIClientStatus 记录API客户端状态
func LogAPIClientStatus(apiClient *APIClient, context string) {
	if apiClient == nil {
		logrus.Errorf("[%s] API客户端为空", context)
		return
	}

	logrus.WithFields(logrus.Fields{
		"context":     context,
		"tenantID":    apiClient.GetTenantID(),
		"storeID":     apiClient.GetStoreID(),
		"hasCookies":  apiClient.HasCookies(),
		"cookieCount": apiClient.GetCookieCount(),
		"baseURL":     apiClient.GetBaseURL(),
	}).Info("API客户端状态")
}
