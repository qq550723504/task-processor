package impl

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/imroc/req/v3"
)

// ManagementAPIClientImpl 管理系统API客户端实现
type ManagementAPIClientImpl struct {
	baseURL    string
	httpClient *req.Client

	// 用户访问令牌（从登录会话获取）
	accessToken string
	tenantID    string
	tokenMutex  sync.RWMutex
}

// NewManagementAPIClientWithBaseURL 创建新的管理系统API客户端，可以指定baseURL
// 注意：请通过management.ClientManager来获取客户端实例，不要直接调用此函数
func NewManagementAPIClientWithBaseURL(baseURL string) *ManagementAPIClientImpl {
	return &ManagementAPIClientImpl{
		baseURL:    baseURL,
		httpClient: req.C(),
	}
}

// GetBaseURL 获取基础URL（用于测试）
func (m *ManagementAPIClientImpl) GetBaseURL() string {
	return m.baseURL
}

// SetUserToken 设置用户访问令牌和租户ID
func (m *ManagementAPIClientImpl) SetUserToken(accessToken, tenantID string) {
	m.tokenMutex.Lock()
	defer m.tokenMutex.Unlock()
	m.accessToken = accessToken
	m.tenantID = tenantID
}

// GetAccessToken 获取访问令牌（用于测试）
func (m *ManagementAPIClientImpl) GetAccessToken() (string, error) {
	return m.getAccessToken()
}

// getAccessToken 获取访问令牌
func (m *ManagementAPIClientImpl) getAccessToken() (string, error) {
	m.tokenMutex.RLock()
	defer m.tokenMutex.RUnlock()

	if m.accessToken == "" {
		return "", fmt.Errorf("未设置用户访问令牌，请先登录")
	}

	return m.accessToken, nil
}

// apiRequest 统一的API请求方法
func (m *ManagementAPIClientImpl) apiRequest(method, url string, requestBody interface{}, result interface{}) error {
	// 获取访问令牌
	token, err := m.getAccessToken()
	if err != nil {
		return fmt.Errorf("获取访问令牌失败: %w", err)
	}

	// 构建请求
	request := m.httpClient.R()

	// 设置租户ID
	m.tokenMutex.RLock()
	tenantID := m.tenantID
	m.tokenMutex.RUnlock()

	if tenantID == "" {
		tenantID = "1" // 默认租户ID
	}
	request.SetHeader("tenant-id", tenantID)

	// 添加访问令牌到请求头
	if token != "" {
		request.SetBearerAuthToken(token)
	}

	// 根据请求方法执行不同的请求
	var resp interface{}
	switch strings.ToUpper(method) {
	case http.MethodGet:
		// GET请求：将参数作为查询参数
		if requestBody != nil {
			if params, ok := requestBody.(map[string]interface{}); ok {
				request.SetQueryParams(convertToStringMap(params))
			}
		}
		resp, err = request.SetSuccessResult(result).Get(url)
	case http.MethodPost:
		resp, err = request.SetBody(requestBody).SetSuccessResult(result).Post(url)
	case http.MethodPut:
		resp, err = request.SetBody(requestBody).SetSuccessResult(result).Put(url)
	case http.MethodPatch:
		resp, err = request.SetBody(requestBody).SetSuccessResult(result).Patch(url)
	case http.MethodDelete:
		resp, err = request.SetBody(requestBody).SetSuccessResult(result).Delete(url)
	default:
		return fmt.Errorf("不支持的HTTP方法: %s", method)
	}

	// 处理请求错误
	if err != nil {
		return fmt.Errorf("API调用失败: %w", err)
	}

	// 类型断言获取响应对象
	response, ok := resp.(*req.Response)
	if !ok {
		return fmt.Errorf("无法获取响应对象")
	}

	// 检查响应状态码
	if !response.IsSuccessState() {
		// 尝试获取错误信息
		errorMessage := response.String()
		if errorMessage == "" {
			errorMessage = http.StatusText(response.StatusCode)
		}

		return &ManagementAPIError{
			Code:    response.StatusCode,
			Message: errorMessage,
			Details: fmt.Sprintf("请求URL: %s", url),
		}
	}

	return nil
}

// convertToStringMap 将 map[string]interface{} 转换为 map[string]string
func convertToStringMap(params map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for key, value := range params {
		if value != nil {
			result[key] = fmt.Sprintf("%v", value)
		}
	}
	return result
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

// NonRetryableError 不可重试错误类型
type NonRetryableError struct {
	Message string
	Err     error
}

func (e *NonRetryableError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable 实现RetryableError接口，返回false表示不可重试
func (e *NonRetryableError) IsRetryable() bool {
	return false
}

// NewNonRetryableError 创建不可重试错误
func NewNonRetryableError(message string, err error) *NonRetryableError {
	return &NonRetryableError{
		Message: message,
		Err:     err,
	}
}

// APIResponse 通用API响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ProcessAPIResponse 处理API响应的通用方法
func (m *ManagementAPIClientImpl) ProcessAPIResponse(resp *APIResponse, expectedCode int) error {
	if resp.Code != expectedCode {
		return &ManagementAPIError{
			Code:    resp.Code,
			Message: resp.Message,
		}
	}
	return nil
}

// TokenResponse 令牌响应结构
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
}
