package currentapplication

import "testing"

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
