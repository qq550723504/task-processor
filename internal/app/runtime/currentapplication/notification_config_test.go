package currentapplication

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotificationAndSupplyAssetRequireSeparateDatabaseOwners(t *testing.T) {
	cfg := supplyRuntimeConfig(t)
	db := cfg.SupplyChain.AssetDatabase
	db.User = "notification_center_runtime"
	db.Database = "notification_center"
	cfg.NotificationCenterDatabase = &db
	require.NoError(t, cfg.validate())
	cfg.NotificationCenterDatabase.Database = cfg.SupplyChain.AssetDatabase.Database
	require.ErrorContains(t, cfg.validate(), "notification center requires a dedicated database")
	require.ErrorContains(t, cfg.validateSupplyChain(), "supply assets require their independently owned database")
}

func TestNotificationDatabaseRequiresIndependentBoundedRuntimeRole(t *testing.T) {
	for _, kind := range []string{"valid", "shared", "owner-role", "unbounded"} {
		t.Run(kind, func(t *testing.T) {
			cfg := runtimeTestConfig()
			db := cfg.SourceAccountDatabase
			db.User = "notification_center_runtime"
			db.Database = "notification_center"
			db.MaxConnections = 4
			switch kind {
			case "shared":
				db.Database = cfg.SourceAccountDatabase.Database
			case "owner-role":
				db.User = "postgres"
			case "unbounded":
				db.MaxConnections = 5
			}
			cfg.NotificationCenterDatabase = &db
			err := cfg.validate()
			if (err == nil) != (kind == "valid") {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}
