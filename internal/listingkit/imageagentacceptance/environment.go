package imageagentacceptance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	DatabaseName                   = "image_agent_acceptance"
	DatabaseUser                   = "acceptance"
	DatabasePort                   = "15433"
	ComposeProjectName             = "task-processor-image-agent-acceptance"
	ComposePostgresService         = "acceptance-postgres"
	IssuerPort                     = "8080"
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
	if strings.TrimSpace(config.ComposeProject) != ComposeProjectName {
		return errors.New("acceptance runtime Compose project is not the isolated acceptance project")
	}
	if strings.EqualFold(strings.TrimSpace(config.APIClientID), "pending-provision") ||
		strings.EqualFold(strings.TrimSpace(config.APIClientSecret), "pending-provision") {
		return errors.New("acceptance runtime API credentials are still pending provisioning")
	}
	if err := validateAcceptanceDatabaseDSN(config.DatabaseDSN); err != nil {
		return err
	}
	if err := validateAcceptanceIssuer(config.IssuerURL); err != nil {
		return err
	}
	return nil
}

func validateAcceptanceDatabaseDSN(raw string) error {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil {
		return errors.New("acceptance runtime database DSN is invalid")
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return errors.New("acceptance runtime database DSN must use PostgreSQL")
	}
	query := parsed.Query()
	if len(query) != 1 || len(query["sslmode"]) != 1 || query.Get("sslmode") != "disable" {
		return errors.New("acceptance runtime database DSN only allows sslmode=disable")
	}
	config, err := pgx.ParseConfig(raw)
	if err != nil {
		return errors.New("acceptance runtime database DSN is invalid")
	}
	if !isLoopbackHost(config.Host) || config.Port != 15433 {
		return errors.New("acceptance runtime database DSN must target the isolated loopback port")
	}
	if config.User != DatabaseUser {
		return errors.New("acceptance runtime database DSN must use the acceptance user")
	}
	if config.Database != DatabaseName {
		return errors.New("acceptance runtime database DSN must target the acceptance database")
	}
	return nil
}

func validateAcceptanceIssuer(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "http" || parsed.User != nil {
		return errors.New("acceptance runtime issuer must be a local HTTP URL")
	}
	if !isLoopbackHost(parsed.Hostname()) || parsed.Port() != IssuerPort {
		return errors.New("acceptance runtime issuer must target the isolated loopback port")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return errors.New("acceptance runtime issuer must not contain a path")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
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
