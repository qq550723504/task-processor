package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/listingkit"
)

func TestGetAuthContextReturnsVerifiedIdentity(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/auth-context", (&handler{}).GetAuthContext)

	request := httptest.NewRequest(http.MethodGet, "/auth-context", nil).WithContext(
		listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{
			TenantID: "373211199677923496",
			UserID:   "zitadel-user-1",
			Roles:    []string{"listingkit_operator"},
		}),
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"tenant_id":"373211199677923496","user_id":"zitadel-user-1","roles":["listingkit_operator"]}`, response.Body.String())
	require.NotContains(t, response.Body.String(), "billing")
}

func TestGetAuthContextRejectsMissingIdentity(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/auth-context", (&handler{}).GetAuthContext)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth-context", nil))

	require.Equal(t, http.StatusForbidden, response.Code)
	require.NotContains(t, response.Body.String(), "tenant_id")
	require.NotContains(t, response.Body.String(), "user_id")
	require.NotContains(t, response.Body.String(), "roles")
}
