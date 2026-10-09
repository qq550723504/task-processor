package storeobservationshttp

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	o "task-processor/internal/marketplace/shein/observations"
	"testing"
)

func TestListQueryBoundedAndUnique(t *testing.T) {
	store := uuid.NewString()
	r := httptest.NewRequest("GET", "/orders?storeId="+store+"&limit=20&status=exceptional&keyword=shoe", nil)
	q, e := listQuery(r, o.Orders)
	require.NoError(t, e)
	require.Equal(t, []string{store}, q.Stores)
	require.Equal(t, 20, q.Limit)
	for _, raw := range []string{"limit=51", "storeId=" + store + "&storeId=" + store, "url=https://evil.test", "limit=01", "keyword=bad%0A", "limit=10;keyword=x"} {
		_, e = listQuery(httptest.NewRequest("GET", "/orders?"+raw, nil), o.Orders)
		require.ErrorIs(t, e, o.ErrInvalid)
	}
}

type unreadBody struct{}

func (unreadBody) Read([]byte) (int, error) { panic("read-only request body must not be consumed") }
func (unreadBody) Close() error             { return nil }
func TestReadRejectsBodyWithoutBlockingOrBindingIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := Routes(nil, nil)
	engine := gin.New()
	for _, route := range routes {
		engine.Handle(route.Method, route.Path, route.Handler)
	}
	request := httptest.NewRequest("GET", BasePath+"/orders", nil)
	request.Body = unreadBody{}
	request.ContentLength = 1
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, 400, response.Code)
	for _, body := range []string{`{"kind":"orders","stores":[],"url":"https://evil.test"}`, `{"kind":"orders","kind":"orders","stores":[]}`} {
		r := httptest.NewRequest(http.MethodPost, BasePath+"/orders/syncs", io.NopCloser(strings.NewReader(body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", uuid.NewString())
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		require.Equal(t, 400, w.Code)
	}
}
