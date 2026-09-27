package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestZitadelSMSHandlerUsesInjectedHTTPAdapter(t *testing.T) {
	for _, configured := range []bool{false, true} {
		calls := 0
		var deliver gin.HandlerFunc
		if configured {
			deliver = func(c *gin.Context) { calls++; c.Status(http.StatusNoContent) }
		}
		h, err := NewHandler(&stubHandlerCoreService{}, WithDependencies(HandlerDependencies{ZitadelSMSHandler: deliver}))
		require.NoError(t, err)
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		h.DeliverZitadelSMS(c)
		c.Writer.WriteHeaderNow()
		if configured {
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, 1, calls)
		} else {
			require.Equal(t, http.StatusServiceUnavailable, response.Code)
			require.Zero(t, calls)
		}
	}
}
