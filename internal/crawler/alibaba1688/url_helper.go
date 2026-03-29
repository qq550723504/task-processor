// Package alibaba1688 提供1688 URL处理工具
package alibaba1688

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"task-processor/internal/core/logger"
)

// URLHelper 1688 URL处理辅助工具
type URLHelper struct {
	// 1688商品URL的正则表达式
	productURLRegex *regexp.Regexp
	offerURLRegex   *regexp.Regexp
}

// NewURLHelper 创建新的URL辅助工具
func NewURLHelper() *URLHelper {
	return &URLHelper{
		// 匹配1688商品详情页URL
		productURLRegex: regexp.MustCompile(`^https?://detail\.1688\.com/offer/(\d+)\.html`),
		// 匹配1688 offer URL
		offerURLRegex: regexp.MustCompile(`^https?://.*1688\.com.*offer[/=](\d+)`),
	}
}

// IsValid1688URL 检查是否为有效的1688商品URL
func (h *URLHelper) IsValid1688URL(rawURL string) bool {
	if rawURL == "" {
		return false
	}

	// 清理URL
	cleanURL := h.CleanURL(rawURL)

	// 检查是否匹配1688商品URL模式
	return h.productURLRegex.MatchString(cleanURL) || h.offerURLRegex.MatchString(cleanURL)
}

// ExtractProductID 从URL中提取商品ID
func (h *URLHelper) ExtractProductID(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("URL不能为空")
	}

	cleanURL := h.CleanURL(rawURL)

	// 尝试从标准商品详情页URL提取
	if matches := h.productURLRegex.FindStringSubmatch(cleanURL); len(matches) > 1 {
		return matches[1], nil
	}

	// 尝试从offer URL提取
	if matches := h.offerURLRegex.FindStringSubmatch(cleanURL); len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("无法从URL中提取商品ID: %s", rawURL)
}

// CleanURL 清理和标准化URL
func (h *URLHelper) CleanURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	// 移除前后空格
	cleanURL := strings.TrimSpace(rawURL)

	// 确保有协议前缀
	if !strings.HasPrefix(cleanURL, "http://") && !strings.HasPrefix(cleanURL, "https://") {
		cleanURL = "https://" + cleanURL
	}

	// 解析URL以进行进一步清理
	parsedURL, err := url.Parse(cleanURL)
	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("解析URL失败: %v, 返回原始URL", err)
		return cleanURL
	}

	// 移除不必要的查询参数，保留重要的参数
	query := parsedURL.Query()
	newQuery := url.Values{}

	// 保留重要的查询参数
	importantParams := []string{"offerId", "memberId", "spm"}
	for _, param := range importantParams {
		if value := query.Get(param); value != "" {
			newQuery.Set(param, value)
		}
	}

	parsedURL.RawQuery = newQuery.Encode()
	parsedURL.Fragment = "" // 移除锚点

	return parsedURL.String()
}

// BuildStandardURL 构建标准的1688商品URL
func (h *URLHelper) BuildStandardURL(productID string) string {
	if productID == "" {
		return ""
	}

	return fmt.Sprintf("https://detail.1688.com/offer/%s.html", productID)
}

// GetMobileURL 获取移动版URL
func (h *URLHelper) GetMobileURL(productID string) string {
	if productID == "" {
		return ""
	}

	return fmt.Sprintf("https://m.1688.com/offer/%s.html", productID)
}

// ValidateAndNormalizeURL 验证并标准化URL
func (h *URLHelper) ValidateAndNormalizeURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("URL不能为空")
	}

	// 清理URL
	cleanURL := h.CleanURL(rawURL)

	// 验证是否为有效的1688 URL
	if !h.IsValid1688URL(cleanURL) {
		return "", fmt.Errorf("不是有效的1688商品URL: %s", rawURL)
	}

	// 提取商品ID
	productID, err := h.ExtractProductID(cleanURL)
	if err != nil {
		return "", fmt.Errorf("提取商品ID失败: %w", err)
	}

	// 返回标准化的URL
	return h.BuildStandardURL(productID), nil
}

// GetDomain 获取域名
func (h *URLHelper) GetDomain() string {
	return "1688.com"
}

// GetPlatformName 获取平台名称
func (h *URLHelper) GetPlatformName() string {
	return "阿里巴巴1688"
}

// IsProductPage 检查是否为商品详情页
func (h *URLHelper) IsProductPage(rawURL string) bool {
	return h.IsValid1688URL(rawURL)
}

// GetSupplierURL 根据供应商ID构建供应商页面URL
func (h *URLHelper) GetSupplierURL(supplierID string) string {
	if supplierID == "" {
		return ""
	}

	return fmt.Sprintf("https://shop%s.1688.com", supplierID)
}
