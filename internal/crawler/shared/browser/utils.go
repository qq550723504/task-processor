package browser

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// IPLocationInfo IP地理位置信息
type IPLocationInfo struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	Timezone    string  `json:"timezone"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

// GetUserAgentPool 获取用户代理池
func GetUserAgentPool() []string {
	return []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/131.0.0.0 Safari/537.36",
	}
}

// GenerateUserAgent 动态生成用户代理，避免版本不匹配检测
func GenerateUserAgent(customUserAgent string, userAgents []string) string {
	// 如果指定了自定义用户代理，使用指定的
	if customUserAgent != "" {
		return customUserAgent
	}

	// 从用户代理池中随机选择
	if len(userAgents) > 0 {
		return userAgents[rand.Intn(len(userAgents))]
	}

	// 默认用户代理
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
}

// GetLocationInfoByIP 根据IP获取地理位置信息（与Python版本一致）
func GetLocationInfoByIP(ip string) (*IPLocationInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s", ip))
	if err != nil {
		return nil, fmt.Errorf("请求IP地理位置失败: %w", err)
	}
	defer resp.Body.Close()

	var info IPLocationInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("解析IP地理位置响应失败: %w", err)
	}

	if info.Status != "success" {
		return nil, fmt.Errorf("IP地理位置查询失败: %s", info.Status)
	}

	return &info, nil
}

// GetTimezoneForRegion 根据代理IP地理位置返回对应时区（与Python版本一致）
func GetTimezoneForRegion(proxyServer string) *string {
	// 从代理服务器地址中提取IP
	ip := extractIPFromProxy(proxyServer)
	if ip == "" {
		timezone := "America/Los_Angeles"
		return &timezone
	}

	// 调用ip-api获取时区
	info, err := GetLocationInfoByIP(ip)
	if err != nil {
		logrus.Warnf("获取IP地理位置失败: %v，使用默认时区", err)
		timezone := "America/Los_Angeles"
		return &timezone
	}

	logrus.Infof("IP %s 对应时区: %s, 国家: %s", ip, info.Timezone, info.Country)
	return &info.Timezone
}

// GetLocaleForRegion 根据代理IP地理位置返回对应语言环境（与Python版本一致）
func GetLocaleForRegion(proxyServer string) string {
	// 从代理服务器地址中提取IP
	ip := extractIPFromProxy(proxyServer)
	if ip == "" {
		return "en-US"
	}

	// 调用ip-api获取国家代码
	info, err := GetLocationInfoByIP(ip)
	if err != nil {
		return "en-US"
	}

	// 语言环境映射（与Python版本一致）
	localeMap := map[string]string{
		"US": "en-US",
		"GB": "en-GB",
		"DE": "de-DE",
		"FR": "fr-FR",
		"IT": "it-IT",
		"ES": "es-ES",
		"JP": "ja-JP",
		"CN": "zh-CN",
		"AU": "en-AU",
		"CA": "en-CA",
	}

	if locale, ok := localeMap[info.CountryCode]; ok {
		return locale
	}
	return "en-US"
}

// extractIPFromProxy 从代理服务器地址中提取IP
func extractIPFromProxy(proxyServer string) string {
	if proxyServer == "" {
		return ""
	}

	// 移除协议前缀 http:// 或 https://
	proxy := proxyServer
	if strings.HasPrefix(proxy, "http://") {
		proxy = strings.TrimPrefix(proxy, "http://")
	} else if strings.HasPrefix(proxy, "https://") {
		proxy = strings.TrimPrefix(proxy, "https://")
	}

	// 移除端口号
	if idx := strings.Index(proxy, ":"); idx != -1 {
		proxy = proxy[:idx]
	}

	return proxy
}

// GetCurrencyByRegion 根据地区获取货币代码
func GetCurrencyByRegion(region string) string {
	switch region {
	case "US":
		return "USD"
	case "FR":
		return "EUR"
	case "DE":
		return "EUR"
	case "IT":
		return "EUR"
	case "ES":
		return "EUR"
	case "UK":
		return "GBP"
	case "AU":
		return "AUD"
	case "JP":
		return "JPY"
	case "CA":
		return "CAD"
	case "MX":
		return "MXN"
	case "SA":
		return "SAR" // 沙特里亚尔
	case "AE":
		return "AED" // 阿联酋迪拉姆
	case "CN":
		return "CNY" // 人民币 - 为1688准备
	default:
		return "USD"
	}
}
