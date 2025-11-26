package temu

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"task-processor/common/management"
	"time"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// APIClient TEMU API客户端 - 使用req库的增强版本
type APIClient struct {
	config        *Config
	client        *req.Client
	tenantID      int64
	storeID       int64
	cookies       []*http.Cookie
	cookieManager *CookieManager
	proxyURL      string // 代理地址
	logger        *logrus.Entry
}

// NewAPIClient 创建TEMU API客户端
func NewAPIClient(tenantID, storeID int64, managementClient *management.ClientManager) *APIClient {
	config := DefaultConfig()

	logger := logrus.WithFields(logrus.Fields{
		"component": "TEMUAPIClient",
		"tenantID":  tenantID,
		"storeID":   storeID,
	})

	apiClient := &APIClient{
		config:        config,
		tenantID:      tenantID,
		storeID:       storeID,
		cookieManager: NewCookieManager(storeID, managementClient),
		logger:        logger,
	}

	// 获取店铺配置信息（包括代理设置）
	if managementClient != nil {
		storeClient := managementClient.GetStoreClient()
		if storeClient != nil {
			if storeInfo, err := storeClient.GetStore(storeID); err != nil {
				apiClient.logger.WithError(err).Warn("获取店铺配置失败，将不使用代理")
			} else if storeInfo != nil && storeInfo.Proxy != "" {
				apiClient.proxyURL = storeInfo.Proxy
				apiClient.logger.Infof("店铺 %d 配置了代理地址: %s", storeID, storeInfo.Proxy)
			}
		}
	}

	// 初始化req客户端
	apiClient.initHTTPClient()

	// 在初始化时测试管理系统连接
	if err := apiClient.cookieManager.TestConnection(); err != nil {
		apiClient.logger.WithError(err).Error("管理系统连接测试失败，跳过Cookie加载")
	} else {
		// 连接正常，尝试加载Cookie
		if cookies, err := apiClient.cookieManager.LoadCookies(); err != nil {
			apiClient.logger.WithError(err).Error("初始化时加载Cookie失败")
		} else if cookies != nil {
			apiClient.SetCookies(cookies)
			apiClient.logger.Info("成功在初始化时加载Cookie")
		} else {
			apiClient.logger.Info("初始化时未找到Cookie数据")
		}
	}

	return apiClient
}

// initHTTPClient 初始化HTTP客户端 - 参考TEMU项目实现
func (c *APIClient) initHTTPClient() {
	client := req.C().
		SetTLSFingerprintChrome().
		EnableAutoDecompress().
		SetTLSClientConfig(c.getTLSConfig()).
		SetCommonHeaders(c.getDefaultHeaders()).
		SetCommonRetryCount(3).
		SetCommonRetryInterval(func(resp *req.Response, attempt int) time.Duration {
			// 动态退避策略
			baseDelay := time.Duration(attempt*attempt) * time.Second
			return baseDelay
		}).
		SetCommonRetryCondition(func(resp *req.Response, err error) bool {
			// 网络错误重试
			if err != nil {
				return true
			}
			// HTTP错误重试
			if resp != nil && (resp.StatusCode >= 500 || resp.StatusCode == 429) {
				return true
			}
			return false
		}).
		SetTimeout(c.config.RequestTimeout)

	// 如果配置了代理，则设置代理
	if c.proxyURL != "" {
		c.logger.Infof("使用代理地址: %s", c.proxyURL)
		client = client.SetProxyURL(c.proxyURL)
	}

	c.client = client
}

// getTLSConfig 获取TLS配置 - 参考TEMU项目
func (c *APIClient) getTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}
}

