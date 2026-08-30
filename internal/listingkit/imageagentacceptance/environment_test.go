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

func TestEnvironmentGuardRejectsMismatchedLocalTargetsBeforeDockerOrDatabase(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RuntimeConfig)
	}{
		{name: "compose project", mutate: func(config *RuntimeConfig) { config.ComposeProject = "other-project" }},
		{name: "database host", mutate: func(config *RuntimeConfig) {
			config.DatabaseDSN = "postgres://acceptance@db.example.test:15433/image_agent_acceptance"
		}},
		{name: "database port", mutate: func(config *RuntimeConfig) {
			config.DatabaseDSN = "postgres://acceptance@127.0.0.1:5432/image_agent_acceptance"
		}},
		{name: "database name", mutate: func(config *RuntimeConfig) { config.DatabaseDSN = "postgres://acceptance@127.0.0.1:15433/postgres" }},
		{name: "database query host override", mutate: func(config *RuntimeConfig) {
			config.DatabaseDSN = "postgres://acceptance@127.0.0.1:15433/image_agent_acceptance?sslmode=disable&host=db.example.test"
		}},
		{name: "database query user override", mutate: func(config *RuntimeConfig) {
			config.DatabaseDSN = "postgres://acceptance@127.0.0.1:15433/image_agent_acceptance?sslmode=disable&user=prod"
		}},
		{name: "database unsupported parameter", mutate: func(config *RuntimeConfig) {
			config.DatabaseDSN = "postgres://acceptance@127.0.0.1:15433/image_agent_acceptance?sslmode=disable&application_name=acceptance"
		}},
		{name: "issuer host", mutate: func(config *RuntimeConfig) { config.IssuerURL = "http://zitadel.example.test:8080" }},
		{name: "issuer port", mutate: func(config *RuntimeConfig) { config.IssuerURL = "http://localhost:9090" }},
		{name: "placeholder client", mutate: func(config *RuntimeConfig) { config.APIClientSecret = "pending-provision" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validRuntimeConfig()
			tt.mutate(&config)
			probed := false
			guard := NewEnvironmentGuard(EnvironmentProbes{
				ComposeProject: func(context.Context, RuntimeConfig) (bool, error) {
					probed = true
					return true, nil
				},
				Open: func(context.Context, RuntimeConfig) (*gorm.DB, error) {
					probed = true
					return openAcceptanceTestDB(t), nil
				},
			})
			if _, err := guard.Verify(context.Background(), config); err == nil {
				t.Fatal("Verify() accepted a mismatched local target")
			}
			if probed {
				t.Fatal("Verify() probed Docker or opened a database before rejecting runtime identity")
			}
		})
	}
}

func validRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		DatabaseDSN:       "postgres://acceptance@127.0.0.1:15433/image_agent_acceptance?sslmode=disable",
		EnvironmentMarker: "marker-1",
		ComposeProject:    "task-processor-image-agent-acceptance",
		IssuerURL:         "http://localhost:8080",
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
