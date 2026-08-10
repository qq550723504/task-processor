package listingkitidentitypreflight

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingkit/identitypreflight"
	"task-processor/internal/listingkit/userdirectory"

	"gorm.io/gorm"
)

func TestDefaultRuntimeDependenciesUseNonCreatingDatabaseFactory(t *testing.T) {
	t.Parallel()

	got := reflect.ValueOf(defaultRuntimeDependencies().OpenDB).Pointer()
	want := reflect.ValueOf(database.NewDatabaseFromConfigWithoutCreate).Pointer()
	if got != want {
		t.Fatal("identity preflight runtime is not wired to the non-creating database factory")
	}
}

func TestRunLoadConfigFailureDoesNotOpenDatabase(t *testing.T) {
	const sentinel = "postgres://operator:loader-secret@db.example/listingkit"
	errLoad := errors.New("config file is unavailable: " + sentinel)
	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return nil, errLoad },
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			t.Fatal("database must not open when config loading fails")
			return nil, nil
		},
		NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
			t.Fatal("directory must not be configured when config loading fails")
			return nil, nil
		},
	})
	if got, want := err.Error(), "load config failed"; got != want {
		t.Fatalf("run error = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("run error leaked loader secret %q: %q", sentinel, err)
	}
}

func TestRunRejectsMissingDatabaseBeforeDirectoryCalls(t *testing.T) {
	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{}, nil
		},
		NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
			t.Fatal("directory must not be configured without a database")
			return nil, nil
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			t.Fatal("database must not open without database configuration")
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "database is required") {
		t.Fatalf("run error = %v, want missing database failure", err)
	}
}

func TestRunRejectsMissingIssuerOrDirectoryToken(t *testing.T) {
	for _, zitadel := range []config.ListingKitZitadelConfig{
		{TenantDirectoryToken: "directory-token"},
		{IssuerURL: "https://issuer.example"},
	} {
		err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
			LoadConfig: func(string) (*config.Config, error) {
				return &config.Config{Database: &config.DatabaseConfig{}, ListingKit: config.ListingKitConfig{Zitadel: zitadel}}, nil
			},
			OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
				t.Fatal("database must not open with incomplete directory configuration")
				return nil, nil
			},
			NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
				t.Fatal("directory constructor must not receive incomplete configuration")
				return nil, nil
			},
		})
		if err == nil || !strings.Contains(err.Error(), "directory") {
			t.Fatalf("run error = %v, want missing directory configuration failure", err)
		}
	}
}

func TestRunDatabaseOpenFailureStopsBeforeDirectoryOrPreflight(t *testing.T) {
	const databaseSecret = "postgres://operator:private-password@db.internal/missing_listingkit"
	var output bytes.Buffer

	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return configuredRuntimeConfig("https://issuer.example", "directory-token"), nil
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			return nil, errors.New("database does not exist: " + databaseSecret)
		},
		NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
			t.Fatal("directory must not be configured when strict database open fails")
			return nil, nil
		},
		NewPreflight: func(identitypreflight.OwnerRepository, userdirectory.Directory, io.Writer) preflightRunner {
			t.Fatal("preflight must not run when strict database open fails")
			return nil
		},
		Output: &output,
	})
	if got, want := err.Error(), "connect database failed"; got != want {
		t.Fatalf("run error = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), databaseSecret) {
		t.Fatalf("run error leaked database details: %q", err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want no successful preflight summary", output.String())
	}
}

func TestRunClosesDatabaseAndWritesOnlySafeSuccessSummary(t *testing.T) {
	const (
		issuer = "https://issuer.private.example"
		token  = "directory-token-private"
	)
	db := &gorm.DB{}
	sqlDB := &sql.DB{}
	var output bytes.Buffer
	var closed bool

	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return configuredRuntimeConfig(issuer, token), nil
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB: func(got *gorm.DB) error {
			if got != db {
				t.Fatalf("closed database = %p, want %p", got, db)
			}
			closed = true
			return nil
		},
		NewDirectory: func(cfg userdirectory.ClientConfig) (userdirectory.Directory, error) {
			if cfg.IssuerURL != issuer || cfg.Token != token {
				t.Fatalf("directory config = %+v, want configured issuer and token", cfg)
			}
			return stubDirectory{}, nil
		},
		DatabaseSQL: func(got *gorm.DB) (*sql.DB, error) {
			if got != db {
				t.Fatalf("database SQL handle requested for unexpected database")
			}
			return sqlDB, nil
		},
		NewOwnerRepository: func(got *sql.DB) identitypreflight.OwnerRepository {
			if got != sqlDB {
				t.Fatalf("owner repository received unexpected SQL database")
			}
			return stubOwners{}
		},
		NewPreflight: func(identitypreflight.OwnerRepository, userdirectory.Directory, io.Writer) preflightRunner {
			return runnerFunc(func(context.Context) error { return nil })
		},
		Output: &output,
	})
	if err != nil {
		t.Fatalf("run error = %v", err)
	}
	if !closed {
		t.Fatal("database was not closed")
	}
	if got, want := output.String(), "status=ok identity_preflight=passed\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	for _, raw := range []string{issuer, token} {
		if strings.Contains(output.String(), raw) {
			t.Fatalf("success summary leaked %q: %q", raw, output.String())
		}
	}
}

func TestRunReturnsCorePreflightFailureAndClosesDatabase(t *testing.T) {
	coreFailure := &identitypreflight.ErrUnknownOwners{Count: 2}
	db := &gorm.DB{}
	var closed bool

	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return configuredRuntimeConfig("https://issuer.example", "directory-token"), nil },
		OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB:    func(*gorm.DB) error { closed = true; return nil },
		NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
			return stubDirectory{}, nil
		},
		DatabaseSQL:        func(*gorm.DB) (*sql.DB, error) { return &sql.DB{}, nil },
		NewOwnerRepository: func(*sql.DB) identitypreflight.OwnerRepository { return stubOwners{} },
		NewPreflight: func(identitypreflight.OwnerRepository, userdirectory.Directory, io.Writer) preflightRunner {
			return runnerFunc(func(context.Context) error { return coreFailure })
		},
		Output: &bytes.Buffer{},
	})
	if !errors.Is(err, coreFailure) {
		t.Fatalf("run error = %v, want core preflight failure", err)
	}
	if !closed {
		t.Fatal("database was not closed after core preflight failure")
	}
}

func configuredRuntimeConfig(issuer, token string) *config.Config {
	return &config.Config{
		Database: &config.DatabaseConfig{},
		ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{
			IssuerURL:            issuer,
			TenantDirectoryToken: token,
		}},
	}
}

type runnerFunc func(context.Context) error

func (fn runnerFunc) Run(ctx context.Context) error { return fn(ctx) }

type stubDirectory struct{}

func (stubDirectory) ListByTenant(context.Context, string) ([]userdirectory.User, error) { return nil, nil }

type stubOwners struct{}

func (stubOwners) List(context.Context) ([]identitypreflight.PersistedOwner, error) { return nil, nil }
