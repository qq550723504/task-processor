package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

// ClientCredentialsAuthClient 客户端凭证模式授权客户端
type ClientCredentialsAuthClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	tenantID     string
	httpClient   *http.Client
	accessToken  string
	expiresAt    time.Time
}

// ClientCredentialsTokenResponse 客户端凭证令牌响应
type ClientCredentialsTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// NewClientCredentialsAuthClient 创建客户端凭证模式授权客户端
func NewClientCredentialsAuthClient(baseURL, clientID, clientSecret, tenantID string) *ClientCredentialsAuthClient {
	return &ClientCredentialsAuthClient{
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// GetAccessToken 获取访问令牌（如果已过期则自动刷新）
func (c *ClientCredentialsAuthClient) GetAccessToken() (string, error) {
	// 如果token还有效，直接返回
	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		return c.accessToken, nil
	}

	// 否则获取新token
	return c.fetchAccessToken()
}

// fetchAccessToken 获取新的访问令牌
func (c *ClientCredentialsAuthClient) fetchAccessToken() (string, error) {
	tokenURL := c.baseURL + "/admin-api/system/oauth2/token"

	// 构建表单数据
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("scope", "user.read")

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("tenant-id", c.tenantID)

	// 不输出敏感信息到日志
	logrus.Infof("请求客户端凭证令牌: URL=%s", tokenURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取令牌失败 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	// 先尝试解析为业务包装响应（常见格式）
	var businessResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
			TokenType    string `json:"token_type"`
			Scope        string `json:"scope"`
		} `json:"data"`
	}

	var accessToken string
	var expiresIn int64

	if err := json.Unmarshal(body, &businessResp); err == nil && businessResp.Data.AccessToken != "" {
		// 成功解析为业务响应格式
		logrus.Infof("解析为业务响应格式: code=%d, msg=%s", businessResp.Code, businessResp.Msg)
		if businessResp.Code != 0 {
			return "", fmt.Errorf("获取令牌失败: %s (code: %d)", businessResp.Msg, businessResp.Code)
		}
		accessToken = businessResp.Data.AccessToken
		expiresIn = businessResp.Data.ExpiresIn
		logrus.Info("成功从业务响应中提取access_token")
	} else {
		// 尝试解析为标准 OAuth2 响应
		var tokenResp ClientCredentialsTokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return "", fmt.Errorf("解析响应失败: %w，响应内容: %s", err, string(body))
		}
		if tokenResp.AccessToken == "" {
			return "", fmt.Errorf("响应中没有access_token，响应内容: %s", string(body))
		}
		accessToken = tokenResp.AccessToken
		expiresIn = tokenResp.ExpiresIn
		logrus.Info("成功从标准OAuth2响应中提取access_token")
	}

	if accessToken == "" {
		return "", fmt.Errorf("响应中没有access_token，响应内容: %s", string(body))
	}

	// 保存token和过期时间
	c.accessToken = accessToken
	if expiresIn > 0 {
		// 提前5分钟过期，避免边界情况
		c.expiresAt = time.Now().Add(time.Duration(expiresIn)*time.Second - 5*time.Minute)
	} else {
		// 默认1个月过期
		c.expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}

	logrus.Infof("成功获取客户端凭证令牌，过期时间: %s", c.expiresAt.Format("2006-01-02 15:04:05"))

	return c.accessToken, nil
}

// GetTenantID 获取租户ID
func (c *ClientCredentialsAuthClient) GetTenantID() string {
	return c.tenantID
}
