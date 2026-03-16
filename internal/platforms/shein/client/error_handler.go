// Package client 提供SHEIN通用API错误处理功能
package client

import (
	"fmt"
	"task-processor/internal/platforms/shein/api"
)

// APIErrorHandler API错误处理器
type APIErrorHandler struct {
	baseClient *BaseAPIClient
}

// NewAPIErrorHandler 创建API错误处理器
func NewAPIErrorHandler(baseClient *BaseAPIClient) *APIErrorHandler {
	return &APIErrorHandler{
		baseClient: baseClient,
	}
}

// ProcessAPIResponse 处理API响应，统一错误处理
func (h *APIErrorHandler) ProcessAPIResponse(response *api.APIResponse, expectedCode, url, errorMessage string) error {
	// 使用 ProcessAPIResponse 检查认证过期
	if err := h.baseClient.ProcessAPIResponse(response, expectedCode); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return err
		}
		// 其他错误，包装为 APIError
		return &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("%s: %s", errorMessage, response.Msg),
			URL:        url,
		}
	}

	return nil
}

// HandleAuthenticationError 处理认证错误
func (h *APIErrorHandler) HandleAuthenticationError(code, message string) error {
	return &api.AuthenticationExpiredError{
		TenantID: h.baseClient.GetTenantID(),
		ShopID:   h.baseClient.GetShopID(),
		Code:     code,
		Message:  message,
	}
}

// HandleBusinessError 处理业务错误
func (h *APIErrorHandler) HandleBusinessError(statusCode int, message, url string) error {
	return &api.APIError{
		StatusCode: statusCode,
		Message:    message,
		URL:        url,
	}
}

// IsAuthenticationExpired 检查是否为认证过期错误
func (h *APIErrorHandler) IsAuthenticationExpired(code string) bool {
	return code == "20302"
}

// WrapError 包装错误信息
func (h *APIErrorHandler) WrapError(err error, operation, url string) error {
	if err == nil {
		return nil
	}

	// 如果已经是API错误，直接返回
	if apiErr, ok := err.(*api.APIError); ok {
		return apiErr
	}

	// 如果已经是认证过期错误，直接返回
	if authErr, ok := err.(*api.AuthenticationExpiredError); ok {
		return authErr
	}

	// 包装为通用API错误
	return &api.APIError{
		StatusCode: 0,
		Message:    fmt.Sprintf("%s失败: %v", operation, err),
		URL:        url,
	}
}

// LogError 记录错误日志
func (h *APIErrorHandler) LogError(err error, operation string) {
	if err == nil {
		return
	}

	// 这里可以添加日志记录逻辑
	// 例如使用 logrus 或其他日志库
	fmt.Printf("API错误 [%s]: %v\n", operation, err)
}
