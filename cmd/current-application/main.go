package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	aistore "task-processor/internal/aicapability/store"
	"task-processor/internal/app/httpapi"
	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/app/runtime/currentapplication"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	platformdatabase "task-processor/internal/platform/database"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute() error {
	manifestPath := flag.String("config", "", "absolute path to the private current-application JSON manifest")
	shutdownFile := flag.String("shutdown-file", "", "optional absolute path to a private graceful-shutdown trigger")
	flag.Parse()
	if *manifestPath == "" {
		return fmt.Errorf("-config is required")
	}
	cfg, err := currentapplication.LoadConfig(*manifestPath)
	if err != nil {
		return err
	}
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *shutdownFile != "" {
		var cancel context.CancelFunc
		ctx, cancel, err = currentapplication.ContextWithShutdownFile(ctx, *shutdownFile)
		if err != nil {
			return err
		}
		defer cancel()
	}
	return currentapplication.Run(ctx, cfg, logger, currentapplication.Dependencies{
		IdentityPreflight: currentapplication.VerifyIdentityProvider,
		NewKnowledge:      prepareKnowledge,
		NewEcoservices:    prepareEcoservices,
		OpenEcoservices: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenKnowledge: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenStoreCenter: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenLocalTrial: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},

		OpenSourceAccount: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},

		OpenCommercialOwner: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenMoneyOwner: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenProductAcquisition: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenImageAgent: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenAccountAuditUsage: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingReadOnlyContext(ctx, databaseConfig(cfg))
		},
		OpenProductAgent: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenAIWorkbench: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		DialImageAgentWorkflow: func(ctx context.Context, address, namespace string) (imageagent.WorkflowClient, func() error, error) {
			return appruntime.DialOrganizationImageAgentTemporalWorkflowClient(ctx, address, namespace)
		},
		OpenReferrals: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenMembership: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		NewApplicationWithFeatures: func(ctx context.Context, source *gorm.DB, features currentapplication.ApplicationFeatures, cfg *coreconfig.Config, logger *logrus.Logger) (*http.Server, error) {
			options := make([]httpapi.CurrentApplicationOption, 0, 6)
			if features.Ecoservices != nil {
				e := features.Ecoservices
				options = append(options, httpapi.WithEcoservices(httpapi.EcoservicesDependencies{DB: e.DB, Objects: e.Objects, Channel: e.Channel, Protection: e.Protection, MerchantProtection: e.MerchantProtection}))
			}
			if features.Knowledge != nil {
				options = append(options, httpapi.WithKnowledge(features.Knowledge))
			}
			if features.ProductAgent != nil && !features.ProductAgent.Enabled {
				options = append(options, httpapi.WithAgentConfiguration(features.ProductAgentDB))
			}
			if features.ProductAgent != nil && features.ProductAgent.Enabled {
				p := features.ProductAgent
				ledger := aistore.NewGormInvocationRecorder(features.ProductAgentDB)
				options = append(options, httpapi.WithProductAgent(httpapi.ProductAgentDependencies{RunDB: features.ProductAgentDB, ReviewDB: features.ProductReviewDB, AssetDB: features.ProductAgentAssetDB, Ledger: ledger, TextPolicies: p.TextPolicies, Enabled: true, AllowedOrganizationIDs: p.AllowedOrganizationIDs, Limits: p.Limits()}))
			}
			if features.AIWorkbench != nil && features.AIWorkbench.Enabled {
				options = append(options, httpapi.WithAIWorkbench(httpapi.AIWorkbenchDependencies{DB: features.AIWorkbenchDB, PlanningTextPolicies: features.AIWorkbench.PlanningTextPolicies}))
			}
			if features.RuntimeContext != nil {
				options = append(options, httpapi.WithRuntimeContext(features.RuntimeContext))
			}
			if features.CommercialOwnerDB != nil {
				options = append(options, httpapi.WithCommercialOwnerDatabase(features.CommercialOwnerDB))
			}
			if features.StoreCenterDB != nil {
				options = append(options, httpapi.WithStoreCenter(features.StoreCenterDB))
				if features.OfficialStoreProvider != nil || features.OfficialStoreProtection != nil {
					options = append(options, httpapi.WithStoreOfficialConnection(features.OfficialStoreProvider, features.OfficialStoreProtection))
				}
			}
			if features.LocalTrialDB != nil {
				options = append(options, httpapi.WithIssue36Trial(features.LocalTrialDB))
			}
			if features.MoneyOwnerDB != nil {
				options = append(options, httpapi.WithMoneyOwnerDatabase(features.MoneyOwnerDB))
			}
			if features.ProductAcquisitionDB != nil {
				options = append(options, httpapi.WithProductAcquisition(features.ProductAcquisitionDB))
				options = append(options, httpapi.WithBrowserCapture())
			}
			if features.ImageAgentDB != nil {
				options = append(options, httpapi.WithAcquisitionImageAgent(features.ImageAgentDB, features.ImageAgentWorkflow))
			}
			if features.AccountAuditImageDB != nil || features.AccountAuditProductDB != nil {
				options = append(options, httpapi.WithAccountAuditUsageSources(features.AccountAuditImageDB, features.AccountAuditProductDB))
			}
			if features.ReferralDB != nil {
				options = append(options, httpapi.WithReferrals(features.ReferralDB))
			}
			if features.MembershipDB != nil {
				if features.Membership == nil {
					return nil, fmt.Errorf("membership configuration unavailable")
				}
				options = append(options, httpapi.WithMembership(httpapi.MembershipDependencies{ReceiptDB: features.MembershipDB, ProviderOrigin: features.Membership.ProviderOrigin, ReadToken: features.Membership.ReadToken, WriteToken: features.Membership.WriteToken, InvitationMail: features.Membership.InvitationMail}))
			}
			server, buildErr := httpapi.NewCurrentApplicationWithOptions(ctx, source, cfg, logger, options...)
			return server, buildErr
		},
		NewApplication: func(ctx context.Context, source *gorm.DB, cfg *coreconfig.Config, logger *logrus.Logger) (*http.Server, error) {
			return httpapi.NewCurrentApplication(ctx, source, cfg, logger)
		},
		NewReferralsApplication: func(ctx context.Context, source, referrals *gorm.DB, cfg *coreconfig.Config, logger *logrus.Logger) (*http.Server, error) {
			return httpapi.NewCurrentApplicationWithOptions(ctx, source, cfg, logger, httpapi.WithReferrals(referrals))
		},
		CloseDatabase: platformdatabase.Close,
	})
}

func databaseConfig(cfg currentapplication.DatabaseConfig) *platformdatabase.Config {
	return &platformdatabase.Config{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password, Database: cfg.Database,
		MaxConnections: cfg.MaxConnections, MaxIdleConnections: cfg.MaxConnections, ConnectionMaxLifetime: time.Hour,
	}
}
