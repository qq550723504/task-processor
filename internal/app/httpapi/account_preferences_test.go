package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authidentity"
	store "task-processor/internal/integration/persistence/accountprofile"
	"testing"
)

func TestAccountPreferencesRequireCompleteSelfScopedInput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE account_user_preferences(user_id text PRIMARY KEY,country text,province text,city text,updated_at datetime)`).Error)
	repo, err := store.New(db)
	require.NoError(t, err)
	m := accountProfileModule{repository: repo}
	request := func(method, subject, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		r := httptest.NewRequest(method, "/api/v1/account/preferences", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		c.Request = r.WithContext(authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: subject}))
		m.preferences(c)
		return w
	}
	require.Equal(t, 200, request("PUT", "self", `{"country":"中国","province":"浙江","city":"杭州"}`).Code)
	for _, body := range []string{`{"country":"other","country":"China","province":"","city":""}`, `{"country":null,"province":"","city":""}`, `{"country":"China"}`, `{"country":"China","province":"","city":"","userId":"foreign"}`} {
		require.Equal(t, 400, request("PUT", "self", body).Code, body)
	}
	require.Contains(t, request("GET", "self", "").Body.String(), "杭州")
	require.NotContains(t, request("GET", "foreign", "").Body.String(), "杭州")
}
