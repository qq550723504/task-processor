package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"task-processor/internal/listingsubscription"
)

type readerStub struct {
	calls  int
	result *listingsubscription.CommercialOverview
	err    error
}

func (r *readerStub) Read(context.Context) (*listingsubscription.CommercialOverview, error) {
	r.calls++
	return r.result, r.err
}

func TestCommercialHTTPRejectsInputsBeforeRead(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/v1/workbench/commercial/overview?org=other", ""},
		{"GET", "/api/v1/workbench/commercial/overview", "{}"},
		{"POST", "/api/v1/workbench/commercial/overview", ""},
	} {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			r := &readerStub{}
			engine := gin.New()
			engine.Any("/api/v1/workbench/commercial/overview", NewHandler(r).Get)
			out := httptest.NewRecorder()
			engine.ServeHTTP(out, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			require.Contains(t, []int{400, 405}, out.Code)
			require.Zero(t, r.calls)
			require.Contains(t, out.Header().Get("Cache-Control"), "no-store")
		})
	}
}

func TestCommercialHTTPDoesNotLeakDependencyFailure(t *testing.T) {
	r := &readerStub{err: listingsubscription.ErrCommercialUnavailable}
	engine := gin.New()
	engine.GET("/", NewHandler(r).Get)
	out := httptest.NewRecorder()
	engine.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 503, out.Code)
	require.Contains(t, out.Body.String(), `"code":"DEPENDENCY_UNAVAILABLE"`)
}
