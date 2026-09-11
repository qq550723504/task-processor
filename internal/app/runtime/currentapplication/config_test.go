package currentapplication

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func TestDatabasePasswordSeparatorsRejectedBeforeRuntimeDependencies(t *testing.T) {
	for _, role := range []string{"sourceAccountDatabase", "commercialDatabase"} {
		for _, tc := range []struct{ name, password string }{
			{"vertical_tab", "synthetic\vY"},
			{"form_feed", "synthetic\fY"},
			{"space", "synthetic Y"},
			{"tab", "synthetic\tY"},
			{"cr", "synthetic\rY"},
			{"lf", "synthetic\nY"},
			{"nul", "synthetic\x00Y"},
			{"quote", "synthetic'Y"},
			{"backslash", "synthetic\\Y"},
			{"host_override_vt", "synthetic\vhost=198.51.100.7"},
			{"host_override_ff", "synthetic\fhost=198.51.100.7"},
		} {
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				cfg := runtimeTestConfig()
				if role == "sourceAccountDatabase" {
					cfg.SourceAccountDatabase.Password = tc.password
				} else {
					cfg.CommercialDatabase.Password = tc.password
				}
				manifest, err := json.Marshal(cfg)
				if err != nil {
					t.Fatal("could not marshal synthetic manifest")
				}
				_, loadErr := LoadConfig(writeManifest(t, string(manifest)))
				wantError := role + " credentials and database name are required and must be bounded"
				if loadErr == nil || loadErr.Error() != wantError {
					t.Error("LoadConfig must reject the password with a credential-free validation error")
				}

				var logs bytes.Buffer
				logger := logrus.New()
				logger.SetOutput(&logs)
				preflights, opens := 0, 0
				open := func(context.Context, DatabaseConfig) (*gorm.DB, error) {
					opens++
					return &gorm.DB{}, nil
				}
				runErr := Run(context.Background(), cfg, logger, Dependencies{
					IdentityPreflight: func(context.Context, IdentityConfig) error { preflights++; return nil },
					OpenSourceAccount: open,
					OpenCommercial:    open,
					CloseDatabase:     func(*gorm.DB) error { return nil },
				})
				if runErr == nil || runErr.Error() != wantError {
					t.Error("Run must independently reject the password with a credential-free validation error")
				}
				if preflights != 0 || opens != 0 || logs.Len() != 0 {
					t.Errorf("rejected config performed side effects: preflights=%d opens=%d logBytes=%d", preflights, opens, logs.Len())
				}
			})
		}
	}
}

func TestLoadConfigAcceptsBoundedPrivateManifest(t *testing.T) {
	path := writeManifest(t, validManifest())

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.ListenAddress() != "127.0.0.1:18443" {
		t.Fatalf("ListenAddress() = %q", cfg.ListenAddress())
	}
	if cfg.SourceAccountDatabase.User != "source_account_runtime" {
		t.Fatalf("source account user = %q", cfg.SourceAccountDatabase.User)
	}
	if cfg.CommercialDatabase.User != "commercial_reader" {
		t.Fatalf("commercial user = %q", cfg.CommercialDatabase.User)
	}
	core := cfg.CoreConfig()
	if core == nil || !core.Workbench.Enabled || core.ListingKit.Zitadel.ProjectID != "listingkit-project" {
		t.Fatalf("CoreConfig() = %#v", core)
	}
}

func TestLoadConfigRejectsRelativePathAndNonJSONInput(t *testing.T) {
	if _, err := LoadConfig("manifest.json"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative LoadConfig() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte("schemaVersion: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("non-JSON manifest was accepted")
	}
}

func TestLoadConfigRejectsUnknownDuplicateAndTrailingFields(t *testing.T) {
	tests := map[string]string{
		"unknown":   strings.Replace(validManifest(), `"schemaVersion": 1,`, `"schemaVersion": 1, "bootstrap": true,`, 1),
		"duplicate": strings.Replace(validManifest(), `"port": 18443`, `"port": 18443, "port": 18444`, 1),
		"trailing":  validManifest() + `{}`,
	}
	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadConfig(writeManifest(t, manifest)); err == nil {
				t.Fatalf("invalid %s manifest was accepted", name)
			}
		})
	}
}

func TestLoadConfigRejectsNonLoopbackOrSharedDatabaseRole(t *testing.T) {
	tests := map[string]string{
		"listener":      strings.Replace(validManifest(), `"host": "127.0.0.1", "port": 18443`, `"host": "0.0.0.0", "port": 18443`, 1),
		"issuer":        strings.Replace(validManifest(), `"issuerURL": "http://localhost:18080"`, `"issuerURL": "https://identity.example"`, 1),
		"authorization": strings.Replace(validManifest(), `"authorizationAPIURL": "http://localhost:18080"`, `"authorizationAPIURL": "http://127.0.0.1:18081"`, 1),
		"database_host": strings.Replace(validManifest(), `"host": "127.0.0.1", "port": 15432, "user": "source_account_runtime"`, `"host": "postgres", "port": 15432, "user": "source_account_runtime"`, 1),
		"shared_role":   strings.Replace(validManifest(), `"user": "commercial_reader"`, `"user": "source_account_runtime"`, 1),
		"source_admin":  strings.Replace(validManifest(), `"user": "source_account_runtime"`, `"user": "postgres"`, 1),
		"reader_admin":  strings.Replace(validManifest(), `"user": "commercial_reader"`, `"user": "postgres"`, 1),
		"dsn_password":  strings.Replace(validManifest(), `"password": "source-secret"`, `"password": "x host=198.51.100.1"`, 1),
		"dsn_database":  strings.Replace(validManifest(), `"database": "task_processor"`, `"database": "task_processor sslmode=require"`, 1),
	}
	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadConfig(writeManifest(t, manifest)); err == nil {
				t.Fatalf("invalid %s manifest was accepted", name)
			}
		})
	}
}

func TestLoadConfigDoesNotReadEnvironmentOverrides(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_DATABASE_USER", "environment-admin")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_CLIENT_SECRET", "environment-secret")

	cfg, err := LoadConfig(writeManifest(t, validManifest()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceAccountDatabase.User != "source_account_runtime" || cfg.Identity.ClientSecret != "runtime-secret" {
		t.Fatalf("environment changed manifest: %#v", cfg)
	}
}

func writeManifest(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "current-application.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validManifest() string {
	return `{
  "schemaVersion": 1,
  "listen": {"host": "127.0.0.1", "port": 18443},
  "identity": {
    "issuerURL": "http://localhost:18080",
    "authorizationAPIURL": "http://localhost:18080",
    "clientID": "runtime-client",
    "clientSecret": "runtime-secret",
    "projectID": "listingkit-project"
  },
  "sourceAccountDatabase": {
    "host": "127.0.0.1", "port": 15432, "user": "source_account_runtime",
    "password": "source-secret", "database": "task_processor", "maxConnections": 4
  },
  "commercialDatabase": {
    "host": "127.0.0.1", "port": 15432, "user": "commercial_reader",
    "password": "commercial-secret", "database": "task_processor", "maxConnections": 4
  }
}`
}
