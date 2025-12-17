package impl

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"task-processor/internal/common/shein/api"

	"github.com/imroc/req/v3"
)

// BaseAPIClient 基础API客户端实现
type BaseAPIClient struct {
	baseUrl    string
	tenantID   int64
	shopID     int64
	shopType   string // 添加shopType字段
	httpClient *req.Client
}

// NewBaseAPIClient 创建新的基础API客户端
func NewBaseAPIClient(baseUrl string, tenantID, shopID int64, httpClient *req.Client) *BaseAPIClient {
	return &BaseAPIClient{
		baseUrl:    baseUrl,
		tenantID:   tenantID,
		shopID:     shopID,
		httpClient: httpClient,
	}
}

// createHeaders 创建基础请求头
func (b *BaseAPIClient) createHeaders(requestURL string) map[string]string {
	// 解析URL获取基础域名和路径信息
	baseURL := b.extractBaseURL(requestURL)
	apiPath := b.extractAPIPath(requestURL)

	// 基础请求头配置
	headers := map[string]string{
		"priority":           "u=1, i",
		"referer":            baseURL + "/",
		"sec-ch-ua":          `"Google Chrome";v="138", "Chromium";v="138", "Not/A)Brand";v="24"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"user-agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
	}

	// 添加标准请求头
	headers["front-version"] = "20250207"
	headers["x-lt-language"] = "CN"
	headers["x-req-zone-id"] = "America/Los_Angeles"

	// 根据API路径设置特定请求头（仅保留必要的）
	switch {
	case b.contains(apiPath, "sso-prefix/auth/getUser"):
		headers["method"] = "GET"
		headers["scheme"] = "https"
		headers["x-sso-scene"] = "sw"
		headers["gmpsso-language"] = "CN"

	case b.contains(apiPath, "/sso/public/account/supplier/getSupplierOperateInfo"):
		headers["method"] = "POST"
		headers["path"] = "/sso/public/account/supplier/getSupplierOperateInfo"
		headers["scheme"] = "https"
		// 根据shopType设置特定头
		if b.shopType != "self_operated" { // 假设自营店铺的类型标识
			headers["gmpsso-language"] = "CN"
			headers["sso-frontend-version"] = "1.0.0"
			headers["time-zone"] = "America/Los_Angeles"
			headers["x-sso-scene"] = "sw"
		}

	default:
		headers["method"] = "POST"
		headers["scheme"] = "https"
	}

	return headers
}

// extractBaseURL 从完整URL中提取基础URL
func (b *BaseAPIClient) extractBaseURL(fullURL string) string {
	// 如果 URL 不包含协议，添加默认协议以便正确解析
	urlToParse := fullURL
	if !strings.HasPrefix(fullURL, "http://") && !strings.HasPrefix(fullURL, "https://") {
		urlToParse = "https://" + fullURL
	}

	parsedURL, err := url.Parse(urlToParse)
	if err != nil {
		return fullURL
	}

	// 如果 URL 缺少协议，默认使用 https
	scheme := parsedURL.Scheme
	if scheme == "" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, parsedURL.Host)
}

// extractAPIPath 从完整URL中提取API路径
func (b *BaseAPIClient) extractAPIPath(fullURL string) string {
	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		return fullURL
	}
	return parsedURL.Path
}

// contains 检查字符串是否包含子字符串
func (b *BaseAPIClient) contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// apiRequest 统一的API请求方法
func (b *BaseAPIClient) apiRequest(method, url string, requestBody interface{}, result interface{}) error {
	// 创建请求头
	headers := b.createHeaders(url)

	// 创建请求
	request := b.httpClient.R()

	// 设置请求头
	for key, value := range headers {
		request.SetHeader(key, value)
	}

	var resp *req.Response
	var err error

	// 根据请求方法执行不同的请求
	switch strings.ToUpper(method) {
	case http.MethodGet:
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

	// 检查响应状态码
	if !resp.IsSuccessState() {
		// 尝试获取错误信息
		errorMessage := resp.String()
		if errorMessage == "" {
			errorMessage = http.StatusText(resp.StatusCode)
		}

		return &api.APIError{
			StatusCode: resp.StatusCode,
			Message:    errorMessage,
			URL:        url,
		}
	}

	return nil
}

// ProcessAPIResponse 处理API响应的通用方法（保留向后兼容性）
func (b *BaseAPIClient) ProcessAPIResponse(resp *api.APIResponse, expectedCode string) error {
	// 检查是否为认证过期错误（子系统登录重定向）
	if resp.Code == "20302" && strings.Contains(resp.Msg, "子系统登录重定向") {
		return &api.AuthenticationExpiredError{
			TenantID: b.tenantID,
			ShopID:   b.shopID,
			Code:     resp.Code,
			Message:  fmt.Sprintf("认证已过期，需要重新登录: %s", resp.Msg),
		}
	}

	if resp.Code != expectedCode {
		return fmt.Errorf("%s: %s", resp.Code, resp.Msg)
	}
	return nil
}

// ProcessError 处理通用错误，检查是否为认证相关错误
func (b *BaseAPIClient) ProcessError(err error) error {
	if err == nil {
		return nil
	}

	// 检查是否为认证过期错误（包括Redis Cookie错误）
	if authErr, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
		// 确保错误包含正确的租户和店铺信息
		if authErr.TenantID == 0 {
			authErr.TenantID = b.tenantID
		}
		if authErr.ShopID == 0 {
			authErr.ShopID = b.shopID
		}
		return authErr
	}

	return err
}

// GetTenantID 获取租户ID
func (b *BaseAPIClient) GetTenantID() int64 {
	return b.tenantID
}

// GetShopID 获取店铺ID
func (b *BaseAPIClient) GetShopID() int64 {
	return b.shopID
}

// GetShopType 获取店铺类型
func (b *BaseAPIClient) GetShopType() string {
	return b.shopType
}

func (b *BaseAPIClient) GetBaseURL() string {
	// 确保 baseUrl 包含协议前缀
	if b.baseUrl == "" {
		return ""
	}

	// 如果已经包含协议，直接返回
	if strings.HasPrefix(b.baseUrl, "http://") || strings.HasPrefix(b.baseUrl, "https://") {
		return b.baseUrl
	}

	// 如果缺少协议，添加 https://
	// 处理可能以 / 开头的情况
	cleanURL := strings.TrimPrefix(b.baseUrl, "/")
	return "https://" + cleanURL
}
