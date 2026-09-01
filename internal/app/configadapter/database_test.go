package configadapter

import (
	"reflect"
	"testing"
	"time"

	coreconfig "task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

func TestDatabaseConfigPreservesRuntimeFields(t *testing.T) {
	t.Parallel()

	in := &coreconfig.DatabaseConfig{
		Host:                  "db",
		Port:                  5432,
		User:                  "worker",
		Password:              "secret",
		Database:              "tasks",
		MaxConnections:        12,
		MaxIdleConnections:    4,
		ConnectionMaxLifetime: 3 * time.Minute,
	}

	got := Database(in)
	want := &platformdatabase.Config{
		Host:                  "db",
		Port:                  5432,
		User:                  "worker",
		Password:              "secret",
		Database:              "tasks",
		MaxConnections:        12,
		MaxIdleConnections:    4,
		ConnectionMaxLifetime: 3 * time.Minute,
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
}

func TestDatabaseConfigPreservesNil(t *testing.T) {
	t.Parallel()

	if got := Database(nil); got != nil {
		t.Fatalf("config = %#v, want nil", got)
	}
}
