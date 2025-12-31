// Package auth 提供认证功能
package auth

// ClientCredentialsClient 客户端凭证认证接口
type ClientCredentialsClient interface {
	GetAccessToken() (string, error)
	GetTenantID() string
}

// SessionManagerInterface 会话管理器接口
type SessionManagerInterface interface {
	CreateSession(username, tenantID, accessToken, refreshToken string) (string, error)
	ValidateToken(token string) (*Session, error)
	RevokeToken(token string)
	GetAccessToken(sessionToken string) (string, error)
	GetTenantID(sessionToken string) (string, error)
	GetAllSessions() []*Session
}
