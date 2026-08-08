package httpapi

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCrawler1688RoutesDoesNotRegisterSharedHealthEndpoint(t *testing.T) {
	t.Parallel()

	routes := crawler1688Routes(func(*gin.Context) {})
	for _, route := range routes {
		if route.Method == http.MethodGet && route.Path == "/health" {
			t.Fatal("crawler-1688 must not register the shared GET /health endpoint")
		}
	}
}
