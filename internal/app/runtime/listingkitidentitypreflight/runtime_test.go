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
	"task-processor/internal/listingkit/ownerreconcile"
	"task-processor/internal/listingkit/userdirectory"
	"task-processor/internal/tenantbridge"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDefaultRuntimeDependenciesUseNonCreatingDatabaseFactory(t *testing.T) {
	t.Parallel()

	want := reflect.ValueOf(database.NewDatabaseFromConfigWithoutCreate).Pointer()
	for name, factory := range map[string]func(*config.DatabaseConfig) (*gorm.DB, error){
		"owner":    defaultRuntimeDependencies().OpenDB,
		"metadata": defaultRuntimeDependencies().OpenMetadataDB,
	} {
		if got := reflect.ValueOf(factory).Pointer(); got != want {
			t.Fatalf("%s database factory is not the non-creating factory", name)
		}
	}
}

func TestDefaultRuntimeDependenciesUseNonValidatingConfigLoader(t *testing.T) {
	t.Parallel()

	want := reflect.ValueOf(config.LoadConfigFromFileWithoutValidation).Pointer()
	if got := reflect.ValueOf(defaultRuntimeDependencies().LoadConfig).Pointer(); got != want {
		t.Fatal("identity preflight must use the non-validating config loader because its Job mounts only preflight credentials")
	}
}

func TestDefaultRuntimeDependenciesUseMetadataBridgeResolver(t *testing.T) {
	t.Parallel()

	resolver := defaultRuntimeDependencies().NewLegacyTenantResolver(&gorm.DB{})
	if _, ok := resolver.(*tenantbridge.MetadataResolver); !ok {
		t.Fatalf("legacy tenant resolver = %T, want *tenantbridge.MetadataResolver", resolver)
	}
}

func TestLegacyTenantMetadataTableExistsUsesReadOnlyRegclassProbe(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open GORM database: %v", err)
	}
	mock.ExpectQuery("select to_regclass($1) as name").
		WithArgs("projections.org_metadata2").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("projections.org_metadata2"))

	exists, err := legacyTenantMetadataTableExists(db)
	if err != nil {
		t.Fatalf("metadata table probe: %v", err)
	}
	if !exists {
		t.Fatal("metadata table probe = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
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
		NewPreflight: func(identitypreflight.OwnerRepository, userdirectory.Directory, identitypreflight.LegacyTenantOrganizationResolver, io.Writer) preflightRunner {
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

func TestRunBlocksOnUnresolvedOwnerReconciliationBeforeDirectory(t *testing.T) {
	var output bytes.Buffer
	db := &gorm.DB{}
	metadataDB := &gorm.DB{}
	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return configuredRuntimeConfig("https://issuer.example", "directory-token"), nil
		},
		OpenDB:      func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB:     func(*gorm.DB) error { return nil },
		DatabaseSQL: func(*gorm.DB) (*sql.DB, error) { return &sql.DB{}, nil },
		OpenMetadataDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			if cfg.Database == "zitadel_auth" {
				return nil, errors.New("candidate is absent")
			}
			return metadataDB, nil
		},
		MetadataTableExists: func(*gorm.DB) (bool, error) { return true, nil },
		RunOwnerReconciliation: func(context.Context, *gorm.DB, *gorm.DB) (ownerreconcile.Report, error) {
			return ownerreconcile.NewReport("config.yaml", "db", []ownerreconcile.Finding{{Rows: 3, Reason: "unmapped_candidate"}}, 0), nil
		},
		NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
			t.Fatal("directory must not run while owner reconciliation is unresolved")
			return nil, nil
		},
		Output: &output,
	})
	if err == nil || !strings.Contains(err.Error(), "owner reconciliation") {
		t.Fatalf("run error = %v, want owner reconciliation blocker", err)
	}
	if !strings.Contains(output.String(), "status=blocked owner_reconciliation") {
		t.Fatalf("output = %q, want redacted owner reconciliation blocker", output.String())
	}
}

