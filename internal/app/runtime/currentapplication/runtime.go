package currentapplication

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	coreconfig "task-processor/internal/core/config"
)

type Dependencies struct {
	IdentityPreflight func(context.Context, IdentityConfig) error
	OpenSourceAccount func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenCommercial    func(context.Context, DatabaseConfig) (*gorm.DB, error)
	NewApplication    func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	Listen            func(string, string) (net.Listener, error)
	CloseDatabase     func(*gorm.DB) error
	ShutdownTimeout   time.Duration
}

type runtimeDependencies = Dependencies

// Run opens only existing databases, constructs the current application and
// serves until cancellation. Schema installation and environment destruction
// are deliberately outside this runtime lifecycle.
func Run(ctx context.Context, cfg *Config, logger *logrus.Logger, dependencies Dependencies) error {
	if dependencies.Listen == nil {
		dependencies.Listen = net.Listen
	}
	return run(ctx, cfg, logger, dependencies)
}

func run(ctx context.Context, cfg *Config, logger *logrus.Logger, dependencies runtimeDependencies) (resultErr error) {
	if ctx == nil || cfg == nil || logger == nil {
		return errors.New("current application runtime dependencies unavailable")
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	if dependencies.ShutdownTimeout <= 0 {
		dependencies.ShutdownTimeout = 10 * time.Second
	}
	if dependencies.IdentityPreflight == nil || dependencies.OpenSourceAccount == nil || dependencies.OpenCommercial == nil || dependencies.CloseDatabase == nil {
		return errors.New("current application database lifecycle unavailable")
	}
	startupContext, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStartup()
	if err := dependencies.IdentityPreflight(startupContext, cfg.Identity); err != nil {
		return fmt.Errorf("verify identity provider readiness: %w", err)
	}
	if err := startupContext.Err(); err != nil {
		return fmt.Errorf("current application startup canceled: %w", err)
	}

	sourceAccountDB, err := dependencies.OpenSourceAccount(startupContext, cfg.SourceAccountDatabase)
	if err != nil {
		return fmt.Errorf("open existing source account database: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(sourceAccountDB)) }()
	if err := startupContext.Err(); err != nil {
		return fmt.Errorf("current application startup canceled: %w", err)
	}

	commercialDB, err := dependencies.OpenCommercial(startupContext, cfg.CommercialDatabase)
	if err != nil {
		return fmt.Errorf("open existing commercial database: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(commercialDB)) }()
	if err := startupContext.Err(); err != nil {
		return fmt.Errorf("current application startup canceled: %w", err)
	}

	if dependencies.NewApplication == nil || dependencies.Listen == nil {
		return errors.New("current application serving lifecycle unavailable")
	}
	server, err := dependencies.NewApplication(startupContext, sourceAccountDB, commercialDB, cfg.CoreConfig(), logger)
	if err != nil {
		return fmt.Errorf("construct current application: %w", err)
	}
	if err := startupContext.Err(); err != nil {
		return fmt.Errorf("current application startup canceled: %w", err)
	}
	listener, err := dependencies.Listen("tcp", cfg.ListenAddress())
	if err != nil {
		return fmt.Errorf("listen for current application: %w", err)
	}
	server.Addr = cfg.ListenAddress()
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()

	select {
	case serveErr := <-serveResult:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve current application: %w", serveErr)
		}
		return nil
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), dependencies.ShutdownTimeout)
		shutdownErr := server.Shutdown(shutdownContext)
		cancel()
		if shutdownErr != nil {
			shutdownErr = errors.Join(shutdownErr, server.Close())
		}
		serveErr := <-serveResult
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("serve current application: %w", serveErr))
		}
		if shutdownErr != nil {
			return fmt.Errorf("stop current application: %w", shutdownErr)
		}
		return nil
	}
}
