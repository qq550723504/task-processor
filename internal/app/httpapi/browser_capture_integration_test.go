package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/workbenchcontext"
	workbenchcontexthttpapi "task-processor/internal/workbenchcontext/httpapi"
)

// Reuse the acquisition fixture's new isolated database, runtime role and
// external identity/grant doubles. All mounted route middleware, live access,
// Browser service, staging, SRC-1 and Catalog persistence remain production code.
type browserFixtureGrants struct{ *titleGrants }

func (g browserFixtureGrants) Load(ctx context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	result, err := g.titleGrants.Load(ctx, source, request)
	for i := range result.Grants {
		result.Grants[i].OrganizationName = "Fixture " + result.Grants[i].OrganizationID
	}
	return result, err
}

func (f *acquisitionHTTPFixture) browserServer(t *testing.T) *httptest.Server {
	t.Helper()
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	deps.organizationResolver = workbenchcontext.NewResolver(browserFixtureGrants{f.grants}, "project", "v1", nil)
	factories := currentApplicationFactories{
		buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			return workbenchContextBuildResult{module: workbenchcontexthttpapi.NewModule(workbenchcontexthttpapi.NewHandler()), authDependencies: &deps}, nil
		},
		buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return currentApplicationTestModule{name: "source", routes: currentWorkbenchApplicationRoutes[5:]}, nil
		},
		buildCommercial: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return currentApplicationTestModule{name: "commercial", routes: currentWorkbenchApplicationRoutes[4:5]}, nil
		},
		buildAcquisition: func(auth *authz.ListingKitAuthorizer, d routeAuthDependencies) (kernelmodule.Module, error) {
			return buildProductAcquisitionModule(context.Background(), f.db, d, auth, f.provider)
		},
		buildBrowserCapture: func(auth *authz.ListingKitAuthorizer, d routeAuthDependencies) (kernelmodule.Module, error) {
			return buildBrowserCaptureModule(context.Background(), f.db, d, auth)
		},
	}
	server, err := buildCurrentApplication(context.Background(), &gorm.DB{}, &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories)
	require.NoError(t, err)
	result := httptest.NewServer(server.Handler)
	t.Cleanup(result.Close)
	return result
}

