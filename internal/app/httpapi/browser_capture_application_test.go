package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"task-processor/internal/httproute"
	"task-processor/internal/product/sourcing"
)

type browserHTTPService struct{ calls []string }

func (s *browserHTTPService) result(action string) (sourcing.AcquisitionResult, error) {
	s.calls = append(s.calls, action)
	return sourcing.AcquisitionResult{Operation: sourcing.AcquisitionOperation{ID: "ec5d1d45-4223-4e90-a451-7a1ebd88d496", State: sourcing.AcquisitionAcquiring}}, nil
}
func (s *browserHTTPService) Capture(context.Context, string, []byte) (sourcing.AcquisitionResult, error) {
	return s.result("capture")
}
func (s *browserHTTPService) Verify(context.Context, string, []byte) (sourcing.AcquisitionResult, error) {
	return s.result("verify")
}
func (s *browserHTTPService) ByKey(context.Context, string) (sourcing.AcquisitionResult, error) {
	return s.result("by-key")
}
func (s *browserHTTPService) Read(context.Context, string) (sourcing.AcquisitionResult, error) {
	return s.result("read")
}

func browserHTTPFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("../../product/sourcing/testdata/browser-capture-v1.json")
	require.NoError(t, err)
	return body
}

func browserHTTPRouter(service *browserHTTPService) *gin.Engine {
	router := gin.New()
	router.RedirectTrailingSlash = false
	router.RedirectFixedPath = false
	bind := func(ctx context.Context, bearer string) (context.Context, error) {
		if bearer != "Bearer fixture-only" {
			return nil, sourcing.ErrPublicationForbidden
		}
		return ctx, nil
	}
	for _, route := range browserCaptureRoutes(service, bind) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	return router
}

func TestBrowserCaptureHTTPFrozenRouteContract(t *testing.T) {
	routes := browserCaptureRoutes(nil, nil)
	require.Len(t, routes, 4)
	require.Equal(t, browserCaptureBase+"/by-key/:key", routes[2].Path)
	require.Equal(t, browserCaptureBase+"/:operation_id", routes[3].Path)
	for _, route := range routes {
		require.Equal(t, httproute.AuthPolicyVerifiedIdentity, route.AuthPolicy)
		require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, route.OrganizationAccessPolicy)
		require.Equal(t, "product_sourcing.write", route.Permission)
		require.Equal(t, sourcing.AcquisitionTimeout, route.RequestTimeout)
		require.True(t, route.RejectUnreadRequestBody)
	}
}

func TestBrowserCaptureHTTPDispatchAndStaticByKey(t *testing.T) {
	for _, tc := range []struct{ method, path, action string }{
		{http.MethodPost, browserCaptureBase, "capture"},
		{http.MethodPost, browserCaptureBase + "/verify", "verify"},
		{http.MethodGet, browserCaptureBase + "/by-key/12aa5bfc-669c-47b6-af07-561178f9c149", "by-key"},
		{http.MethodGet, browserCaptureBase + "/ec5d1d45-4223-4e90-a451-7a1ebd88d496", "read"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			service := &browserHTTPService{}
			router := browserHTTPRouter(service)
			var body []byte
			if tc.method == http.MethodPost {
				body = browserHTTPFixture(t)
			}
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
			request.Header.Set("Authorization", "Bearer fixture-only")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "12aa5bfc-669c-47b6-af07-561178f9c149")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Equal(t, []string{tc.action}, service.calls)
		})
	}
}

func TestBrowserCaptureHTTPRejectsUntrustedEnvelopeBeforeDispatch(t *testing.T) {
	for _, mode := range []string{"no auth", "missing key", "duplicate key header", "invalid key", "wrong media", "encoding", "unknown field", "duplicate field", "oversize", "query", "empty query"} {
		t.Run(mode, func(t *testing.T) {
			service := &browserHTTPService{}
			router := browserHTTPRouter(service)
			body := browserHTTPFixture(t)
			path := browserCaptureBase
			switch mode {
			case "unknown field":
				body = bytes.Replace(body, []byte("{"), []byte(`{"token":"untrusted",`), 1)
			case "duplicate field":
				body = bytes.Replace(body, []byte("{"), []byte(`{"captureVersion":1,`), 1)
			case "oversize":
				body = append([]byte(strings.Repeat(" ", sourcing.BrowserCaptureMaxBytes)), body...)
			case "query":
				path += "?organization=other"
			case "empty query":
				path += "?"
			}
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			request.Header.Set("Authorization", "Bearer fixture-only")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "12aa5bfc-669c-47b6-af07-561178f9c149")
			switch mode {
			case "no auth":
				request.Header.Del("Authorization")
			case "missing key":
				request.Header.Del("Idempotency-Key")
			case "duplicate key header":
				request.Header.Add("Idempotency-Key", "12aa5bfc-669c-47b6-af07-561178f9c149")
			case "invalid key":
				request.Header.Set("Idempotency-Key", "not-a-uuid")
			case "wrong media":
				request.Header.Set("Content-Type", "text/plain")
			case "encoding":
				request.Header.Set("Content-Encoding", "gzip")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.GreaterOrEqual(t, response.Code, 400)
			require.Empty(t, service.calls)
			require.NotContains(t, response.Body.String(), "untrusted")
		})
	}
}

func TestBrowserCaptureHTTPRejectsReadBodiesAndInvalidPaths(t *testing.T) {
	for _, path := range []string{browserCaptureBase + "/by-key/not-a-uuid", browserCaptureBase + "/not-a-uuid", browserCaptureBase + "/by-key/12aa5bfc-669c-47b6-af07-561178f9c149?extra=1", browserCaptureBase + "/by-key/12aa5bfc-669c-47b6-af07-561178f9c149"} {
		service := &browserHTTPService{}
		router := browserHTTPRouter(service)
		request := httptest.NewRequest(http.MethodGet, path, strings.NewReader("{}"))
		request.Header.Set("Authorization", "Bearer fixture-only")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.GreaterOrEqual(t, response.Code, 400)
		require.Empty(t, service.calls)
	}
}

func TestBrowserCaptureHTTPOptionalCompositionPreservesExistingContracts(t *testing.T) {
	for _, tc := range []struct {
		public, browser bool
		count           int
	}{{false, false, 10}, {true, false, 13}, {false, true, 14}, {true, true, 17}} {
		var routes []httproute.Descriptor
		for _, base := range currentWorkbenchApplicationRoutes {
			routes = append(routes, httproute.Descriptor{Method: base.Method, Path: base.Path})
		}
		if tc.public {
			routes = append(routes, productAcquisitionRoutes(nil, nil)...)
		}
		if tc.browser {
			routes = append(routes, browserCaptureRoutes(nil, nil)...)
		}
		require.Len(t, routes, tc.count)
		require.NoError(t, validateCurrentApplicationRoutesForSourcing(routes, tc.public, tc.browser))
		if !tc.browser {
			require.NoError(t, validateCurrentApplicationRoutesForAcquisition(routes, tc.public))
		}
		extra := append(append([]httproute.Descriptor(nil), routes...), httproute.Descriptor{Method: http.MethodPost, Path: browserCaptureBase + "/by-key/:key"})
		require.Error(t, validateCurrentApplicationRoutesForSourcing(extra, tc.public, tc.browser))
	}
}
