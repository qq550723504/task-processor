// Package auth 提供认证功能
package auth

import "time"

// loadPersistedToken 加载持久化的token
func (sm *SessionManager) loadPersistedToken() {
	session, err := sm.tokenStore.Load()
	if err != nil {
		sm.logger.Infof("加载持久化token失败: %v", err)
		return
	}

	if session == nil {
		sm.logger.Info("没有找到持久化的token")
		return
	}

	// 验证会话是否有效
	if err := ValidateSession(session); err != nil {
		sm.logger.Infof("持久化的token无效: %v", err)
		return
	}

	// 恢复会话
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.sessions[session.Token] = session
	sm.logger.Infof("已恢复会话: 用户=%s, 租户=%s, 剩余有效期=%v",
		session.Username, session.TenantID, time.Until(session.ExpiresAt).Round(time.Minute))
}

// cleanupExpiredSessions 清理过期会话
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanupOnce()
	}
}

// cleanupOnce 执行一次清理
func (sm *SessionManager) cleanupOnce() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	now := time.Now()
	expiredCount := 0

	// 清理过期会话
	for token, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			delete(sm.sessions, token)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		sm.logger.Infof("清理了 %d 个过期会话", expiredCount)
	}
}