// getDefaultHeaders 获取默认请求头 - 参考TEMU项目
func (c *APIClient) getDefaultHeaders() map[string]string {
	return map[string]string{
		"Accept":                    "application/json, text/plain, */*",
		"Accept-Encoding":           "gzip, deflate, br",
		"Accept-Language":           "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
		"Cache-Control":             "no-cache",
		"Pragma":                    "no-cache",
		"Priority":                  "u=1, i",
		"Sec-Ch-Ua":                 `"Not A;Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		"Sec-Ch-Ua-Mobile":          "?0",
		"Sec-Ch-Ua-Platform":        `"Windows"`,
		"Sec-Fetch-Dest":            "empty",
		"Sec-Fetch-Mode":            "cors",
		"Sec-Fetch-Site":            "same-origin",
		"Upgrade-Insecure-Requests": "1",
	}
}

// SetCookies 设置Cookie
func (c *APIClient) SetCookies(cookies []*http.Cookie) {
	c.cookies = cookies
	// req包使用SetCommonCookies来设置全局Cookie
	c.client.SetCommonCookies(cookies...)
	c.logger.WithField("cookieNum", len(cookies)).Info("设置Cookie")
}

// ReloadCookies 重新加载Cookie
func (c *APIClient) ReloadCookies() error {
	cookies, err := c.cookieManager.LoadCookies()
	if err != nil {
		c.logger.WithError(err).Error("重新加载Cookie失败")
		return fmt.Errorf("重新加载Cookie失败: %w", err)
	}

	if cookies != nil {
		c.SetCookies(cookies)
		c.logger.Info("成功重新加载Cookie")
	} else {
		c.logger.Info("未找到Cookie数据")
	}

	return nil
}

// HasCookies 检查是否有Cookie
func (c *APIClient) HasCookies() bool {
	return len(c.cookies) > 0
}

// GetCookieCount 获取Cookie数量
func (c *APIClient) GetCookieCount() int {
	return len(c.cookies)
}

// SendTEMURequest 发送TEMU API请求（带Cookie检查和重试逻辑）
func (c *APIClient) SendTEMURequest(request map[string]interface{}, result interface{}) error {
	// 检查Cookie
	if !c.HasCookies() {
		c.logger.Warnf("店铺ID=%d没有Cookie数据，尝试重新加载Cookie", c.GetStoreID())

		// 尝试重新加载Cookie
		if err := c.ReloadCookies(); err != nil {
			// Cookie加载失败，设置暂停键
			c.logger.Errorf("重新加载Cookie失败: %v", err)
			if pauseErr := c.setPauseKeyForAuthExpired("从管理系统获取Cookie失败: Cookie数据为空"); pauseErr != nil {
				c.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}
			// 返回AuthExpiredError以便任务处理器识别并暂停任务
			return NewAuthExpiredError(
				fmt.Sprintf("店铺ID=%d没有Cookie数据且重新加载失败，请先在管理系统中设置Cookie", c.GetStoreID()),
				err,
			)
		}

		// 再次检查Cookie
		if !c.HasCookies() {
			// Cookie为空，设置暂停键
			c.logger.Warn("Cookie数据为空")
			if pauseErr := c.setPauseKeyForAuthExpired("Cookie数据为空"); pauseErr != nil {
				c.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}
			// 返回AuthExpiredError以便任务处理器识别并暂停任务
			return NewAuthExpiredError(
				fmt.Sprintf("店铺ID=%d没有Cookie数据，请先在管理系统中设置Cookie", c.GetStoreID()),
				nil,
			)
		}

		c.logger.Infof("成功重新加载Cookie，数量: %d", c.GetCookieCount())
	} else {
		c.logger.Debugf("Cookie检查通过，数量: %d", c.GetCookieCount())
	}

	// 使用重试逻辑发送请求
	return c.sendTEMURequestWithRetry(request, result)
}

// sendTEMURequestWithRetry 发送TEMU API请求（带重试逻辑）
func (c *APIClient) sendTEMURequestWithRetry(request map[string]interface{}, result interface{}) error {
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		c.logger.Debugf("API调用尝试 %d/%d", attempt, maxRetries)

		err := c.sendTEMURequestOnce(request, result)
		if err == nil {
			c.logger.Debugf("API调用成功，尝试次数: %d", attempt)
			return nil
		}

		c.logger.Warnf("API调用失败 (尝试 %d/%d): %v", attempt, maxRetries, err)

		// 如果是认证相关错误，尝试重新加载Cookie
		if c.isAuthenticationError(err) {
			c.logger.Infof("检测到认证错误，尝试重新加载Cookie...")
			if reloadErr := c.ReloadCookies(); reloadErr != nil {
				c.logger.Warnf("重新加载Cookie失败: %v", reloadErr)
				// 如果是最后一次尝试且Cookie加载失败，设置暂停键
				if attempt == maxRetries {
					c.logger.Error("所有重试均失败，设置认证过期暂停键")
					if pauseErr := c.setPauseKeyForAuthExpired(fmt.Sprintf("认证错误且Cookie重新加载失败: %v", reloadErr)); pauseErr != nil {
						c.logger.Errorf("设置暂停键失败: %v", pauseErr)
					}
				}
			} else {
				c.logger.Infof("成功重新加载Cookie，数量: %d", c.GetCookieCount())
			}
		}

		// 如果不是最后一次尝试，记录重试信息
		if attempt < maxRetries {
			c.logger.Debugf("准备重试...")
		}
	}

	return fmt.Errorf("API调用失败，已重试%d次", maxRetries)
}

// sendTEMURequestOnce 发送单次TEMU API请求 - 使用req库
func (c *APIClient) sendTEMURequestOnce(request map[string]interface{}, result interface{}) error {
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
	fileFields, _ := request["fileFields"].(map[string]interface{})

	// 验证请求参数
	if err := c.validateRequest(method, url); err != nil {
		return fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 构造完整URL
	fullURL := c.config.BaseURL + url

	// 发送HTTP请求
	response, err := c.sendHTTPRequest(method, fullURL, headers, body, formFields, fileFields)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}

	// 检查HTTP状态码
	if !c.isSuccess(response) {
		// 尝试读取错误响应体
		if errorBody, err := response.ToBytes(); err == nil {
			c.logger.Errorf("HTTP请求失败，状态码: %d, 响应体: %s", response.StatusCode, string(errorBody))
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

// sendHTTPRequest 发送HTTP请求的内部方法 - 使用req库
func (c *APIClient) sendHTTPRequest(method, url string, headers map[string]string, body interface{}, formFields map[string]string, fileFields map[string]interface{}) (*req.Response, error) {
	// 创建动态请求
	r := c.client.R()

	// 设置请求头
	for key, value := range headers {
		r.SetHeader(key, value)
	}

	// 如果有文件字段，则使用multipart/form-data格式
	if len(fileFields) > 0 {
		// 添加表单字段
		if len(formFields) > 0 {
			r.SetFormData(formFields)
		}

		// 添加文件字段
		for fieldName, fileData := range fileFields {
			if fileInfo, ok := fileData.(map[string]interface{}); ok {
				filename, _ := fileInfo["filename"].(string)
				content, _ := fileInfo["content"].([]byte)
				r.SetFileBytes(fieldName, filename, content)
			}
		}

		// 根据方法类型发送请求
		switch method {
		case "GET":
			return r.Get(url)
		case "POST":
			return r.Post(url)
		case "PUT":
			return r.Put(url)
		case "DELETE":
			return r.Delete(url)
		default:
			return r.Send(method, url)
		}
	} else {
		// 否则使用JSON格式
		// 如果没有设置Content-Type，则默认为JSON
		if _, exists := headers["content-type"]; !exists && body != nil {
			r.SetHeader("content-type", "application/json;charset=UTF-8")
		}

		// 根据方法类型发送请求
		switch method {
		case "GET":
			return r.Get(url)
		case "POST":
			return r.SetBody(body).Post(url)
		case "PUT":
			return r.SetBody(body).Put(url)
		case "DELETE":
			return r.SetBody(body).Delete(url)
		default:
			return r.SetBody(body).Send(method, url)
		}
	}
}

// validateRequest 验证请求参数
func (c *APIClient) validateRequest(method, url string) error {
	if method == "" {
		return fmt.Errorf("请求方法不能为空")
	}

	if url == "" {
		return fmt.Errorf("请求URL不能为空")
	}

	return nil
}

// isSuccess 检查响应是否成功
func (c *APIClient) isSuccess(response *req.Response) bool {
	if response == nil {
		return false
	}
	return response.StatusCode >= 200 && response.StatusCode < 300
}

// GetTenantID 获取租户ID
func (c *APIClient) GetTenantID() int64 {
	return c.tenantID
}

// GetStoreID 获取店铺ID
func (c *APIClient) GetStoreID() int64 {
	return c.storeID
}

// GetBaseURL 获取基础URL
func (c *APIClient) GetBaseURL() string {
	return c.config.BaseURL
}

// isAuthenticationError 判断是否为认证相关错误
func (c *APIClient) isAuthenticationError(err error) bool {
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
			c.logger.Debugf("检测到认证错误关键词: %s", keyword)
			return true
		}
	}

	return false
}

// setPauseKeyForAuthExpired 设置认证过期暂停键
// 暂停键格式: listing:task:pause:temu:{tenant_id}:{shop_id}
// 暂停键值格式: {"type":"auth_expired","reason":"原因","timestamp":1234567890}
func (c *APIClient) setPauseKeyForAuthExpired(reason string) error {
	if c.cookieManager == nil || c.cookieManager.managementClient == nil {
		c.logger.Warn("管理客户端未初始化，无法设置暂停键")
		return fmt.Errorf("管理客户端未初始化")
	}

	// 调用管理系统API设置暂停状态
	// pauseType: "auth_expired" 表示认证过期类型
	storeClient := c.cookieManager.managementClient.GetStoreClient()
	if storeClient == nil {
		c.logger.Warn("店铺客户端未初始化，无法设置暂停键")
		return fmt.Errorf("店铺客户端未初始化")
	}

	c.logger.Infof("设置店铺 %d 的认证过期暂停键，原因: %s", c.storeID, reason)
	success, err := storeClient.SetStorePauseStatus(c.storeID, true, "auth_expired")
	if err != nil {
		c.logger.Errorf("设置店铺 %d 的暂停状态失败: %v", c.storeID, err)
		return fmt.Errorf("设置暂停状态失败: %w", err)
	}

	if success {
		c.logger.Infof("✓ 成功设置店铺 %d 的认证过期暂停键", c.storeID)
	} else {
		c.logger.Warnf("设置店铺 %d 的暂停状态返回失败", c.storeID)
		return fmt.Errorf("设置暂停状态返回失败")
	}

	return nil
}
