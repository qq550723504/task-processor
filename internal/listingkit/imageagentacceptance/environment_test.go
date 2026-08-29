package imageagentacceptance

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnvironmentGuardRejectsNonAcceptanceDatabase(t *testing.T) {
	var opened bool
	guard := NewEnvironmentGuard(EnvironmentProbes{
		Open: func(context.Context, RuntimeConfig) (*gorm.DB, error) {
			opened = true
			return openAcceptanceTestDB(t), nil
		},
		ComposeProject: func(context.Context, RuntimeConfig) (bool, error) { return true, nil },
		DatabaseName:   func(context.Context, *gorm.DB) (string, error) { return "postgres", nil },
		Marker:         func(context.Context, *gorm.DB) (string, error) { return "marker-1", nil },
	})

	_, err := guard.Verify(context.Background(), validRuntimeConfig())
	if err == nil || !strings.Contains(err.Error(), "database name") {
		t.Fatalf("Verify() error = %v, want a database-name rejection", err)
	}
	if !opened {
		t.Fatal("Verify() did not open the database after the Compose identity check")
	}
}

func TestEnvironmentGuardRejectsMissingOrMismatchedMarker(t *testing.T) {
	tests := []struct {
		name   string
		marker string
	}{
		{name: "missing", marker: ""},
		{name: "mismatched", marker: "other-marker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard := NewEnvironmentGuard(EnvironmentProbes{
				Open:           func(context.Context, RuntimeConfig) (*gorm.DB, error) { return openAcceptanceTestDB(t), nil },
				ComposeProject: func(context.Context, RuntimeConfig) (bool, error) { return true, nil },
				DatabaseName:   func(context.Context, *gorm.DB) (string, error) { return DatabaseName, nil },
				Marker:         func(context.Context, *gorm.DB) (string, error) { return tt.marker, nil },
			})

			if _, err := guard.Verify(context.Background(), validRuntimeConfig()); err == nil || !strings.Contains(err.Error(), "environment marker") {
				t.Fatalf("Verify() error = %v, want an environment-marker rejection", err)
			}
		})
	}
}

func TestEnvironmentGuardRejectsOtherComposeProjectBeforeOpeningDatabase(t *testing.T) {
	opened := false
	guard := NewEnvironmentGuard(EnvironmentProbes{
		Open: func(context.Context, RuntimeConfig) (*gorm.DB, error) {
			opened = true
			return openAcceptanceTestDB(t), nil
		},
		ComposeProject: func(context.Context, RuntimeConfig) (bool, error) { return false, nil },
		DatabaseName:   func(context.Context, *gorm.DB) (string, error) { return DatabaseName, nil },
		Marker:         func(context.Context, *gorm.DB) (string, error) { return "marker-1", nil },
	})

	if _, err := guard.Verify(context.Background(), validRuntimeConfig()); err == nil || !strings.Contains(err.Error(), "Compose project") {
		t.Fatalf("Verify() error = %v, want a Compose-project rejection", err)
	}
	if opened {
		t.Fatal("Verify() opened the database after the Compose project identity failed")
	}
}

func TestEnvironmentGuardFailsClosedWhenAProbeIsUnavailable(t *testing.T) {
	guard := NewEnvironmentGuard(EnvironmentProbes{
		Open: func(context.Context, RuntimeConfig) (*gorm.DB, error) { return openAcceptanceTestDB(t), nil },
	})
	if _, err := guard.Verify(context.Background(), validRuntimeConfig()); err == nil {
		t.Fatal("Verify() accepted an environment without complete probes")
	}
}

func validRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		DatabaseDSN:       "postgres://acceptance@localhost/image_agent_acceptance",
		EnvironmentMarker: "marker-1",
		ComposeProject:    "task-processor-image-agent-acceptance",
		IssuerURL:         "https://zitadel.example.test",
		APIClientID:       "api-client",
		APIClientSecret:   "api-secret",
	}
}

func openAcceptanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}