func TestRunClosesDatabaseAndWritesOnlySafeSuccessSummary(t *testing.T) {
	const (
		issuer = "https://issuer.private.example"
		token  = "directory-token-private"
	)
	db := &gorm.DB{}
	metadataDB := &gorm.DB{}
	sqlDB := &sql.DB{}
	var output bytes.Buffer
	var closed []*gorm.DB

	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return configuredRuntimeConfig(issuer, token), nil
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB: func(got *gorm.DB) error {
			closed = append(closed, got)
			return nil
		},
		OpenMetadataDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			if cfg.Database == "zitadel_auth" {
				return nil, errors.New("candidate is absent")
			}
			if cfg.Database != "zitadel" {
				t.Fatalf("metadata database candidate = %q", cfg.Database)
			}
			return metadataDB, nil
		},
		MetadataTableExists: func(got *gorm.DB) (bool, error) {
			if got != metadataDB {
				t.Fatalf("metadata probe database = %p, want %p", got, metadataDB)
			}
			return true, nil
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
		NewLegacyTenantResolver: func(got *gorm.DB) identitypreflight.LegacyTenantOrganizationResolver {
			if got != metadataDB || got == db {
				t.Fatalf("legacy tenant resolver received database = %p, want metadata %p and not owner %p", got, metadataDB, db)
			}
			return stubLegacyTenantResolver{}
		},
		NewPreflight: func(
			_ identitypreflight.OwnerRepository,
			_ userdirectory.Directory,
			resolver identitypreflight.LegacyTenantOrganizationResolver,
			_ io.Writer,
		) preflightRunner {
			if _, ok := resolver.(stubLegacyTenantResolver); !ok {
				t.Fatalf("preflight resolver = %T, want stubLegacyTenantResolver", resolver)
			}
			return runnerFunc(func(context.Context) error { return nil })
		},
		Output: &output,
	})
	if err != nil {
		t.Fatalf("run error = %v", err)
	}
	if want := []*gorm.DB{metadataDB, db}; !reflect.DeepEqual(closed, want) {
		t.Fatalf("closed databases = %#v, want metadata then owner %#v", closed, want)
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
	metadataDB := &gorm.DB{}
	var closed []*gorm.DB

	err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return configuredRuntimeConfig("https://issuer.example", "directory-token"), nil
		},
		OpenDB:  func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB: func(got *gorm.DB) error { closed = append(closed, got); return nil },
		OpenMetadataDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			if cfg.Database == "zitadel" {
				return nil, errors.New("candidate is absent")
			}
			return metadataDB, nil
		},
		MetadataTableExists: func(got *gorm.DB) (bool, error) {
			return got == metadataDB, nil
		},
		NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
			return stubDirectory{}, nil
		},
		DatabaseSQL:        func(*gorm.DB) (*sql.DB, error) { return &sql.DB{}, nil },
		NewOwnerRepository: func(*sql.DB) identitypreflight.OwnerRepository { return stubOwners{} },
		NewPreflight: func(identitypreflight.OwnerRepository, userdirectory.Directory, identitypreflight.LegacyTenantOrganizationResolver, io.Writer) preflightRunner {
			return runnerFunc(func(context.Context) error { return coreFailure })
		},
		Output: &bytes.Buffer{},
	})
	if !errors.Is(err, coreFailure) {
		t.Fatalf("run error = %v, want core preflight failure", err)
	}
	if want := []*gorm.DB{metadataDB, db}; !reflect.DeepEqual(closed, want) {
		t.Fatalf("closed databases after core failure = %#v, want %#v", closed, want)
	}
}

func TestRunFailsClosedWhenMetadataDatabaseCandidateIsMissingOrAmbiguous(t *testing.T) {
	for _, test := range []struct {
		name   string
		exists map[string]bool
	}{
		{name: "missing", exists: map[string]bool{}},
		{name: "ambiguous", exists: map[string]bool{"zitadel_auth": true, "zitadel": true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ownerDB := &gorm.DB{}
			candidateDBs := map[string]*gorm.DB{
				"zitadel_auth": {},
				"zitadel":      {},
			}
			databaseNames := make(map[*gorm.DB]string, len(candidateDBs))
			for name, candidateDB := range candidateDBs {
				databaseNames[candidateDB] = name
			}
			var closed []*gorm.DB
			var output bytes.Buffer

			err := runWithDependencies(context.Background(), Options{}, runtimeDependencies{
				LoadConfig: func(string) (*config.Config, error) {
					return configuredRuntimeConfig("https://issuer.example", "directory-token"), nil
				},
				OpenDB:      func(*config.DatabaseConfig) (*gorm.DB, error) { return ownerDB, nil },
				DatabaseSQL: func(*gorm.DB) (*sql.DB, error) { return &sql.DB{}, nil },
				CloseDB: func(got *gorm.DB) error {
					closed = append(closed, got)
					return nil
				},
				OpenMetadataDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
					return candidateDBs[cfg.Database], nil
				},
				MetadataTableExists: func(got *gorm.DB) (bool, error) {
					if got == ownerDB {
						t.Fatal("owner database must not be probed for ZITADEL metadata")
					}
					return test.exists[databaseNames[got]], nil
				},
				NewDirectory: func(userdirectory.ClientConfig) (userdirectory.Directory, error) {
					t.Fatal("directory must not be configured without exactly one metadata database")
					return nil, nil
				},
				NewPreflight: func(identitypreflight.OwnerRepository, userdirectory.Directory, identitypreflight.LegacyTenantOrganizationResolver, io.Writer) preflightRunner {
					t.Fatal("preflight must not run without exactly one metadata database")
					return nil
				},
				Output: &output,
			})
			if err == nil || !strings.Contains(err.Error(), "metadata database") {
				t.Fatalf("run error = %v, want sanitized metadata database failure", err)
			}
			if output.Len() != 0 {
				t.Fatalf("output = %q, want no partial output", output.String())
			}
			if len(closed) != 3 {
				t.Fatalf("closed database count = %d, want two candidates plus owner", len(closed))
			}
		})
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

func (stubDirectory) ListByTenant(context.Context, string) ([]userdirectory.User, error) {
	return nil, nil
}

type stubOwners struct{}

func (stubOwners) List(context.Context) ([]identitypreflight.PersistedOwner, error) { return nil, nil }

type stubLegacyTenantResolver struct{}

func (stubLegacyTenantResolver) ResolveOrganizationID(context.Context, int64) (string, bool, error) {
	return "", false, nil
}

