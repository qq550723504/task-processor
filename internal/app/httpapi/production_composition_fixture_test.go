package httpapi

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/amazonlisting"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	"task-processor/internal/core/config"
	imageagenthttpapi "task-processor/internal/imageagent/httpapi"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

// buildPersistentProductionCompositionFixture keeps production module wiring
// intact while replacing only persistence and ImageAgent transport edges with
// deterministic local dependencies.
func buildPersistentProductionCompositionFixture(t *testing.T) (httpFeatureComposition, *config.Config) {
	t.Helper()

	cfg := currentE2EConfig(t)
	configureCurrentE2ESheinCookieRedis(t, cfg)
	cfg.Database = nil
	cfg.ListingKit.ImageUpload.Provider = "local"
	cfg.ListingKit.ImageUpload.Local.RootDir = t.TempDir()

	dbPath := filepath.Join(t.TempDir(), "production-composition.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, listingkithttpapi.AutoMigrateListingKitRuntimeSchema(db))
	require.NoError(t, db.AutoMigrate(&amazonlisting.Task{}))
	repositories, err := listingkithttpapi.NewPersistentRepositories(db)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	var closeOnce sync.Once
	var closeErr error
	closeDB := func() error {
		closeOnce.Do(func() { closeErr = sqlDB.Close() })
		return closeErr
	}

	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{cfg: cfg},
		features: &featureRuntimeState{
			productSnapshotReader: stubCompositionProductSnapshotReader{},
			listingKitSupport: &listingKitSupport{
				approvedAssetReader: repositories.Core.ApprovedAsset,
			},
		},
	}
	t.Cleanup(func() {
		closeCurrentE2EClosers(t, deps.shared.closers)
		require.NoError(t, closeDB())
	})

	builder := newHTTPFeatureCompositionBuilder()
	builder.buildListingRepos = func(*config.DatabaseConfig, *logrus.Logger) (listingkithttpapi.BuildServiceRepositories, func() error, error) {
		return repositories, closeDB, nil
	}
	builder.buildAmazonRepo = func(*config.DatabaseConfig, *logrus.Logger) (amazonlisting.Repository, func() error, error) {
		return amazonlistingstore.NewTaskRepository(db), nil, nil
	}
	builder.buildImageAgent = func(*config.Config, *logrus.Logger) (*imageagenthttpapi.BuildResult, error) {
		return nil, nil
	}

	composition, err := builder.build(currentE2ELogger(), deps)
	require.NoError(t, err)
	require.NotNil(t, composition.amazonListingModule)
	require.NotNil(t, composition.listingKitModule)
	return composition, cfg
}
