package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/pkg/utils"
	"time"

	"github.com/sirupsen/logrus"
)

// LWATokenResponse LWA 令牌响应
type LWATokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// AuthManager 认证管理器
type AuthManager struct {
	clientID     string
	clientSecret string
	refreshToken string
	accessToken  string
	expiresAt    time.Time
	mutex        sync.RWMutex
	httpClient   *http.Client
	logger       *logrus.Entry
}

// NewAuthManager 创建认证管理器
func NewAuthManager(clientID, clientSecret, refreshToken string) *AuthManager {
	return &AuthManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		httpClient:   utils.CreateSimpleHTTPClient(),
		logger: logrus.WithFields(logrus.Fields{
			"component": "AuthManager",
		}),
	}
}

// GetAccessToken 获取访问令牌（自动刷新）
func (a *AuthManager) GetAccessToken(ctx context.Context) (string, error) {
	a.mutex.RLock()
	// 如果令牌有效且未过期（提前5分钟刷新）
	if a.accessToken != "" && time.Now().Add(5*time.Minute).Before(a.expiresAt) {
		token := a.accessToken
		a.mutex.RUnlock()
		return token, nil
	}
	a.mutex.RUnlock()

	// 需要刷新令牌
	return a.refreshAccessToken(ctx)
}

// refreshAccessToken 刷新访问令牌
func (a *AuthManager) refreshAccessToken(ctx context.Context) (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 双重检查，避免重复刷新
	if a.accessToken != "" && time.Now().Add(5*time.Minute).Before(a.expiresAt) {
		return a.accessToken, nil
	}

	a.logger.Info("刷新 Amazon LWA 访问令牌")

	// 构建请求参数
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", a.refreshToken)
	data.Set("client_id", a.clientID)
	data.Set("client_secret", a.clientSecret)

	// 创建请求
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.amazon.com/auth/o2/token",
		bytes.NewBufferString(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("令牌刷新失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var tokenResp LWATokenResponse
	if err := jsonutil.UnmarshalBytes(body, &tokenResp, "解析响应失败"); err != nil {
		return "", err
	}

	// 更新令牌
	a.accessToken = tokenResp.AccessToken
	a.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// 如果返回了新的 refresh token，也更新它
	if tokenResp.RefreshToken != "" {
		a.refreshToken = tokenResp.RefreshToken
	}

	a.logger.WithFields(logrus.Fields{
		"expires_in": tokenResp.ExpiresIn,
		"expires_at": a.expiresAt.Format(time.RFC3339),
	}).Info("访问令牌刷新成功")

	return a.accessToken, nil
}

// SetAccessToken 手动设置访问令牌（用于测试）
func (a *AuthManager) SetAccessToken(token string, expiresIn int) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.accessToken = token
	a.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
}

// IsTokenValid 检查令牌是否有效
func (a *AuthManager) IsTokenValid() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.accessToken != "" && time.Now().Before(a.expiresAt)
}
