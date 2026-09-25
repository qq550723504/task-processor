//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	"task-processor/internal/listingsubscription"
)

func TestPostgresIsolatedCatalogOwnerReadbackAndNoEntitlementSeed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("subscription_trial"), tcpostgres.WithUsername("subscription_trial"), tcpostgres.WithPassword("subscription_trial"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	require.NoError(t, commercialstore.AutoMigrate(db))

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	portNumber, err := strconv.Atoi(port.Port())
	require.NoError(t, err)
	configData, err := json.Marshal(databaseManifest{Host: host, Port: portNumber, User: "subscription_trial", Password: "subscription_trial", Database: "subscription_trial", MaxConnections: 5, MaxIdleConnections: 1})
	require.NoError(t, err)
	configPath := filepath.Join(t.TempDir(), "private-db.json")
	require.NoError(t, os.WriteFile(configPath, configData, 0600))
	require.Error(t, run(configPath, "wrong_database"))
	require.NoError(t, run(configPath, "subscription_trial"))
	owner, err := listingsubscription.NewRuntimeService(listingsubscription.NewGormRepository(db))
	require.NoError(t, err)
	snapshot, err := owner.ResolvePurchasablePlan(ctx, trialPlanCode)
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.Fingerprint)
	commercial, err := commercialstore.New(db)
	require.NoError(t, err)
	offer, err := commercial.ReadOffer(ctx, trialOfferID)
	require.NoError(t, err)
	require.Equal(t, snapshot.PlanCode, offer.PlanCode)
	require.Equal(t, "isolated-trial-v1", offer.PricingVersion)

	for _, table := range []string{"saas_tenant_subscriptions", "saas_tenant_entitlements", "commercial_quotes", "commercial_orders"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Zero(t, count, table)
	}
	require.Error(t, run(configPath, "subscription_trial"))
}
