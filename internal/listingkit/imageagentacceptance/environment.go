package imageagentacceptance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	DatabaseName                   = "image_agent_acceptance"
	EnvironmentMarkerTable         = "listingkit_acceptance_environment"
	EnvironmentMarkerColumn        = "marker"
	composeProjectProbeUnavailable = "acceptance Compose project probe is not configured"
)

// EnvironmentGuard verifies the isolated acceptance target before mutation.
type EnvironmentGuard interface {
	Verify(context.Context, RuntimeConfig) (*gorm.DB, error)
}

// EnvironmentProbes are the external checks used by the guard. ComposeProject
// must prove that the configured endpoint belongs to the expected project.
// Keeping all probes injectable makes the guard testable without Docker or a
// live PostgreSQL connection.
type EnvironmentProbes struct {
	Open           func(context.Context, RuntimeConfig) (*gorm.DB, error)
	ComposeProject func(context.Context, RuntimeConfig) (bool, error)
	DatabaseName   func(context.Context, *gorm.DB) (string, error)
	Marker         func(context.Context, *gorm.DB) (string, error)
}

type environmentGuard struct {
	probes EnvironmentProbes
}

// NewEnvironmentGuard constructs a fail-closed acceptance guard.
func NewEnvironmentGuard(probes EnvironmentProbes) EnvironmentGuard {
	if probes.Open == nil {
		probes.Open = openRuntimeDatabase
	}
	if probes.DatabaseName == nil {
		probes.DatabaseName = currentDatabaseName
	}
	if probes.Marker == nil {
		probes.Marker = currentEnvironmentMarker
	}
	if probes.ComposeProject == nil {
		probes.ComposeProject = func(context.Context, RuntimeConfig) (bool, error) {
			return false, errors.New(composeProjectProbeUnavailable)
		}
	}
	return &environmentGuard{probes: probes}
}

func (g *environmentGuard) Verify(ctx context.Context, config RuntimeConfig) (*gorm.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateRuntimeConfig(config); err != nil {
		return nil, err
	}
	belongs, err := g.probes.ComposeProject(ctx, config)
	if err != nil {
		return nil, errors.New("verify Compose project identity failed")
	}
	if !belongs {
		return nil, errors.New("verify Compose project identity failed")
	}

	db, err := g.probes.Open(ctx, config)
	if err != nil {
		return nil, errors.New("open acceptance database failed")
	}
	if db == nil {
		return nil, errors.New("open acceptance database: probe returned nil database")
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = closeDatabase(db)
		}
	}()

	databaseName, err := g.probes.DatabaseName(ctx, db)
	if err != nil {
		return nil, errors.New("verify database name failed")
	}
	if strings.TrimSpace(databaseName) != DatabaseName {
		return nil, errors.New("verify database name failed")
	}
	marker, err := g.probes.Marker(ctx, db)
	if err != nil {
		return nil, errors.New("verify environment marker failed")
	}
	if strings.TrimSpace(marker) == "" || strings.TrimSpace(marker) != strings.TrimSpace(config.EnvironmentMarker) {
		return nil, errors.New("verify environment marker failed")
	}
	closeOnError = false
	return db, nil
}

func validateRuntimeConfig(config RuntimeConfig) error {
	fields := map[string]string{
		"database DSN":       config.DatabaseDSN,
		"environment marker": config.EnvironmentMarker,
		"Compose project":    config.ComposeProject,
		"issuer URL":         config.IssuerURL,
		"API client ID":      config.APIClientID,
		"API client secret":  config.APIClientSecret,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("acceptance runtime %s is required", name)
		}
	}
	return nil
}

func openRuntimeDatabase(ctx context.Context, config RuntimeConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(config.DatabaseDSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db.WithContext(ctx), nil
}

func currentDatabaseName(ctx context.Context, db *gorm.DB) (string, error) {
	var name string
	err := db.WithContext(ctx).Raw("SELECT current_database()").Scan(&name).Error
	return name, err
}

func currentEnvironmentMarker(ctx context.Context, db *gorm.DB) (string, error) {
	var marker string
	err := db.WithContext(ctx).Table(EnvironmentMarkerTable).Select(EnvironmentMarkerColumn).Limit(1).Scan(&marker).Error
	return marker, err
}

func closeDatabase(db *gorm.DB) error {
	if db == nil || db.ConnPool == nil {
		return nil
	}
	if closer, ok := db.ConnPool.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
