package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/sourcing"
)

type acquisitionHTTPSpy struct {
	acquire, verify, read int
	result                sourcing.AcquisitionResult
	err                   error
}

func (s *acquisitionHTTPSpy) Acquire(context.Context, string, string) (sourcing.AcquisitionResult, error) {
	s.acquire++
	return s.result, s.err
}
func (s *acquisitionHTTPSpy) Verify(context.Context, string, string) (sourcing.AcquisitionResult, error) {
	s.verify++
	return s.result, s.err
}
func (s *acquisitionHTTPSpy) Read(context.Context, string) (sourcing.AcquisitionResult, error) {
	s.read++
	return s.result, s.err
}

func acquisitionHandler(t *testing.T, spy *acquisitionHTTPSpy, bind func(context.Context, string) (context.Context, error)) *gin.Engine {
	t.Helper()
	routes := productAcquisitionRoutes(spy, bind)
	require.Len(t, routes, 3)
	router := gin.New()
	for _, route := range routes {
		require.Equal(t, httproute.AuthPolicyVerifiedIdentity, route.AuthPolicy)
		require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, route.OrganizationAccessPolicy)
		require.Equal(t, "product_sourcing.write", route.Permission)
		require.Equal(t, sourcing.AcquisitionTimeout, route.RequestTimeout)
		router.Handle(route.Method, route.Path, route.Handler)
	}
	return router
}

func TestProductAcquisitionHTTPStrictInputAndBoundedProjection(t *testing.T) {
	operationID := uuid.NewString()
	pubID := "source-run:acquisition:" + operationID
	spy := &acquisitionHTTPSpy{result: sourcing.AcquisitionResult{Operation: sourcing.AcquisitionOperation{ID: operationID, State: sourcing.AcquisitionPublished, Command: &sourcing.PublicationCommand{PublicationID: "MUST_NOT_LEAK", ProductKey: "INTERNAL"}}, Publication: &sourcing.PersistedPublication{Receipt: sourcing.PublicationReceipt{PublicationID: pubID, ProductKey: "crawler:1688:981645030344", CatalogVersion: 1}, Envelope: sourcing.SourceEnvelope{MissingFacts: []sourcing.MissingFact{{Field: "price", Reason: "not provided"}}}}}}
	router := acquisitionHandler(t, spy, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil })
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	key := uuid.NewString()
	response := request("POST", productAcquisitionBase, `{"source":"981645030344"}`, key)
	require.Equal(t, 200, response.Code, response.Body.String())
	var dto map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &dto))
	require.Equal(t, operationID, dto["operationId"])
	require.Equal(t, "1", dto["catalogVersion"])
	require.Equal(t, pubID, dto["publicationId"])
	require.NotContains(t, response.Body.String(), "MUST_NOT_LEAK")
	require.NotContains(t, response.Body.String(), "Command")
	for _, tc := range []struct {
		name, path, body, key string
		status                int
	}{
		{"unknown", productAcquisitionBase, `{"source":"981645030344","organizationId":"other"}`, key, 400},
		{"duplicate", productAcquisitionBase, `{"source":"981645030344","source":"123"}`, key, 400},
		{"null", productAcquisitionBase, `null`, key, 400},
		{"invalid_source", productAcquisitionBase, `{"source":"https://evil.test/offer/123.html"}`, key, 400},
		{"missing_key", productAcquisitionBase, `{"source":"981645030344"}`, "", 400},
		{"bad_key", productAcquisitionBase, `{"source":"981645030344"}`, "not-a-uuid", 400},
		{"query", productAcquisitionBase + "?org=other", `{"source":"981645030344"}`, key, 400},
		{"oversize", productAcquisitionBase, strings.Repeat(" ", 8193), key, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request("POST", tc.path, tc.body, tc.key)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Equal(t, 1, spy.acquire)
		})
	}
	response = request("POST", productAcquisitionBase+"/verify", `{"source":"981645030344"}`, key)
	require.Equal(t, 200, response.Code)
	require.Equal(t, 1, spy.verify)
	require.Equal(t, 1, spy.acquire)
	response = request("GET", productAcquisitionBase+"/"+operationID, "", "")
	require.Equal(t, 200, response.Code)
	require.Equal(t, 1, spy.read)
	spy.result = sourcing.AcquisitionResult{Operation: sourcing.AcquisitionOperation{ID: operationID, State: sourcing.AcquisitionPublishing}}
	response = request("GET", productAcquisitionBase+"/"+operationID, "", "")
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), "outcome_unknown")
	require.NotContains(t, response.Body.String(), "catalogVersion")
	spy.err = sourcing.ErrAcquisitionUnknown
	response = request("POST", productAcquisitionBase+"/verify", `{"source":"981645030344"}`, key)
	require.Equal(t, 503, response.Code)
	require.Contains(t, response.Body.String(), "OUTCOME_UNKNOWN")
	spy.err = errors.New("sensitive SQL or provider response")
	response = request("POST", productAcquisitionBase, `{"source":"981645030344"}`, key)
	require.Equal(t, 503, response.Code)
	require.NotContains(t, response.Body.String(), "sensitive")
}

func TestCurrentApplicationMountsExplicitAcquisitionWithoutChangingDefaultRoutes(t *testing.T) {
	deps := newRouteAuthDependencies()
	spy := &acquisitionHTTPSpy{}
	called := false
	factories := currentApplicationFactories{
		buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			return workbenchContextBuildResult{module: currentApplicationTestModule{name: "workbench", routes: currentWorkbenchApplicationRoutes[:4]}, authDependencies: &deps}, nil
		},
		buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return currentApplicationTestModule{name: "source", routes: currentWorkbenchApplicationRoutes[5:]}, nil
		},
		buildCommercial: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return currentApplicationTestModule{name: "commercial", routes: currentWorkbenchApplicationRoutes[4:5]}, nil
		},
		buildAcquisition: func(auth *authz.ListingKitAuthorizer, got routeAuthDependencies) (kernelmodule.Module, error) {
			called = true
			require.NotNil(t, auth)
			require.Equal(t, auth, got.authorizer)
			binder := productReviewCapabilityBinder{}
			return productAcquisitionModule{routes: productAcquisitionRoutes(spy, binder.Bind)}, nil
		},
	}
	server, err := buildCurrentApplication(context.Background(), &gorm.DB{}, &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories)
	require.NoError(t, err)
	require.True(t, called)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, productAcquisitionBase, strings.NewReader(`{"source":"981645030344"}`)))
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	require.Zero(t, spy.acquire)
	require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	require.Len(t, currentWorkbenchApplicationRoutes, 10, "the existing default contract is unchanged")
	factories.buildAcquisition = nil
	server, err = buildCurrentApplication(context.Background(), &gorm.DB{}, &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, productAcquisitionBase, nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestProductAcquisitionHTTPBindsLiveCapabilityOnAllActions(t *testing.T) {
	spy := &acquisitionHTTPSpy{}
	binds := 0
	router := acquisitionHandler(t, spy, func(ctx context.Context, _ string) (context.Context, error) {
		binds++
		return ctx, sourcing.ErrPublicationForbidden
	})
	for _, tc := range []struct{ method, path string }{{"POST", productAcquisitionBase}, {"POST", productAcquisitionBase + "/verify"}, {"GET", productAcquisitionBase + "/" + uuid.NewString()}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"source":"981645030344"}`)))
		require.Equal(t, 403, w.Code)
	}
	require.Equal(t, 3, binds)
	require.Zero(t, spy.acquire+spy.verify+spy.read)
}
