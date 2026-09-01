package httpapi

import (
	"reflect"
	"testing"
	"time"

	coreconfig "task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

func TestPlatformDatabaseConfigPreservesLegacyRuntimeFields(t *testing.T) {
	t.Parallel()

	in := &coreconfig.DatabaseConfig{
		Host:                  "legacy-db",
		Port:                  5433,
		User:                  "legacy-worker",
		Password:              "legacy-secret",
		Database:              "legacy-tasks",
		MaxConnections:        15,
		MaxIdleConnections:    6,
		ConnectionMaxLifetime: 7 * time.Minute,
	}
	want := &platformdatabase.Config{
		Host:                  "legacy-db",
		Port:                  5433,
		User:                  "legacy-worker",
		Password:              "legacy-secret",
		Database:              "legacy-tasks",
		MaxConnections:        15,
		MaxIdleConnections:    6,
		ConnectionMaxLifetime: 7 * time.Minute,
	}

	if got := platformDatabaseConfig(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestPlatformDatabaseConfigPreservesLegacyNil(t *testing.T) {
	t.Parallel()

	if got := platformDatabaseConfig(nil); got != nil {
		t.Fatalf("config = %#v, want nil", got)
	}
}
