package local

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeResourcesInitializesRepositoriesAndClosesIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	resources := NewRuntimeResources(db, nil)
	if resources == nil {
		t.Fatal("runtime resources is nil")
	}
	if resources.StoreRepository() == nil {
		t.Fatal("store repository is nil")
	}
	if resources.ImportTaskRepository() == nil {
		t.Fatal("import task repository is nil")
	}
	if resources.RawJSONDataRepository() == nil {
		t.Fatal("raw JSON data repository is nil")
	}

	if err := resources.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := resources.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestValidateLocalListingRuntimeReportsResourceCapabilities(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	resources := NewRuntimeResources(db, nil)

	report, err := ValidateLocalListingRuntime(resources)
	if err == nil {
		t.Fatal("health validation unexpectedly succeeded without Redis")
	}
	if !report.DB {
		t.Fatal("database capability is false")
	}
	if report.Redis || report.DailyQuota || report.Ready {
		t.Fatalf("Redis-dependent capability report = %#v", report)
	}
}

func TestNewLocalTaskRPCProviderAcceptsDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if provider := NewLocalTaskRPCProvider(db); provider == nil {
		t.Fatal("task RPC provider is nil")
	}
	if provider := NewLocalTaskRPCProvider(nil); provider != nil {
		t.Fatalf("nil database provider = %#v", provider)
	}
}
