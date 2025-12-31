// Package auth 提供认证功能
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
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

// IsExpired 检查会话是否过期
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid 检查会话是否有效
func (s *Session) IsValid() bool {
	return !s.IsExpired() && s.Token != "" && s.AccessToken != ""
}

// generateToken 生成随机token
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// NewSession 创建新会话
func NewSession(username, tenantID, accessToken, refreshToken string) (*Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	return &Session{
		Token:        token,
		Username:     username,
		TenantID:     tenantID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour), // 30天过期
	}, nil
}

// ValidateSession 验证会话
func ValidateSession(session *Session) error {
	if session == nil {
		return errors.New("session is nil")
	}

	if session.Token == "" {
		return errors.New("invalid token")
	}

	if session.IsExpired() {
		return errors.New("token expired")
	}

	return nil
}
