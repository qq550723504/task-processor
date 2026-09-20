package currentapplication

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	coreconfig "task-processor/internal/core/config"
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
	if cfg.CommercialDatabase.User != "commercial_runtime" {
		t.Fatalf("commercial user = %q", cfg.CommercialDatabase.User)
	}
	core := cfg.CoreConfig()
	if core == nil || !core.Workbench.Enabled || core.ListingKit.Zitadel.ProjectID != "listingkit-project" {
		t.Fatalf("CoreConfig() = %#v", core)
	}
}

func TestLoadConfigAcceptsHTTPSLoopbackIdentityOrigin(t *testing.T) {
	manifest := strings.ReplaceAll(validManifest(), "http://localhost:18080", "https://localhost:18443")

	cfg, err := LoadConfig(writeManifest(t, manifest))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Identity.IssuerURL != "https://localhost:18443" || cfg.Identity.AuthorizationAPIURL != "https://localhost:18443" {
		t.Fatalf("HTTPS identity origin = %#v", cfg.Identity)
	}
}

func TestLoadConfigAcceptsExplicitlyDisabledReferrals(t *testing.T) {
	manifest := strings.Replace(validManifest(), `"schemaVersion": 1,`, `"schemaVersion": 1, "referrals": {"enabled": false},`, 1)
	if _, err := LoadConfig(writeManifest(t, manifest)); err != nil {
		t.Fatalf("explicitly disabled referrals must require no credentials or database: %v", err)
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
		"shared_role":   strings.Replace(validManifest(), `"user": "commercial_runtime"`, `"user": "source_account_runtime"`, 1),
		"source_admin":  strings.Replace(validManifest(), `"user": "source_account_runtime"`, `"user": "postgres"`, 1),
		"reader_admin":  strings.Replace(validManifest(), `"user": "commercial_runtime"`, `"user": "postgres"`, 1),
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

func referralRuntimeConfig(t *testing.T) *Config {
	t.Helper()
	cfg := runtimeTestConfig()
	secret := func(name string) string {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(hex.EncodeToString(b)), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cfg.Referrals = ReferralsConfig{ReferralsConfig: coreconfig.ReferralsConfig{Enabled: true, Issuer: cfg.Identity.IssuerURL, InstanceID: "fixture", SignupOrganizationID: "signup", ProviderOrigin: "https://provider.example", OfficialLoginOrigin: "https://login.example", PublicAppOrigin: "https://app.example", CredentialFile: secret("provider"), ServiceCredentialFile: secret("service"), LookupKeyFile: secret("lookup"), KeyID: "k1", ProofKeyFiles: map[string]string{"k1": secret("proof")}, EncryptionKeyFiles: map[string]string{"k1": secret("encryption")}}, Database: DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "referral_runtime", Password: "fixture-referral", Database: "referrals", MaxConnections: 2}}
	return cfg
}

func TestReferralManifestRejectsPublicRead(t *testing.T) {
	cfg := referralRuntimeConfig(t)
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, string(data))
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("icacls", path, "/grant", "*S-1-1-0:(R)").CombinedOutput(); err != nil {
			t.Fatalf("synthetic manifest ACL: %v %s", err, out)
		}
	} else if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("public-readable referral manifest accepted")
	}
}

func TestReferralPrivateFilesRejectPublicRead(t *testing.T) {
	cfg := referralRuntimeConfig(t)
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("icacls", cfg.Referrals.CredentialFile, "/grant", "*S-1-1-0:(R)").CombinedOutput(); err != nil {
			t.Fatalf("synthetic fixture ACL: %v %s", err, out)
		}
	} else if err := os.Chmod(cfg.Referrals.CredentialFile, 0644); err != nil {
		t.Fatal(err)
	}
	prepared, err := cfg.Referrals.Prepare(context.Background())
	if prepared != nil {
		prepared.HTTPClient.CloseIdleConnections()
	}
	if err == nil {
		t.Fatal("public-readable provider credential accepted")
	}
}

func TestReferralPrivateConfigurationRejectsIncompleteAndReusedSecrets(t *testing.T) {
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Referrals.CredentialFile = "" },
		func(c *Config) { c.Referrals.ProofKeyFiles["k1"] = c.Referrals.EncryptionKeyFiles["k1"] },
		func(c *Config) { c.Referrals.LookupKeyFile = c.Referrals.ServiceCredentialFile },
		func(c *Config) { c.Referrals.ProviderOrigin = "http://provider.example" },
		func(c *Config) { c.Referrals.ProviderOrigin = "https://provider.example/path" },
		func(c *Config) { c.Referrals.KeyID = "missing" },
	} {
		cfg := referralRuntimeConfig(t)
		mutate(cfg)
		if _, err := cfg.Referrals.Prepare(context.Background()); err == nil {
			t.Fatal("invalid referral configuration accepted")
		}
	}
	cfg := referralRuntimeConfig(t)
	prepared, err := cfg.Referrals.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.HTTPClient.CloseIdleConnections()
	if len(prepared.Lookup) != 32 || len(prepared.Proof["k1"]) != 32 {
		t.Fatal("private keys not loaded")
	}
	core := cfg.CoreConfig()
	core.Referrals.Prepared = prepared
	data, err := json.Marshal(core.Referrals)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), prepared.ServiceCredential) || strings.Contains(string(data), prepared.ProviderToken) {
		t.Fatal("manifest serializes secrets")
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
    "host": "127.0.0.1", "port": 15432, "user": "commercial_runtime",
    "password": "commercial-secret", "database": "task_processor", "maxConnections": 4
  }
}`
}
