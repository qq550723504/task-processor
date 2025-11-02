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

// RequireAuth middleware that requires authentication
func (am *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			am.logger.Errorf("获取session_token cookie失败: %v", err)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		_, exists := am.sessionManager.GetSession(cookie.Value)
		if !exists {
			am.logger.Errorf("会话不存在: %s", cookie.Value)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

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
