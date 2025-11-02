package impl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// APIResponse 通用API响应结构
type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ManagementAPIError 管理系统API错误
type ManagementAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *ManagementAPIError) Error() string {
	return fmt.Sprintf("Management API error %d: %s", e.Code, e.Message)
}

// BaseManagementClient 基础管理系统客户端
type BaseManagementClient struct {
	baseURL     string
	accessToken string
	tenantID    string
	mutex       sync.RWMutex
	httpClient  *http.Client
}

// NewBaseManagementClient 创建基础管理系统客户端
func NewBaseManagementClient(baseURL string) *BaseManagementClient {
	if baseURL == "" {
		baseURL = "http://getway.linkcloudai.com"
	}

	return &BaseManagementClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetUserToken 设置用户访问令牌
func (c *BaseManagementClient) SetUserToken(accessToken, tenantID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.accessToken = accessToken
	c.tenantID = tenantID
}

// GetUserToken 获取用户访问令牌
func (c *BaseManagementClient) GetUserToken() (string, string) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.accessToken, c.tenantID
}

// HasValidToken 检查是否有有效的令牌
func (c *BaseManagementClient) HasValidToken() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.accessToken != "" && c.tenantID != ""
}

// GetToken 获取当前访问令牌
func (c *BaseManagementClient) GetToken() (string, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.accessToken == "" {
		return "", fmt.Errorf("访问令牌为空")
	}

	return c.accessToken, nil
}

// IsAuthenticated 检查是否已认证
func (c *BaseManagementClient) IsAuthenticated() bool {
	return c.HasValidToken()
}

// makeAPIRequest 通用API请求方法
func (c *BaseManagementClient) makeAPIRequest(method, endpoint string, params map[string]any) ([]byte, error) {
	c.mutex.RLock()
	accessToken := c.accessToken
	tenantID := c.tenantID
	c.mutex.RUnlock()

	if accessToken == "" {
		return nil, fmt.Errorf("访问令牌为空")
	}

	// 构建完整URL
	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	var req *http.Request
	var err error

	if method == "GET" {
		req, err = http.NewRequest("GET", url, nil)
	} else {
		jsonData, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			return nil, fmt.Errorf("序列化请求参数失败: %w", marshalErr)
		}
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	}

	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("tenant-id", tenantID)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败: 状态码=%d, 响应=%s", resp.StatusCode, string(body))
	}

	return body, nil
}

// makeAPIRequestWithURL 使用完整URL的API请求方法
func (c *BaseManagementClient) makeAPIRequestWithURL(method, urlPath string) ([]byte, error) {
	c.mutex.RLock()
	accessToken := c.accessToken
	tenantID := c.tenantID
	c.mutex.RUnlock()

	if accessToken == "" {
		return nil, fmt.Errorf("访问令牌为空")
	}

	// 构建完整URL
	url := fmt.Sprintf("%s%s", c.baseURL, urlPath)

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("tenant-id", tenantID)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败: 状态码=%d, 响应=%s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ProcessAPIResponse 处理API响应的通用方法
func (c *BaseManagementClient) ProcessAPIResponse(resp *APIResponse, expectedCode int) error {
	if resp.Code != expectedCode {
		return &ManagementAPIError{
			Code:    resp.Code,
			Message: resp.Message,
		}
	}
	return nil
}
