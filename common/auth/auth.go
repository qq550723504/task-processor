package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Session 会话信息
type Session struct {
	Token        string
	Username     string
	TenantID     string // 租户ID
	AccessToken  string // 访问令牌（用于调用Java API）
	RefreshToken string // 刷新令牌
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions   map[string]*Session
	tokenStore *TokenStore // token持久化存储
	mutex      sync.RWMutex
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions:   make(map[string]*Session),
		tokenStore: NewTokenStore("data/token.json"),
	}
	// 启动清理过期会话的协程
	go sm.cleanupExpiredSessions()
	// 尝试加载持久化的token
	sm.loadPersistedToken()
	return sm
}

// loadPersistedToken 加载持久化的token
func (sm *SessionManager) loadPersistedToken() {
	storedToken, err := sm.tokenStore.Load()
	if err != nil {
		logrus.Infof("加载持久化token失败: %v", err)
		return
	}

	if storedToken == nil {
		logrus.Info("没有找到持久化的token")
		return
	}

	// 恢复会话
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session := &Session{
		Token:        storedToken.Token,
		Username:     storedToken.Username,
		TenantID:     storedToken.TenantID,
		AccessToken:  storedToken.AccessToken,
		RefreshToken: storedToken.RefreshToken,
		CreatedAt:    storedToken.CreatedAt,
		ExpiresAt:    storedToken.ExpiresAt,
	}

	sm.sessions[storedToken.Token] = session
	logrus.Infof("已恢复会话: 用户=%s, 租户=%s, 剩余有效期=%v",
		storedToken.Username, storedToken.TenantID, time.Until(storedToken.ExpiresAt).Round(time.Minute))
}

// ValidateToken 验证token
func (sm *SessionManager) ValidateToken(token string) (*Session, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[token]
	if !exists {
		return nil, errors.New("invalid token")
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, errors.New("token expired")
	}

	return session, nil
}

// RevokeToken 撤销token
func (sm *SessionManager) RevokeToken(token string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	delete(sm.sessions, token)

	// 删除持久化的token
	if err := sm.tokenStore.Delete(); err != nil {
		logrus.Infof("警告: 删除持久化token失败: %v", err)
	}
}

// CreateSession 创建用户会话
func (sm *SessionManager) CreateSession(username, tenantID, accessToken, refreshToken string) (string, error) {
	// 生成会话token
	token, err := generateToken()
	if err != nil {
		return "", err
	}

	// 创建会话
	session := &Session{
		Token:        token,
		Username:     username,
		TenantID:     tenantID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour), // 30天过期
	}

	sm.mutex.Lock()
	sm.sessions[token] = session
	sm.mutex.Unlock()

	// 持久化token
	storedToken := &StoredToken{
		Token:        token,
		Username:     username,
		TenantID:     tenantID,
		ExpiresAt:    session.ExpiresAt,
		CreatedAt:    session.CreatedAt,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	if err := sm.tokenStore.Save(storedToken); err != nil {
		logrus.Infof("警告: 保存token失败: %v", err)
		// 不影响登录流程，继续
	}

	return token, nil
}

// GetAccessToken 获取用户的访问令牌
func (sm *SessionManager) GetAccessToken(sessionToken string) (string, error) {
	session, err := sm.ValidateToken(sessionToken)
	if err != nil {
		return "", err
	}
	return session.AccessToken, nil
}

// GetTenantID 获取用户的租户ID
func (sm *SessionManager) GetTenantID(sessionToken string) (string, error) {
	session, err := sm.ValidateToken(sessionToken)
	if err != nil {
		return "", err
	}
	return session.TenantID, nil
}

// GetAllSessions 获取所有会话（用于恢复会话）
func (sm *SessionManager) GetAllSessions() []*Session {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// cleanupExpiredSessions 清理过期会话
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.mutex.Lock()
		now := time.Now()

		// 清理过期会话
		for token, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, token)
			}
		}

		sm.mutex.Unlock()
	}
}

// generateToken 生成随机token
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
