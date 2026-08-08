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

func TestCrawler1688RoutesDoesNotRegisterDuplicateMethodPath(t *testing.T) {
	t.Parallel()

	routes := crawler1688Routes(func(*gin.Context) {})
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		key := route.Method + " " + route.Path
		if _, ok := seen[key]; ok {
			t.Fatalf("crawler-1688 registered duplicate route %q", key)
		}
		seen[key] = struct{}{}
	}
}
