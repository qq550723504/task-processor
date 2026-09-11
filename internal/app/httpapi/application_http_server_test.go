package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/httproute"
)

func TestBuildIsolatedApplicationHTTPServerPreservesShellContract(t *testing.T) {
	const applicationTimeout = 37 * time.Second
	type contextKey struct{}
	contextValue := &struct{}{}

	var observedDeadline time.Time
	var observedValue any
	var observedErr error
	routes := []httproute.Descriptor{{
		Method:         http.MethodGet,
		Path:           "/contract",
		AuthPolicy:     httproute.AuthPolicyPublic,
		RequestTimeout: time.Hour,
		Handler: func(c *gin.Context) {
			observedDeadline, _ = c.Request.Context().Deadline()
			observedValue = c.Request.Context().Value(contextKey{})
			observedErr = c.Request.Context().Err()
			c.Status(http.StatusNoContent)
		},
	}}
	server := buildIsolatedApplicationHTTPServer(routes, routeAuthDependencies{}, applicationTimeout)

	require.Equal(t, "127.0.0.1:0", server.Addr)
	require.Equal(t, 5*time.Second, server.ReadHeaderTimeout)
	require.Equal(t, applicationTimeout, server.ReadTimeout)
	require.Equal(t, applicationTimeout+2*time.Second, server.WriteTimeout)

	parentDeadline := time.Now().Add(5 * time.Second)
	parent, cancel := context.WithDeadline(context.WithValue(context.Background(), contextKey{}, contextValue), parentDeadline)
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/contract", nil).WithContext(parent)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	require.Same(t, contextValue, observedValue)
	require.NoError(t, observedErr)
	require.WithinDuration(t, parentDeadline, observedDeadline, 100*time.Millisecond, "the application and route wrappers must not extend an earlier caller deadline")

	notFound := httptest.NewRecorder()
	server.Handler.ServeHTTP(notFound, httptest.NewRequest(http.MethodGet, "/not-found", nil))
	require.Equal(t, http.StatusNotFound, notFound.Code)
	require.Equal(t, "no-store", notFound.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", notFound.Header().Get("X-Content-Type-Options"))

	canceled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	canceledResponse := httptest.NewRecorder()
	server.Handler.ServeHTTP(canceledResponse, httptest.NewRequest(http.MethodGet, "/contract", nil).WithContext(canceled))
	require.ErrorIs(t, observedErr, context.Canceled, "the application wrapper must preserve parent cancellation")
}
