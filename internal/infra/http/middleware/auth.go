// Package middleware 提供HTTP中间件
package middleware

import (
	"net/http"
	"task-processor/internal/infra/auth"

	"github.com/sirupsen/logrus"
)

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	sessionManager *auth.SessionManager
	logger         *logrus.Logger
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(sessionManager *auth.SessionManager, logger *logrus.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
		logger:         logger,
	}
}

// RequireAuth 需要认证的中间件（客户端凭证模式下直接放行）
func (am *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 使用客户端凭证模式，不需要会话验证，直接放行
		next(w, r)
	}
}
