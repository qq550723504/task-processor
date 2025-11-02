package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// TokenResponse OAuth2令牌响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// APIResponse API响应包装结构
type APIResponse struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data *TokenResponse `json:"data"`
}

// UserSession 用户会话信息
type UserSession struct {
	Username     string    `json:"username"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TenantID     string    `json:"tenant_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// PasswordAuthClient 密码认证客户端
type PasswordAuthClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions map[string]*UserSession
	mutex    sync.RWMutex
	filePath string
}

// NewPasswordAuthClient 创建密码认证客户端
func NewPasswordAuthClient(baseURL, clientID, clientSecret string) *PasswordAuthClient {
	return &PasswordAuthClient{
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Login 用户登录
func (c *PasswordAuthClient) Login(username, password, tenantID string) (*UserSession, error) {
	tokenURL := c.baseURL + "/admin-api/system/oauth2/token"

	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", username)
	data.Set("password", password)
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("scope", "user.read")

	if tenantID != "" {
		data.Set("tenant_id", tenantID)
	}

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// 尝试通过Header传递租户ID
	if tenantID != "" {
		req.Header.Set("tenant-id", tenantID)
		req.Header.Set("Tenant-Id", tenantID)
		req.Header.Set("X-Tenant-Id", tenantID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("登录失败: %s", string(body))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析API响应失败: %w", err)
	}

	// 检查API响应状态
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("登录失败: %s (code: %d)", apiResp.Msg, apiResp.Code)
	}

	// 检查data字段是否存在
	if apiResp.Data == nil {
		return nil, fmt.Errorf("API响应中缺少data字段")
	}

	tokenResp := apiResp.Data

	// 计算过期时间，如果服务器返回的过期时间太短，使用默认的24小时
	expiresIn := tokenResp.ExpiresIn
	if expiresIn < 3600 { // 如果少于1小时，使用24小时作为默认值
		expiresIn = 24 * 3600 // 24小时
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	session := &UserSession{
		Username:     username,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TenantID:     tenantID,
		ExpiresAt:    expiresAt,
	}

	logrus.Infof("创建的用户会话: Username=%s, AccessToken=%s..., TenantID=%s",
		session.Username, session.AccessToken[:min(len(session.AccessToken), 20)], session.TenantID)

	return session, nil
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*UserSession),
		filePath: "data/token.json",
	}
	sm.loadSessions()
	return sm
}

// SaveSession 保存会话
func (sm *SessionManager) SaveSession(username string, session *UserSession) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.sessions[username] = session
	sm.persistSessions()
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(username string) (*UserSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[username]
	if !exists {
		return nil, false
	}

	// 检查会话是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// GetAllSessions 获取所有会话
func (sm *SessionManager) GetAllSessions() map[string]*UserSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]*UserSession)
	for k, v := range sm.sessions {
		if time.Now().Before(v.ExpiresAt) {
			result[k] = v
		}
	}

	return result
}

// RemoveSession 移除会话
func (sm *SessionManager) RemoveSession(username string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	delete(sm.sessions, username)
	sm.persistSessions()
}

// loadSessions 加载持久化的会话
func (sm *SessionManager) loadSessions() {
	if _, err := os.Stat(sm.filePath); os.IsNotExist(err) {
		return
	}

	data, err := ioutil.ReadFile(sm.filePath)
	if err != nil {
		logrus.Warnf("读取会话文件失败: %v", err)
		return
	}

	var sessions map[string]*UserSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		logrus.Warnf("解析会话文件失败: %v", err)
		return
	}

	sm.sessions = sessions
	logrus.Infof("加载了 %d 个持久化会话", len(sessions))
}

// persistSessions 持久化会话
func (sm *SessionManager) persistSessions() {
	// 确保目录存在
	if err := os.MkdirAll("data", 0755); err != nil {
		logrus.Errorf("创建数据目录失败: %v", err)
		return
	}

	data, err := json.MarshalIndent(sm.sessions, "", "  ")
	if err != nil {
		logrus.Errorf("序列化会话失败: %v", err)
		return
	}

	if err := ioutil.WriteFile(sm.filePath, data, 0644); err != nil {
		logrus.Errorf("保存会话文件失败: %v", err)
		return
	}
}

// ClientCredentialsAuth 客户端凭证认证
type ClientCredentialsAuth struct {
	config *clientcredentials.Config
	token  *oauth2.Token
	mutex  sync.RWMutex
}

// NewClientCredentialsAuth 创建客户端凭证认证
func NewClientCredentialsAuth(tokenURL, clientID, clientSecret string, scopes []string) *ClientCredentialsAuth {
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes:       scopes,
	}

	return &ClientCredentialsAuth{
		config: config,
	}
}

// GetToken 获取访问令牌
func (c *ClientCredentialsAuth) GetToken(ctx context.Context) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查现有令牌是否有效
	if c.token != nil && c.token.Valid() {
		return c.token.AccessToken, nil
	}

	// 获取新令牌
	token, err := c.config.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("获取访问令牌失败: %w", err)
	}

	c.token = token
	return token.AccessToken, nil
}
