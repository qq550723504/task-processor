// Package middleware 提供HTTP中间件
package httpx

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// LoggingMiddleware HTTP请求日志中间件
func LoggingMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Infof("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}
