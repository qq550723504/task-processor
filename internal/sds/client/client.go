package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"task-processor/internal/core/logger"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// Client 是 SDS 的登录态 HTTP 客户端。
type Client struct {
	config       *Config
	httpClient   *req.Client
	sessionStore *SessionStore
	authStore    *AuthStateStore
	logger       *logrus.Entry
	cookies      []*http.Cookie
	authState    *AuthState
}

// New 创建 SDS 客户端。
func New(config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	entry := logger.GetGlobalLogger("sds/client")
	httpClient := req.C().
		SetTimeout(config.Timeout).
		SetCommonHeaders(buildDefaultHeaders(config)).
		SetCommonRetryCount(config.RetryCount).
		SetCommonRetryInterval(func(_ *req.Response, attempt int) time.Duration {
			return time.Duration(attempt) * config.RetryInterval
		}).
		SetCommonRetryCondition(func(resp *req.Response, err error) bool {
			if err != nil {
				return true
			}
			return resp != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError)
		}).
		SetTLSFingerprintChrome()

	if strings.TrimSpace(config.ProxyURL) != "" {
		httpClient = httpClient.SetProxyURL(config.ProxyURL)
	}

	client := &Client{
		config:       config,
		httpClient:   httpClient,
		sessionStore: NewSessionStore(config.CookieFile),
		authStore:    NewAuthStateStore(config.AuthFile),
		logger:       entry.WithField("baseURL", config.BaseURL),
	}

	authState, err := client.authStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load sds auth state: %w", err)
	}
	if authState != nil {
		client.SetAuthState(authState)
	}

	cookies, err := client.sessionStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load sds cookies: %w", err)
	}

	if len(cookies) > 0 {
		client.SetCookies(cookies)
	}

	return client, nil
}

// Config 返回客户端配置。
func (c *Client) Config() *Config {
	return c.config
}

// SetCookies 设置并缓存公共 Cookie。
func (c *Client) SetCookies(cookies []*http.Cookie) {
	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		filtered = append(filtered, cookie)
	}

	c.cookies = filtered
	c.httpClient.SetCommonCookies(filtered...)
}

// Cookies 返回当前内存中的 Cookie 快照。
func (c *Client) Cookies() []*http.Cookie {
	result := make([]*http.Cookie, 0, len(c.cookies))
	for _, item := range c.cookies {
		if item == nil {
			continue
		}
		copied := *item
		result = append(result, &copied)
	}
	return result
}

// ImportCookieHeader 导入人工登录后抓到的 Cookie Header。
func (c *Client) ImportCookieHeader(raw string) error {
	cookies := ParseCookieHeader(raw, ".sdsdiy.com")
	if len(cookies) == 0 {
		return fmt.Errorf("no cookies parsed from header")
	}

	c.SetCookies(cookies)

	if err := c.sessionStore.Save(c.cookies); err != nil {
		return fmt.Errorf("persist sds cookies: %w", err)
	}

	return nil
}

// SaveCookies 将当前 Cookie 持久化到本地。
func (c *Client) SaveCookies() error {
	return c.sessionStore.Save(c.cookies)
}

// Ping 检查基础连通性。
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Do(ctx, http.MethodGet, "/", nil, nil, nil)
	return err
}
