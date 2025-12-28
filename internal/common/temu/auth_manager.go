// Package temu 提供TEMU平台认证管理功能
package temu

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// AuthManager 认证管理器
type AuthManager struct {
	cookieManager *CookieManager
	logger        *logrus.Entry
}

// NewAuthManager 创建新的认证管理器
func NewAuthManager(cookieManager *CookieManager, logger *logrus.Entry) *AuthManager {
	return &AuthManager{
		cookieManager: cookieManager,
		logger:        logger,
	}
}

// SendRequestWithAuth 发送带认证的请求
func (a *AuthManager) SendRequestWithAuth(client APIClientInterface, request map[string]any, result any) error {
	// 检查Cookie
	if !client.HasCookies() {
		a.logger.Warnf("店铺ID=%d没有Cookie数据，尝试重新加载Cookie", client.GetStoreID())

		// 尝试重新加载Cookie
		if err := client.ReloadCookies(); err != nil {
			// Cookie加载失败，设置暂停键
			a.logger.Errorf("重新加载Cookie失败: %v", err)
			if pauseErr := a.setPauseKeyForAuthExpired(client, "从管理系统获取Cookie失败: Cookie数据为空"); pauseErr != nil {
				a.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}
			// 返回AuthExpiredError以便任务处理器识别并暂停任务
			return NewAuthExpiredError(
				fmt.Sprintf("店铺ID=%d没有Cookie数据且重新加载失败，请先在管理系统中设置Cookie", client.GetStoreID()),
				err,
			)
		}

		// 再次检查Cookie
		if !client.HasCookies() {
			// Cookie为空，设置暂停键
			a.logger.Warn("Cookie数据为空")
			if pauseErr := a.setPauseKeyForAuthExpired(client, "Cookie数据为空"); pauseErr != nil {
				a.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}
			// 返回AuthExpiredError以便任务处理器识别并暂停任务
			return NewAuthExpiredError(
				fmt.Sprintf("店铺ID=%d没有Cookie数据，请先在管理系统中设置Cookie", client.GetStoreID()),
				nil,
			)
		}

		a.logger.Infof("成功重新加载Cookie，数量: %d", client.GetCookieCount())
	} else {
		a.logger.Debugf("Cookie检查通过，数量: %d", client.GetCookieCount())
	}

	// 使用重试逻辑发送请求
	return a.sendRequestWithRetry(client, request, result)
}

// sendRequestWithRetry 发送请求（带重试逻辑）
func (a *AuthManager) sendRequestWithRetry(client APIClientInterface, request map[string]any, result any) error {
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		a.logger.Debugf("API调用尝试 %d/%d", attempt, maxRetries)

		err := a.sendRequestOnce(client, request, result)
		if err == nil {
			a.logger.Debugf("API调用成功，尝试次数: %d", attempt)
			return nil
		}

		a.logger.Warnf("API调用失败 (尝试 %d/%d): %v", attempt, maxRetries, err)

		// 如果是认证相关错误，尝试重新加载Cookie
		if a.isAuthenticationError(err) {
			a.logger.Infof("检测到认证错误，尝试重新加载Cookie...")
			if reloadErr := client.ReloadCookies(); reloadErr != nil {
				a.logger.Warnf("重新加载Cookie失败: %v", reloadErr)
				// 如果是最后一次尝试且Cookie加载失败，设置暂停键
				if attempt == maxRetries {
					a.logger.Error("所有重试均失败，设置认证过期暂停键")
					if pauseErr := a.setPauseKeyForAuthExpired(client, fmt.Sprintf("认证错误且Cookie重新加载失败: %v", reloadErr)); pauseErr != nil {
						a.logger.Errorf("设置暂停键失败: %v", pauseErr)
					}
				}
			} else {
				a.logger.Infof("成功重新加载Cookie，数量: %d", client.GetCookieCount())
			}
		}

		// 如果不是最后一次尝试，记录重试信息
		if attempt < maxRetries {
			a.logger.Debugf("准备重试...")
		}
	}

	return fmt.Errorf("API调用失败，已重试%d次", maxRetries)
}

