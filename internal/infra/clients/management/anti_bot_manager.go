package management

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
	return &AntiBotManager{
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/120.0.0.0 Safari/537.36",
		},
	}
}

// CreateDynamicRequest 创建动态请求，随机化请求头
func (a *AntiBotManager) CreateDynamicRequest(client *req.Client, targetURL string) *req.Request {
	r := client.R()

	if len(a.userAgents) > 0 {
		r.SetHeader("User-Agent", a.userAgents[rand.Intn(len(a.userAgents))])
	}

	if parsedURL, err := parseURL(targetURL); err == nil {
		r.SetHeader("Referer", fmt.Sprintf("%s://%s/", parsedURL.Scheme, parsedURL.Host))
	}

	acceptLanguages := []string{
		"en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		"en-US,en;q=0.9",
		"zh-CN,zh;q=0.9,en;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
	}
	r.SetHeader("Accept-Language", acceptLanguages[rand.Intn(len(acceptLanguages))])

	if rand.Float32() < 0.5 {
		r.SetHeader("DNT", "1")
	}
	if rand.Float32() < 0.3 {
		r.SetHeader("Sec-GPC", "1")
	}

	return r
}

func parseURL(rawURL string) (*urlInfo, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("invalid URL scheme")
	}

	var scheme string
	if strings.HasPrefix(rawURL, "https://") {
		scheme = "https"
		rawURL = rawURL[8:]
	} else {
		scheme = "http"
		rawURL = rawURL[7:]
	}

	host := rawURL
	if idx := strings.Index(rawURL, "/"); idx != -1 {
		host = rawURL[:idx]
	}

	return &urlInfo{Scheme: scheme, Host: host}, nil
}

type urlInfo struct {
	Scheme string
	Host   string
}
