package dataserviceauth

import (
	"context"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActiveUserReaderUsesOfficialExactUserAndFailsClosed(t *testing.T) {
	response := `{"user":{"userId":"creator","state":"USER_STATE_ACTIVE"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/users/creator", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "Bearer service-only", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	reader, err := NewActiveUserClient(server.URL, server.Client())
	require.NoError(t, err)
	active, err := reader.IsUserActive(context.Background(), "service-only", "creator")
	require.NoError(t, err)
	require.True(t, active)
	response = `{"user":{"userId":"creator","state":"USER_STATE_INACTIVE"}}`
	active, err = reader.IsUserActive(context.Background(), "service-only", "creator")
	require.NoError(t, err)
	require.False(t, active)
	for _, invalid := range []string{`{"user":{"userId":"foreign","state":"USER_STATE_ACTIVE"}}`, `{}`, strings.Repeat("x", 65537)} {
		response = invalid
		active, err = reader.IsUserActive(context.Background(), "service-only", "creator")
		require.Error(t, err)
		require.False(t, active)
	}
}
