package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	observationruntime "task-processor/internal/app/runtime/storeobservations"
	supplyruntime "task-processor/internal/app/runtime/supplychain"
	"time"

	"github.com/sirupsen/logrus"
	"go.temporal.io/sdk/client"
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
		OpenToolMarket: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		NewKnowledge:   prepareKnowledge,
		NewEcoservices: prepareEcoservices,
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
		OpenImageSetWorker: openImageSetWorker,
		DialImageSetWorkflow: func(ctx context.Context, address, namespace string) (client.Client, func() error, error) {
			current, err := client.DialContext(ctx, client.Options{HostPort: address, Namespace: namespace})
			if err != nil {
				return nil, nil, err
			}
			return current, func() error { current.Close(); return nil }, nil
		},
		OpenSupplyAssets: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		DialSupplyWorkflow:            dialCurrentWorkflow,
		DialStoreObservationsWorkflow: dialCurrentWorkflow,
		OpenAccountAuditUsage: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingReadOnlyContext(ctx, databaseConfig(cfg))
		},
		OpenProductAgent: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenAIWorkbench: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenNotificationCenter: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
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
			if features.NotificationCenterDB != nil {
				options = append(options, httpapi.WithNotificationCenter(features.NotificationCenterDB))
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
				if features.OfficialStoreApplications != nil {
					options = append(options, httpapi.WithStoreOfficialApplications(features.OfficialStoreApplications))
				}
			}
			if features.StoreObservationsWorkflow != nil {
				options = append(options, httpapi.WithStoreObservations(httpapi.StoreObservationsDependencies{Starter: observationruntime.Starter{Client: features.StoreObservationsWorkflow}, Lifecycle: features.StoreObservationsLifecycle, NewWorker: observationruntime.WorkerFactory(features.StoreObservationsWorkflow, features.StoreObservationsLifecycle)}))
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
			if features.ToolMarketDB != nil && features.ToolMarket != nil {
				options = append(options, httpapi.WithToolMarket(httpapi.ToolMarketDependencies{DB: features.ToolMarketDB, Package: features.ToolMarket.PackageConfig()}))
			}
			if features.ProductCollections {
				options = append(options, httpapi.WithProductCollections())
			}
			if features.SourceMediaStorage != nil {
				options = append(options, httpapi.WithCollectionSourceMedia(features.SourceMediaStorage))
			}
			if features.SupplyAssetDB != nil {
				options = append(options, httpapi.WithSupplyChain(httpapi.SupplyChainDependencies{AssetDB: features.SupplyAssetDB, Starter: supplyruntime.TemporalOperationStarter{Client: features.SupplyWorkflow}, NewWorker: supplyruntime.WorkerFactory(features.SupplyWorkflow), Worker: features.SupplyWorker}))
			}
			if features.ImageAgentDB != nil {
				if features.ImageSetWorkerConfig != nil {
					options = append(options, httpapi.WithFullImageSet(features.ImageAgentDB, features.ImageAgentWorkflow, httpapi.FullImageSetDependencies{WorkerDB: features.ImageSetWorkerDB, WorkerConfig: features.ImageSetWorkerConfig, Client: features.ImageSetTemporal, Worker: features.ImageSetWorker}))
				} else {
					options = append(options, httpapi.WithAcquisitionImageAgent(features.ImageAgentDB, features.ImageAgentWorkflow))
				}
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

func dialCurrentWorkflow(ctx context.Context, address, namespace string) (client.Client, func() error, error) {
	current, err := client.DialContext(ctx, client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		return nil, nil, err
	}
	return current, func() error { current.Close(); return nil }, nil
}
