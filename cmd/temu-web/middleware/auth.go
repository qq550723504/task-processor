package middleware

import (
	"net/http"

	"task-processor/common/auth"

	"github.com/sirupsen/logrus"
)

// AuthMiddleware provides authentication middleware
type AuthMiddleware struct {
	sessionManager *auth.SessionManager
	logger         *logrus.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(sessionManager *auth.SessionManager, logger *logrus.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
		logger:         logger,
	}
}

// RequireAuth middleware that requires authentication (客户端凭证模式下直接放行)
func (am *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 使用客户端凭证模式，不需要会话验证，直接放行
		next(w, r)
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Infof("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}
