package temu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"task-processor/common/management"

	"github.com/sirupsen/logrus"
)

// APIClient TEMU API客户端
type APIClient struct {
	config        *Config
	client        *http.Client
	tenantID      int64
	storeID       int64
	cookies       []*http.Cookie
	cookieManager *CookieManager
	logger        *logrus.Entry
}

// NewAPIClient 创建TEMU API客户端
func NewAPIClient(tenantID, storeID int64, managementClient *management.Client) *APIClient {
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
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}

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

// SetCookies 设置Cookie
func (c *APIClient) SetCookies(cookies []*http.Cookie) {
	c.cookies = cookies
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

// SendTEMURequest 发送TEMU API请求
func (c *APIClient) SendTEMURequest(request map[string]interface{}, result interface{}) error {
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

	// 验证请求参数
	if err := c.validateRequest(method, url); err != nil {
		return fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 构造完整URL
	fullURL := c.config.BaseURL + url

	// 发送HTTP请求
	response, err := c.sendHTTPRequest(method, fullURL, headers, body)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer response.Body.Close()

	// 检查HTTP状态码
	if !c.isSuccess(response) {
		return fmt.Errorf("HTTP请求失败，状态码: %d", response.StatusCode)
	}

	// 解析响应体
	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("读取响应体失败: %w", err)
	}

	return json.Unmarshal(respBody, result)
}

// sendHTTPRequest 发送HTTP请求的内部方法
func (c *APIClient) sendHTTPRequest(method, url string, headers map[string]string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader

	// 处理请求体
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	// 创建请求
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置默认请求头
	defaultHeaders := GetDefaultHeaders()
	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}

	// 设置自定义请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 如果没有设置Content-Type，则默认为JSON
	if req.Header.Get("content-type") == "" && body != nil {
		req.Header.Set("content-type", "application/json;charset=UTF-8")
	}

	// 添加Cookie
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	// 发送请求
	return c.client.Do(req)
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
func (c *APIClient) isSuccess(response *http.Response) bool {
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