// sendRequestOnce 发送单次请求
func (a *AuthManager) sendRequestOnce(client APIClientInterface, request map[string]any, result any) error {
	// 从request map中提取参数
	method, ok := request["method"].(string)
	if !ok {
		return fmt.Errorf("请求方法不能为空")
	}

	url, ok := request["url"].(string)
	if !ok {
		return fmt.Errorf("请求URL不能为空")
	}

	headers, _ := request["headers"].(map[string]string)
	body := request["body"]
	formFields, _ := request["formFields"].(map[string]string)
	fileFields, _ := request["fileFields"].(map[string]any)

	// 构造完整URL
	fullURL := client.GetConfig().BaseURL + url

	// 发送HTTP请求
	response, err := client.SendHTTPRequest(method, fullURL, headers, body, formFields, fileFields)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}

	// 检查HTTP状态码
	httpManager := &HTTPManager{}
	if !httpManager.IsSuccess(response) {
		// 尝试读取错误响应体
		if errorBody, err := response.ToBytes(); err == nil {
			a.logger.Errorf("HTTP请求失败，状态码: %d, 响应体: %s", response.StatusCode, string(errorBody))
		}
		return fmt.Errorf("HTTP请求失败，状态码: %d", response.StatusCode)
	}

	// 解析响应体
	respBody, err := response.ToBytes()
	if err != nil {
		return fmt.Errorf("读取响应体失败: %w", err)
	}

	return json.Unmarshal(respBody, result)
}

// isAuthenticationError 判断是否为认证相关错误
func (a *AuthManager) isAuthenticationError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	// 检查常见的认证错误关键词
	authErrors := []string{
		"401",
		"403",
		"unauthorized",
		"forbidden",
		"登录",
		"认证",
		"权限",
		"cookie",
		"signature",
		"expired",
		"签名",
		"过期",
	}

	for _, keyword := range authErrors {
		if strings.Contains(errStr, keyword) {
			a.logger.Debugf("检测到认证错误关键词: %s", keyword)
			return true
		}
	}

	return false
}

// setPauseKeyForAuthExpired 设置认证过期暂停键
func (a *AuthManager) setPauseKeyForAuthExpired(client APIClientInterface, reason string) error {
	cookieManager := client.GetCookieManager()
	if cookieManager == nil || cookieManager.managementClient == nil {
		a.logger.Warn("管理客户端未初始化，无法设置暂停键")
		return fmt.Errorf("管理客户端未初始化")
	}

	// 调用管理系统API设置暂停状态
	storeClient := cookieManager.managementClient.GetStoreClient()
	if storeClient == nil {
		a.logger.Warn("店铺客户端未初始化，无法设置暂停键")
		return fmt.Errorf("店铺客户端未初始化")
	}

	storeID := client.GetStoreID()
	a.logger.Infof("设置店铺 %d 的认证过期暂停键，原因: %s", storeID, reason)
	success, err := storeClient.SetStorePauseStatus(storeID, true, "auth_expired")
	if err != nil {
		a.logger.Errorf("设置店铺 %d 的暂停状态失败: %v", storeID, err)
		return fmt.Errorf("设置暂停状态失败: %w", err)
	}

	if success {
		a.logger.Infof("✓ 成功设置店铺 %d 的认证过期暂停键", storeID)
	} else {
		a.logger.Warnf("设置店铺 %d 的暂停状态返回失败", storeID)
		return fmt.Errorf("设置暂停状态返回失败")
	}

	return nil
}

// APIClientInterface 定义API客户端接口
type APIClientInterface interface {
	HasCookies() bool
	ReloadCookies() error
	GetStoreID() int64
	GetCookieCount() int
	GetConfig() *Config
	GetCookieManager() *CookieManager
	SendHTTPRequest(method, url string, headers map[string]string, body any, formFields map[string]string, fileFields map[string]any) (*req.Response, error)
}
