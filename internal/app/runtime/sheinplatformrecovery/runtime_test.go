package sheinplatformrecovery

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
)

func TestRunDefaultsToDryRun(t *testing.T) {
	t.Parallel()

	var request listingadmin.PlatformRecoveryRequest
	var output bytes.Buffer
	err := runWithDependencies(context.Background(), Options{StoreID: 986, ExpectedCount: 200}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return &config.Config{Database: &config.DatabaseConfig{}}, nil },
		OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { return &gorm.DB{}, nil },
		CloseDB:    func(*gorm.DB) error { return nil },
		Recover: func(_ context.Context, _ *gorm.DB, got listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error) {
			request = got
			return listingadmin.PlatformRecoveryReport{DryRun: got.Execute == false}, nil
		},
		Output: &output,
	})
	if err != nil {
		t.Fatalf("runWithDependencies() error = %v", err)
	}
	if request != (listingadmin.PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 200}) {
		t.Fatalf("request = %+v, want default dry-run request", request)
	}
	if output.Len() == 0 {
		t.Fatal("expected recovery report output")
	}
}

func TestRunRejectsAnyStoreExcept986BeforeOpeningDatabase(t *testing.T) {
	t.Parallel()

	loaded := false
	opened := false
	err := runWithDependencies(context.Background(), Options{StoreID: 985, ExpectedCount: 200}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			loaded = true
			return nil, errors.New("load should not run")
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			opened = true
			return nil, errors.New("open should not run")
		},
	})
	if err == nil {
		t.Fatal("runWithDependencies() error = nil, want store-scope rejection")
	}
	if loaded {
		t.Fatal("configuration loaded for invalid store scope")
	}
	if opened {
		t.Fatal("database opened for invalid store scope")
	}
}

func TestRunPassesExplicitExpectedCountAndExecuteFlag(t *testing.T) {
	t.Parallel()

	var request listingadmin.PlatformRecoveryRequest
	err := runWithDependencies(context.Background(), Options{StoreID: 986, ExpectedCount: 200, Execute: true}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return &config.Config{Database: &config.DatabaseConfig{}}, nil },
		OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { return &gorm.DB{}, nil },
		CloseDB:    func(*gorm.DB) error { return nil },
		Recover: func(_ context.Context, _ *gorm.DB, got listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error) {
			request = got
			return listingadmin.PlatformRecoveryReport{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDependencies() error = %v", err)
	}
	want := listingadmin.PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 200, Execute: true}
	if request != want {
		t.Fatalf("request = %+v, want %+v", request, want)
	}
}
