package httpapi

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authz"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/storecenter"
)

func TestIssue36ListingRoutesUseExistingSharedStoreBoundary(t *testing.T) {
	fixture := newTitleFixture(t)
	require.NoError(t, assetstore.AutoMigrate(fixture.db))
	require.NoError(t, storecenter.AutoMigrateStoreRepository(fixture.db))
	schema, err := os.ReadFile("../listingrecordstore/schema.sql")
	require.NoError(t, err)
	require.NoError(t, fixture.db.Exec(string(schema)).Error)
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	routes, reader, err := buildLocalTrialListingRoutes(fixture.db, authorizer)
	require.NoError(t, err)
	require.NotNil(t, reader)
	want := map[currentApplicationRoute]struct{}{
		{Method: http.MethodPost, Path: sheinRecordPath}:    {},
		{Method: http.MethodGet, Path: sheinRecordPath}:     {},
		{Method: http.MethodGet, Path: sheinDiagnosticPath}: {},
	}
	require.Len(t, routes, len(want))
	for _, route := range routes {
		key := currentApplicationRoute{Method: route.Method, Path: route.Path}
		_, ok := want[key]
		require.True(t, ok, "unexpected Listing route: %s %s", route.Method, route.Path)
		delete(want, key)
		require.Equal(t, "listing-record", route.Module)
		require.Greater(t, route.RequestTimeout.Nanoseconds(), int64(0))
	}
	require.Empty(t, want)
}
