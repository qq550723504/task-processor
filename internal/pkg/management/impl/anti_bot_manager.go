// Package impl 提供反机器人检测功能
package impl

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/imroc/req/v3"
)

// AntiBotManager 反机器人管理器
type AntiBotManager struct {
	userAgents []string
}

// NewAntiBotManager 创建新的反机器人管理器
func NewAntiBotManager() *AntiBotManager {
	// 多样化的User-Agent池，模拟不同浏览器和设备
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/120.0.0.0 Safari/537.36",
	}

	return &AntiBotManager{
		userAgents: userAgents,
	}
}

// CreateDynamicRequest 创建动态请求，随机化请求头
func (a *AntiBotManager) CreateDynamicRequest(client *req.Client, targetURL string) *req.Request {
	req := client.R()

	// 随机选择User-Agent
	if len(a.userAgents) > 0 {
		userAgent := a.userAgents[rand.Intn(len(a.userAgents))]
		req.SetHeader("User-Agent", userAgent)
	}

	// 动态设置Referer
	if parsedURL, err := parseURL(targetURL); err == nil {
		referer := fmt.Sprintf("%s://%s/", parsedURL.Scheme, parsedURL.Host)
		req.SetHeader("Referer", referer)
	}

	// 随机化一些请求头
	acceptLanguages := []string{
		"en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		"en-US,en;q=0.9",
		"zh-CN,zh;q=0.9,en;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
	}
	req.SetHeader("Accept-Language", acceptLanguages[rand.Intn(len(acceptLanguages))])

	// 随机化DNT头
	if rand.Float32() < 0.5 {
		req.SetHeader("DNT", "1")
	}

	// 模拟真实浏览器行为：随机添加一些可选头
	if rand.Float32() < 0.3 {
		req.SetHeader("Sec-GPC", "1")
	}

	return req
}

// parseURL 解析URL的辅助函数
func parseURL(rawURL string) (*urlInfo, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("invalid URL scheme")
	}

	var scheme, host string
	if strings.HasPrefix(rawURL, "https://") {
		scheme = "https"
		rawURL = rawURL[8:] // 移除 "https://"
	} else {
		scheme = "http"
		rawURL = rawURL[7:] // 移除 "http://"
	}

	// 找到第一个 '/' 或字符串结尾
	slashIndex := strings.Index(rawURL, "/")
	if slashIndex == -1 {
		host = rawURL
	} else {
		host = rawURL[:slashIndex]
	}

	return &urlInfo{Scheme: scheme, Host: host}, nil
}

// urlInfo URL信息结构
type urlInfo struct {
	Scheme string
	Host   string
}
