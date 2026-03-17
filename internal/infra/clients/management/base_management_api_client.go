package management

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/imroc/req/v3"
)

// ManagementAPIClient 管理系统API客户端实现
type ManagementAPIClient struct {
	baseURL    string
	httpClient *req.Client

	accessToken string
	tenantID    string
	tokenMutex  sync.RWMutex
}

// NewManagementAPIClientWithBaseURL 创建新的管理系统API客户端
func NewManagementAPIClientWithBaseURL(baseURL string) *ManagementAPIClient {
	return &ManagementAPIClient{
		baseURL:    baseURL,
		httpClient: req.C(),
	}
}

// GetBaseURL 获取基础URL
func (m *ManagementAPIClient) GetBaseURL() string {
	return m.baseURL
}

// SetUserToken 设置用户访问令牌和租户ID
func (m *ManagementAPIClient) SetUserToken(accessToken, tenantID string) {
	m.tokenMutex.Lock()
	defer m.tokenMutex.Unlock()
	m.accessToken = accessToken
	m.tenantID = tenantID
}

// GetAccessToken 获取访问令牌
func (m *ManagementAPIClient) GetAccessToken() (string, error) {
	return m.getAccessToken()
}

func (m *ManagementAPIClient) getAccessToken() (string, error) {
	m.tokenMutex.RLock()
	defer m.tokenMutex.RUnlock()

	if m.accessToken == "" {
		return "", fmt.Errorf("未设置用户访问令牌，请先登录")
	}

	return m.accessToken, nil
}

// apiRequest 统一的API请求方法
func (m *ManagementAPIClient) apiRequest(method, url string, requestBody any, result any) error {
	token, err := m.getAccessToken()
	if err != nil {
		return fmt.Errorf("获取访问令牌失败: %w", err)
	}

	request := m.httpClient.R()

	m.tokenMutex.RLock()
	tenantID := m.tenantID
	m.tokenMutex.RUnlock()

	if tenantID == "" {
		tenantID = "1"
	}
	request.SetHeader("tenant-id", tenantID)

	if token != "" {
		request.SetBearerAuthToken(token)
	}

	var resp any
	switch strings.ToUpper(method) {
	case http.MethodGet:
		if requestBody != nil {
			if params, ok := requestBody.(map[string]any); ok {
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

	if err != nil {
		return fmt.Errorf("API调用失败: %w", err)
	}

	response, ok := resp.(*req.Response)
	if !ok {
		return fmt.Errorf("无法获取响应对象")
	}

	if !response.IsSuccessState() {
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

func convertToStringMap(params map[string]any) map[string]string {
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

func (e *NonRetryableError) IsRetryable() bool {
	return false
}

// NewNonRetryableError 创建不可重试错误
func NewNonRetryableError(message string, err error) *NonRetryableError {
	return &NonRetryableError{Message: message, Err: err}
}

// APIResponse 通用API响应结构
type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ProcessAPIResponse 处理API响应
func (m *ManagementAPIClient) ProcessAPIResponse(resp *APIResponse, expectedCode int) error {
	if resp.Code != expectedCode {
		return &ManagementAPIError{
			Code:    resp.Code,
			Message: resp.Message,
		}
	}
	return nil
}

// getTypedResult 发起请求并将响应 Data 断言为 *T，返回解引用后的值。
// 适用于返回单个对象或标量（bool、int64 等）的接口。
func getTypedResult[T any](m *ManagementAPIClient, method, url string, body any) (T, error) {
	var zero T
	var result APIResponse
	result.Data = new(T)

	if err := m.apiRequest(method, url, body, &result); err != nil {
		return zero, err
	}
	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return zero, err
	}
	if result.Data == nil {
		return zero, fmt.Errorf("响应数据为空")
	}
	v, ok := result.Data.(*T)
	if !ok {
		return zero, fmt.Errorf("响应数据类型转换失败")
	}
	return *v, nil
}

// getSliceResult 发起 GET 请求并将响应 Data 断言为 *[]T，返回切片。
// 适用于返回列表的接口。
func getSliceResult[T any](m *ManagementAPIClient, url string, params map[string]any) ([]T, error) {
	var result APIResponse
	result.Data = &[]T{}

	if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
		return nil, err
	}
	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}
	v, ok := result.Data.(*[]T)
	if !ok {
		return nil, fmt.Errorf("响应数据类型转换失败")
	}
	return *v, nil
}

// groupByPlatform 按平台字段将切片分组，platform 字段为空时归入 "UNKNOWN"。
// getPlatform 是从元素中提取平台名的函数，适用于所有需要按平台分批提交的 API 客户端。
func groupByPlatform[T any](items []T, getPlatform func(T) string) map[string][]T {
	groups := make(map[string][]T)
	for _, item := range items {
		p := getPlatform(item)
		if p == "" {
			p = "UNKNOWN"
		}
		groups[p] = append(groups[p], item)
	}
	return groups
}

// TokenResponse 令牌响应结构
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
}
