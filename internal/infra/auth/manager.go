// Package auth 提供认证功能
package auth

import (
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
)

// SessionManager 会话管理器
type SessionManager struct {
	sessions   map[string]*Session
	tokenStore TokenStore
	mutex      sync.RWMutex
	logger     *logrus.Logger
}

// NewSessionManager 创建会话管理器
func NewSessionManager(tokenStore TokenStore, logger *logrus.Logger) *SessionManager {
	sm := &SessionManager{
		sessions:   make(map[string]*Session),
		tokenStore: tokenStore,
		logger:     logger,
	}

	// 启动清理过期会话的协程
	go sm.cleanupExpiredSessions()

	// 尝试加载持久化的token
	sm.loadPersistedToken()

	return sm
}

// CreateSession 创建用户会话
func (sm *SessionManager) CreateSession(username, tenantID, accessToken, refreshToken string) (string, error) {
	session, err := NewSession(username, tenantID, accessToken, refreshToken)
	if err != nil {
		return "", err
	}

	sm.mutex.Lock()
	sm.sessions[session.Token] = session
	sm.mutex.Unlock()

	// 持久化token
	if err := sm.tokenStore.Save(session); err != nil {
		sm.logger.Warnf("保存token失败: %v", err)
		// 不影响登录流程，继续
	}

	return session.Token, nil
}

// ValidateToken 验证token
func (sm *SessionManager) ValidateToken(token string) (*Session, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[token]
	if !exists {
		return nil, errors.New("invalid token")
	}

	return session, ValidateSession(session)
}

// RevokeToken 撤销token
func (sm *SessionManager) RevokeToken(token string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	delete(sm.sessions, token)

	// 删除持久化的token
	if err := sm.tokenStore.Delete(); err != nil {
		sm.logger.Warnf("删除持久化token失败: %v", err)
	}
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

// GetAllSessions 获取所有会话
func (sm *SessionManager) GetAllSessions() []*Session {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}