func TestBrowserCaptureMountedFullChainExactReceiptAndLiveAuthorization(t *testing.T) {
	f := newAcquisitionHTTPFixture(t)
	server := f.browserServer(t)
	body := string(browserHTTPFixture(t))
	key := uuid.NewString()
	status, rawContext, err := acquisitionHTTPRequest(server, "GET", "/api/v1/workbench/context", "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, 200, status)
	var projection struct {
		Organizations []struct {
			Name string `json:"name"`
		} `json:"organizations"`
	}
	require.NoError(t, json.Unmarshal(rawContext, &projection))
	require.Len(t, projection.Organizations, 2)
	for _, organization := range projection.Organizations {
		require.NotEmpty(t, organization.Name, "the real Web context contract requires a provider display name")
	}
	contextReads := f.grants.cached.Load()
	for _, actor := range []string{"", "bad", "viewer"} {
		status := http.StatusUnauthorized
		if actor == "viewer" {
			status = http.StatusForbidden
		}
		acquisitionHTTPCall(t, server, "POST", browserCaptureBase, actor, "B", uuid.NewString(), body, status)
	}
	first := acquisitionHTTPCall(t, server, "POST", browserCaptureBase, "operator", "B", key, body, 200)
	require.Equal(t, "published", first.Outcome)
	require.Equal(t, "1", first.CatalogVersion)
	require.Equal(t, "crawler:1688:981645030344", first.ProductKey)
	require.NotEqual(t, key, first.OperationID)
	require.NotEmpty(t, first.MissingFacts)
	require.Zero(t, f.fetches.Load())
	public := acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", uuid.NewString(), `{"source":"981645030344"}`, 200)
	require.Equal(t, "2", public.CatalogVersion)
	for _, action := range []struct{ method, path, key, body string }{
		{"POST", browserCaptureBase, key, body}, {"POST", browserCaptureBase + "/verify", key, body},
		{"GET", browserCaptureBase + "/by-key/" + key, "", ""}, {"GET", browserCaptureBase + "/" + first.OperationID, "", ""},
	} {
		replay := acquisitionHTTPCall(t, server, action.method, action.path, "operator", "B", action.key, action.body, 200)
		require.True(t, replay.Replayed)
		require.Equal(t, first.PublicationID, replay.PublicationID)
		require.Equal(t, "1", replay.CatalogVersion)
	}
	acquisitionHTTPCall(t, server, "POST", browserCaptureBase, "operator", "B", key, strings.Replace(body, "12.34000001", "12.34000002", 1), 409)
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", key, `{"source":"981645030344"}`, 409)
	acquisitionHTTPCall(t, server, "GET", browserCaptureBase+"/by-key/"+key, "operator", "A", "", "", 404)
	acquisitionHTTPCall(t, server, "GET", browserCaptureBase+"/by-key/"+key, "operator2", "B", "", "", 404)
	f.grants.revoked.Store(true)
	acquisitionHTTPCall(t, server, "GET", browserCaptureBase+"/by-key/"+key, "operator", "B", "", "", 403)
	acquisitionHTTPCall(t, server, "POST", browserCaptureBase+"/verify", "operator", "B", key, body, 403)
	acquisitionHTTPCall(t, server, "POST", browserCaptureBase, "operator", "B", uuid.NewString(), body, 403)
	require.Equal(t, contextReads, f.grants.cached.Load(), "Browser routes must not use the context-read cache")
	require.EqualValues(t, 1, f.fetches.Load())
	for _, table := range []string{"product_acquisition_operations", "product_snapshot_versions", "product_source_publications", "product_source_publication_receipts"} {
		var count int64
		require.NoError(t, f.owner.Table(table).Count(&count).Error)
		require.EqualValues(t, 2, count, table)
	}
	var home int64
	require.NoError(t, f.owner.Table("product_snapshot_versions").Where("tenant_id = ?", "A").Count(&home).Error)
	require.Zero(t, home)
}

func TestBrowserCaptureMountedLostResponseRebuildAndReadOnlyRecovery(t *testing.T) {
	f := newAcquisitionHTTPFixture(t)
	server := f.browserServer(t)
	lost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		server.Config.Handler.ServeHTTP(recorder, r)
		if recorder.Code != 200 {
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(lost.Close)
	key := uuid.NewString()
	body := string(browserHTTPFixture(t))
	_, _, err := acquisitionHTTPRequest(lost, "POST", browserCaptureBase, "operator", "B", key, body)
	require.Error(t, err)
	// A lost HTTP response cannot justify another POST. Restart composition,
	// then force runtime transactions read-only before the recovery reads.
	rebuilt := f.browserServer(t)
	pool, err := f.db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	require.NoError(t, f.db.Exec("SET default_transaction_read_only=on").Error)
	var readOnly string
	require.NoError(t, f.db.Raw("SHOW default_transaction_read_only").Scan(&readOnly).Error)
	require.Equal(t, "on", readOnly)
	for _, action := range []struct{ method, path, key, body string }{{"GET", browserCaptureBase + "/by-key/" + key, "", ""}, {"POST", browserCaptureBase + "/verify", key, body}} {
		result := acquisitionHTTPCall(t, rebuilt, action.method, action.path, "operator", "B", action.key, action.body, 200)
		require.Equal(t, "published", result.Outcome)
		require.Equal(t, "1", result.CatalogVersion)
	}
	require.Zero(t, f.fetches.Load())
	for _, table := range []string{"product_acquisition_operations", "product_snapshot_versions", "product_source_publications", "product_source_publication_receipts"} {
		var count int64
		require.NoError(t, f.owner.Table(table).Count(&count).Error)
		require.EqualValues(t, 1, count, table)
	}
}
