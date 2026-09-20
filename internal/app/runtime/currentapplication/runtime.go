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
	IdentityPreflight                         func(context.Context, IdentityConfig) error
	OpenSourceAccount                         func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenCommercial                            func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenProductAcquisition                    func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenReferrals                             func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenMembership                            func(context.Context, DatabaseConfig) (*gorm.DB, error)
	NewApplicationWithFeatures                func(context.Context, *gorm.DB, *gorm.DB, ApplicationFeatures, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	NewApplicationWithAcquisition             func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	NewReferralsApplication                   func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	NewApplicationWithAcquisitionAndReferrals func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	NewApplicationWithMembership              func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *coreconfig.Config, *MembershipConfig, *logrus.Logger) (*http.Server, error)
	NewApplication                            func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	Listen                                    func(string, string) (net.Listener, error)
	CloseDatabase                             func(*gorm.DB) error
	ShutdownTimeout                           time.Duration
}

// ApplicationFeatures keeps separately owned, opt-in current modules together
// only at the serving composition boundary.
type ApplicationFeatures struct {
	ProductAcquisitionDB *gorm.DB
	ReferralDB           *gorm.DB
	MembershipDB         *gorm.DB
	Membership           *MembershipConfig
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
	core := cfg.CoreConfig()
	startupContext, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStartup()
	if cfg.Referrals.Enabled {
		if dependencies.OpenReferrals == nil || dependencies.NewReferralsApplication == nil {
			return errors.New("current application referrals lifecycle unavailable")
		}
		secrets, err := cfg.Referrals.Prepare(startupContext)
		if err != nil {
			return err
		}
		core.Referrals.Prepared = secrets
		defer secrets.HTTPClient.CloseIdleConnections()
	}
	if dependencies.ShutdownTimeout <= 0 {
		dependencies.ShutdownTimeout = 10 * time.Second
	}
	if dependencies.IdentityPreflight == nil || dependencies.OpenSourceAccount == nil || dependencies.OpenCommercial == nil || dependencies.CloseDatabase == nil {
		return errors.New("current application database lifecycle unavailable")
	}
	if cfg.ProductAcquisitionDatabase != nil && dependencies.OpenProductAcquisition == nil {
		return errors.New("current product acquisition lifecycle unavailable")
	}
	if cfg.Referrals.Enabled && dependencies.OpenReferrals == nil {
		return errors.New("current application referrals lifecycle unavailable")
	}
	if cfg.Membership != nil && dependencies.OpenMembership == nil {
		return errors.New("membership runtime dependencies unavailable")
	}
	if dependencies.NewApplicationWithFeatures == nil {
		if cfg.Membership != nil && (cfg.ProductAcquisitionDatabase != nil || cfg.Referrals.Enabled) {
			return errors.New("current application combined membership lifecycle unavailable")
		}
		if cfg.Membership != nil && dependencies.NewApplicationWithMembership == nil {
			return errors.New("membership runtime dependencies unavailable")
		}
		if cfg.ProductAcquisitionDatabase != nil && cfg.Referrals.Enabled && dependencies.NewApplicationWithAcquisitionAndReferrals == nil {
			return errors.New("current application combined lifecycle unavailable")
		}
		if cfg.ProductAcquisitionDatabase != nil && !cfg.Referrals.Enabled && dependencies.NewApplicationWithAcquisition == nil {
			return errors.New("current product acquisition lifecycle unavailable")
		}
		if cfg.Referrals.Enabled && cfg.ProductAcquisitionDatabase == nil && dependencies.NewReferralsApplication == nil {
			return errors.New("current application referrals lifecycle unavailable")
		}
	}
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

	var productDB *gorm.DB
	if cfg.ProductAcquisitionDatabase != nil {
		productDB, err = dependencies.OpenProductAcquisition(startupContext, *cfg.ProductAcquisitionDatabase)
		if err != nil {
			return fmt.Errorf("open existing product acquisition database: %w", err)
		}
		if productDB == nil || productDB == sourceAccountDB || productDB == commercialDB {
			return errors.New("current product acquisition database unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(productDB)) }()
		if err := startupContext.Err(); err != nil {
			return fmt.Errorf("current application startup canceled: %w", err)
		}
	}
	var referralDB *gorm.DB
	if cfg.Referrals.Enabled {
		referralDB, err = dependencies.OpenReferrals(startupContext, cfg.Referrals.Database)
		if err != nil {
			return fmt.Errorf("open existing referral database: %w", err)
		}
		if referralDB == nil || referralDB == sourceAccountDB || referralDB == commercialDB || referralDB == productDB {
			return errors.New("current application referral database unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(referralDB)) }()
		if err := startupContext.Err(); err != nil {
			return fmt.Errorf("current application startup canceled: %w", err)
		}
	}
	var membershipDB *gorm.DB
	if cfg.Membership != nil {
		membershipDB, err = dependencies.OpenMembership(startupContext, cfg.Membership.Database)
		if err != nil {
			return fmt.Errorf("open existing membership database: %w", err)
		}
		if membershipDB == nil || membershipDB == sourceAccountDB || membershipDB == commercialDB || membershipDB == productDB || membershipDB == referralDB {
			return errors.New("membership requires an independent database pool")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(membershipDB)) }()
		if err := startupContext.Err(); err != nil {
			return fmt.Errorf("membership startup canceled: %w", err)
		}
	}
	if dependencies.Listen == nil {
		return errors.New("current application serving lifecycle unavailable")
	}
	if dependencies.NewApplicationWithFeatures == nil && productDB == nil && referralDB == nil && membershipDB == nil && dependencies.NewApplication == nil {
		return errors.New("current application serving lifecycle unavailable")
	}
	var server *http.Server
	if dependencies.NewApplicationWithFeatures != nil {
		server, err = dependencies.NewApplicationWithFeatures(startupContext, sourceAccountDB, commercialDB, ApplicationFeatures{ProductAcquisitionDB: productDB, ReferralDB: referralDB, MembershipDB: membershipDB, Membership: cfg.Membership}, core, logger)
	} else if membershipDB != nil {
		server, err = dependencies.NewApplicationWithMembership(startupContext, sourceAccountDB, commercialDB, membershipDB, core, cfg.Membership, logger)
	} else if productDB != nil {
		if referralDB != nil {
			server, err = dependencies.NewApplicationWithAcquisitionAndReferrals(startupContext, sourceAccountDB, commercialDB, productDB, referralDB, core, logger)
		} else {
			server, err = dependencies.NewApplicationWithAcquisition(startupContext, sourceAccountDB, commercialDB, productDB, core, logger)
		}
	} else if referralDB != nil {
		server, err = dependencies.NewReferralsApplication(startupContext, sourceAccountDB, commercialDB, referralDB, core, logger)
	} else {
		server, err = dependencies.NewApplication(startupContext, sourceAccountDB, commercialDB, core, logger)
	}
	if err != nil {
		return fmt.Errorf("construct current application: %w", err)
	}
	if server == nil {
		return errors.New("current application server unavailable")
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
