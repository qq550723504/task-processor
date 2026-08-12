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

type failingRecoveryOutput struct{}

func (failingRecoveryOutput) Write([]byte) (int, error) { return 0, errors.New("output failed") }

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

func TestRunPassesExplicitExpectedCountExecuteFlagAndConfirmation(t *testing.T) {
	t.Parallel()

	var request listingadmin.PlatformRecoveryRequest
	err := runWithDependencies(context.Background(), Options{StoreID: 986, ExpectedCount: 200, Execute: true, ConfirmFingerprint: "sha256:confirmed"}, runtimeDependencies{
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
	want := listingadmin.PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 200, Execute: true, ConfirmFingerprint: "sha256:confirmed"}
	if request != want {
		t.Fatalf("request = %+v, want %+v", request, want)
	}
}

func TestRunRejectsExecuteWithoutConfirmationBeforeConfiguration(t *testing.T) {
	t.Parallel()

	loaded := false
	err := runWithDependencies(context.Background(), Options{StoreID: 986, ExpectedCount: 1, Execute: true}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			loaded = true
			return nil, errors.New("load should not run")
		},
	})
	if err == nil {
		t.Fatal("runWithDependencies() error = nil, want missing confirmation rejection")
	}
	if loaded {
		t.Fatal("configuration loaded without execution confirmation")
	}
}

func TestRunUsesReadOnlyDatabaseForDryRun(t *testing.T) {
	t.Parallel()

	readDB := &gorm.DB{}
	writeDB := &gorm.DB{}
	readOpens := 0
	writeOpens := 0
	err := runWithDependencies(context.Background(), Options{StoreID: 986, ExpectedCount: 1}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return &config.Config{Database: &config.DatabaseConfig{}}, nil },
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			readOpens++
			return readDB, nil
		},
		OpenWritableDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			writeOpens++
			return writeDB, nil
		},
		CloseDB: func(*gorm.DB) error { return nil },
		Recover: func(_ context.Context, db *gorm.DB, request listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error) {
			if db != readDB || request.Execute {
				t.Fatalf("recovery received db=%p request=%+v, want read-only dry-run", db, request)
			}
			return listingadmin.PlatformRecoveryReport{DryRun: true}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDependencies() error = %v", err)
	}
	if readOpens != 1 || writeOpens != 0 {
		t.Fatalf("database opens = read %d, write %d; want read 1, write 0", readOpens, writeOpens)
	}
}

func TestRunUsesWritableDatabaseOnlyForExecute(t *testing.T) {
	t.Parallel()

	readDB := &gorm.DB{}
	writeDB := &gorm.DB{}
	readOpens := 0
	writeOpens := 0
	err := runWithDependencies(context.Background(), Options{StoreID: 986, ExpectedCount: 1, Execute: true, ConfirmFingerprint: "sha256:confirmed"}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return &config.Config{Database: &config.DatabaseConfig{}}, nil },
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			readOpens++
			return readDB, nil
		},
		OpenWritableDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			writeOpens++
			return writeDB, nil
		},
		CloseDB: func(*gorm.DB) error { return nil },
		Recover: func(_ context.Context, db *gorm.DB, request listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error) {
			if db != writeDB || !request.Execute {
				t.Fatalf("recovery received db=%p request=%+v, want writable execute", db, request)
			}
			return listingadmin.PlatformRecoveryReport{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDependencies() error = %v", err)
	}
	if readOpens != 0 || writeOpens != 1 {
		t.Fatalf("database opens = read %d, write %d; want read 0, write 1", readOpens, writeOpens)
	}
}

func TestRunReturnsRecoveryReportWriteError(t *testing.T) {
	t.Parallel()

	err := runWithDependencies(context.Background(), Options{StoreID: 986, ExpectedCount: 1}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return &config.Config{Database: &config.DatabaseConfig{}}, nil },
		OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { return &gorm.DB{}, nil },
		CloseDB:    func(*gorm.DB) error { return nil },
		Recover: func(context.Context, *gorm.DB, listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error) {
			return listingadmin.PlatformRecoveryReport{DryRun: true}, nil
		},
		Output: failingRecoveryOutput{},
	})
	if err == nil {
		t.Fatal("runWithDependencies() error = nil, want recovery report write error")
	}
}
