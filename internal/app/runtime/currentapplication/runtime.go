package currentapplication

import (
	"context"
	"errors"
	"fmt"
	"go.temporal.io/sdk/client"
	"net"
	"net/http"
	"task-processor/internal/app/productsourcing"
	supplyapp "task-processor/internal/app/supplychain"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	aistore "task-processor/internal/aicapability/store"
	storeapp "task-processor/internal/app/storecenter"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/knowledge"
)

type Dependencies struct {
	OpenAgentCustomization       func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenEcoservices              func(context.Context, DatabaseConfig) (*gorm.DB, error)
	NewEcoservices               func(context.Context, *gorm.DB, *EcoservicesConfig, *logrus.Logger) (*EcoservicesRuntime, error)
	OpenNotificationCenter       func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenKnowledge                func(context.Context, DatabaseConfig) (*gorm.DB, error)
	NewKnowledge                 func(context.Context, *gorm.DB, *KnowledgeConfig, *logrus.Logger) (*knowledge.Service, *knowledge.Processor, error)
	OpenStoreCenter              func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenLocalTrial               func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenProductAgent             func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenAIWorkbench              func(context.Context, DatabaseConfig) (*gorm.DB, error)
	IdentityPreflight            func(context.Context, IdentityConfig) error
	OpenSourceAccount            func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenCommercialOwner          func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenMoneyOwner               func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenProductAcquisition       func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenImageAgent               func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenAccountAuditUsage        func(context.Context, DatabaseConfig) (*gorm.DB, error)
	DialImageAgentWorkflow       func(context.Context, string, string) (imageagent.WorkflowClient, func() error, error)
	OpenSupplyAssets             func(context.Context, DatabaseConfig) (*gorm.DB, error)
	DialSupplyWorkflow           func(context.Context, string, string) (client.Client, func() error, error)
	OpenReferrals                func(context.Context, DatabaseConfig) (*gorm.DB, error)
	OpenMembership               func(context.Context, DatabaseConfig) (*gorm.DB, error)
	NewApplicationWithFeatures   func(context.Context, *gorm.DB, ApplicationFeatures, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	NewReferralsApplication      func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	NewApplicationWithMembership func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *MembershipConfig, *logrus.Logger) (*http.Server, error)
	NewApplication               func(context.Context, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error)
	Listen                       func(string, string) (net.Listener, error)
	CloseDatabase                func(*gorm.DB) error
	ShutdownTimeout              time.Duration
}

// ApplicationFeatures keeps separately owned, opt-in current modules together
// only at the serving composition boundary.
type ApplicationFeatures struct {
	AgentCustomizationDB                                 *gorm.DB
	Ecoservices                                          *EcoservicesRuntime
	SourceMediaStorage                                   productsourcing.SourceMediaStorage
	NotificationCenterDB                                 *gorm.DB
	Knowledge                                            *knowledge.Service
	StoreCenterDB                                        *gorm.DB
	LocalTrialDB                                         *gorm.DB
	OfficialStoreApplications                            *storeapp.OfficialApplicationRegistry
	ProductAgentDB, ProductReviewDB, ProductAgentAssetDB *gorm.DB
	ProductAgent                                         *ProductAgentConfig
	AIWorkbenchDB                                        *gorm.DB
	AIWorkbench                                          *AIWorkbenchConfig
	CommercialOwnerDB                                    *gorm.DB
	MoneyOwnerDB                                         *gorm.DB
	ProductAcquisitionDB                                 *gorm.DB
	ProductCollections                                   bool
	SupplyAssetDB                                        *gorm.DB
	SupplyWorkflow                                       client.Client
	SupplyWorker                                         *supplyapp.OperationWorker
	ImageAgentDB                                         *gorm.DB
	AccountAuditImageDB, AccountAuditProductDB           *gorm.DB
	ImageAgentWorkflow                                   imageagent.WorkflowClient
	ReferralDB                                           *gorm.DB
	MembershipDB                                         *gorm.DB
	Membership                                           *MembershipConfig
	// RuntimeContext is the long-lived process context. Background recovery
	// must not inherit the bounded startup context passed to the constructor.
	RuntimeContext context.Context
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
	if cfg.Ecoservices != nil && cfg.Ecoservices.Enabled && (dependencies.OpenEcoservices == nil || dependencies.NewEcoservices == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("ecoservices runtime lifecycle unavailable")
	}
	var officialApplications *storeapp.OfficialApplicationRegistry
	if cfg.StoreCenter != nil && cfg.StoreCenter.Enabled {
		var err error
		officialApplications, err = prepareOfficialApplications(ctx, cfg.StoreCenter.OfficialApplications)
		if err != nil {
			return err
		}
	}
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
	if dependencies.IdentityPreflight == nil || dependencies.OpenSourceAccount == nil || dependencies.CloseDatabase == nil {
		return errors.New("current application database lifecycle unavailable")
	}
	if cfg.AgentCustomizationDatabase != nil && (dependencies.OpenAgentCustomization == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("agent customization runtime lifecycle unavailable")
	}
	if cfg.ProductAcquisitionDatabase != nil && dependencies.OpenProductAcquisition == nil {
		return errors.New("current product acquisition lifecycle unavailable")
	}
	if cfg.StoreCenter != nil && cfg.StoreCenter.Enabled && (dependencies.OpenStoreCenter == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("store center runtime lifecycle unavailable")
	}
	if cfg.LocalTrial != nil && (dependencies.OpenLocalTrial == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("local trial runtime lifecycle unavailable")
	}
	if cfg.Knowledge != nil && cfg.Knowledge.Enabled && (dependencies.OpenKnowledge == nil || dependencies.NewKnowledge == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("knowledge runtime lifecycle unavailable")
	}
	if cfg.ProductAgent != nil && (dependencies.OpenProductAgent == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("product agent lifecycle unavailable")
	}
	if cfg.AIWorkbench != nil && cfg.AIWorkbench.Enabled && (dependencies.OpenAIWorkbench == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("AI Workbench lifecycle unavailable")
	}
	if cfg.ImageAgent != nil && (dependencies.OpenImageAgent == nil || dependencies.DialImageAgentWorkflow == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("current image agent owner and organization workflow lifecycle unavailable")
	}
	if cfg.SupplyChain != nil && (dependencies.OpenSupplyAssets == nil || dependencies.DialSupplyWorkflow == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("supply chain runtime dependencies unavailable")
	}
	if cfg.AccountAuditUsage != nil && (dependencies.OpenAccountAuditUsage == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("account audit usage read-only lifecycle unavailable")
	}
	if cfg.CommercialOwnerDatabase != nil && dependencies.OpenCommercialOwner == nil {
		return errors.New("current commercial owner lifecycle unavailable")
	}
	if cfg.MoneyOwnerDatabase != nil && (dependencies.OpenMoneyOwner == nil || dependencies.NewApplicationWithFeatures == nil) {
		return errors.New("current money owner lifecycle unavailable")
	}
	if cfg.Referrals.Enabled && dependencies.OpenReferrals == nil {
		return errors.New("current application referrals lifecycle unavailable")
	}
	if cfg.Membership != nil && dependencies.OpenMembership == nil {
		return errors.New("membership runtime dependencies unavailable")
	}
	if dependencies.NewApplicationWithFeatures == nil {
		if cfg.CommercialOwnerDatabase != nil {
			return errors.New("current application combined commercial owner lifecycle unavailable")
		}
		if cfg.Membership != nil && (cfg.ProductAcquisitionDatabase != nil || cfg.Referrals.Enabled) {
			return errors.New("current application combined membership lifecycle unavailable")
		}
		if cfg.Membership != nil && dependencies.NewApplicationWithMembership == nil {
			return errors.New("membership runtime dependencies unavailable")
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

	var commercialOwnerDB *gorm.DB
	if cfg.CommercialOwnerDatabase != nil {
		commercialOwnerDB, err = dependencies.OpenCommercialOwner(startupContext, *cfg.CommercialOwnerDatabase)
		if err != nil {
			return fmt.Errorf("open existing commercial owner database: %w", err)
		}
		if commercialOwnerDB == nil || commercialOwnerDB == sourceAccountDB {
			return errors.New("commercial owner database unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(commercialOwnerDB)) }()
		if err := startupContext.Err(); err != nil {
			return fmt.Errorf("current application startup canceled: %w", err)
		}
	}

	var moneyOwnerDB *gorm.DB
	if cfg.MoneyOwnerDatabase != nil {
		moneyOwnerDB, err = dependencies.OpenMoneyOwner(startupContext, *cfg.MoneyOwnerDatabase)
		if err != nil {
			return errors.New("open existing money owner database failed")
		}
		if moneyOwnerDB == nil || moneyOwnerDB == sourceAccountDB || moneyOwnerDB == commercialOwnerDB {
			return errors.New("money owner database unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(moneyOwnerDB)) }()
	}
	var productDB *gorm.DB
	if cfg.ProductAcquisitionDatabase != nil {
		productDB, err = dependencies.OpenProductAcquisition(startupContext, *cfg.ProductAcquisitionDatabase)
		if err != nil {
			return fmt.Errorf("open existing product acquisition database: %w", err)
		}
		if productDB == nil || productDB == sourceAccountDB || productDB == commercialOwnerDB {
			return errors.New("current product acquisition database unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(productDB)) }()
		if err := startupContext.Err(); err != nil {
			return fmt.Errorf("current application startup canceled: %w", err)
		}
	}
	var agentDB, agentReviewDB, agentAssetDB *gorm.DB
	if cfg.ProductAgent != nil {
		targets := []struct {
			cfg  DatabaseConfig
			dest **gorm.DB
		}{{cfg.ProductAgent.Database, &agentDB}}
		if cfg.ProductAgent.Enabled {
			targets = append(targets, struct {
				cfg  DatabaseConfig
				dest **gorm.DB
			}{cfg.ProductAgent.ReviewDatabase, &agentReviewDB}, struct {
				cfg  DatabaseConfig
				dest **gorm.DB
			}{cfg.ProductAgent.AssetDatabase, &agentAssetDB})
		}
		for _, target := range targets {
			pool, openErr := dependencies.OpenProductAgent(startupContext, target.cfg)
			if openErr != nil {
				return errors.New("open existing product agent owner database failed")
			}
			if pool == nil {
				return errors.New("product agent owner pool unavailable")
			}
			*target.dest = pool
			defer func(db *gorm.DB) { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(db)) }(pool)
		}
	}
	var workbenchDB *gorm.DB
	var notificationDB *gorm.DB
	if cfg.NotificationCenterDatabase != nil {
		if dependencies.OpenNotificationCenter == nil || dependencies.NewApplicationWithFeatures == nil {
			return errors.New("notification center dependencies unavailable")
		}
		notificationDB, err = dependencies.OpenNotificationCenter(startupContext, *cfg.NotificationCenterDatabase)
		if err != nil || notificationDB == nil || notificationDB == sourceAccountDB || notificationDB == commercialOwnerDB || notificationDB == productDB || notificationDB == agentDB {
			return errors.New("notification center requires an independent existing database")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(notificationDB)) }()
	}
	if cfg.AIWorkbench != nil && cfg.AIWorkbench.Enabled {
		workbenchDB, err = dependencies.OpenAIWorkbench(startupContext, cfg.AIWorkbench.Database)
		if err != nil || workbenchDB == nil || workbenchDB == agentDB {
			return errors.New("open existing AI Workbench runtime database failed")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(workbenchDB)) }()
	}
	var imageDB *gorm.DB
	var imageWorkflow imageagent.WorkflowClient
	if cfg.ImageAgent != nil {
		imageDB, err = dependencies.OpenImageAgent(startupContext, cfg.ImageAgent.Database)
		if err != nil {
			return fmt.Errorf("open existing image agent owner database: %w", err)
		}
		if imageDB == nil || imageDB == sourceAccountDB || imageDB == commercialOwnerDB || imageDB == productDB {
			return errors.New("current image agent owner database unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(imageDB)) }()
		var closeWorkflow func() error
		imageWorkflow, closeWorkflow, err = dependencies.DialImageAgentWorkflow(startupContext, cfg.ImageAgent.TemporalAddress, cfg.ImageAgent.TemporalNamespace)
		if err != nil {
			return fmt.Errorf("connect organization image agent workflow: %w", err)
		}
		if imageWorkflow == nil || closeWorkflow == nil {
			return errors.New("organization image agent workflow unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, closeWorkflow()) }()
	}
	var auditImageDB, auditProductDB *gorm.DB
	if cfg.AccountAuditUsage != nil {
		for _, target := range []struct {
			cfg  DatabaseConfig
			dest **gorm.DB
		}{
			{cfg.AccountAuditUsage.Image, &auditImageDB},
			{cfg.AccountAuditUsage.Product, &auditProductDB},
		} {
			if target.cfg == (DatabaseConfig{}) {
				continue // Config validation binds this namespace to its enabled Agent.
			}
			pool, openErr := dependencies.OpenAccountAuditUsage(startupContext, target.cfg)
			if openErr != nil {
				return fmt.Errorf("open account audit %s read-only owner: %w", target.cfg.Database, openErr)
			}
			if pool == nil || pool == sourceAccountDB || pool == commercialOwnerDB || pool == productDB || pool == imageDB || pool == auditImageDB {
				return errors.New("account audit usage requires independent owner pools")
			}
			*target.dest = pool
			defer func(db *gorm.DB) { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(db)) }(pool)
		}
		for _, target := range []struct {
			namespace string
			pool      *gorm.DB
		}{{"image", auditImageDB}, {"product", auditProductDB}} {
			if target.pool == nil {
				continue
			}
			if _, err := aistore.NewGormInvocationRecorder(target.pool).ListObservedUsage(startupContext, "__account_audit_probe__", target.namespace, 1, nil); err != nil {
				return fmt.Errorf("account audit %s usage owner unreadable: %w", target.namespace, err)
			}
		}
	}
	var referralDB *gorm.DB
	if cfg.Referrals.Enabled {
		referralDB, err = dependencies.OpenReferrals(startupContext, cfg.Referrals.Database)
		if err != nil {
			return fmt.Errorf("open existing referral database: %w", err)
		}
		if referralDB == nil || referralDB == sourceAccountDB || referralDB == commercialOwnerDB || referralDB == productDB {
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
		if membershipDB == nil || membershipDB == sourceAccountDB || membershipDB == commercialOwnerDB || membershipDB == productDB || membershipDB == referralDB {
			return errors.New("membership requires an independent database pool")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(membershipDB)) }()
		if err := startupContext.Err(); err != nil {
			return fmt.Errorf("membership startup canceled: %w", err)
		}
	}

	var storeDB *gorm.DB
	if cfg.StoreCenter != nil && cfg.StoreCenter.Enabled {
		for _, target := range []struct {
			config DatabaseConfig
			open   func(context.Context, DatabaseConfig) (*gorm.DB, error)
			dest   **gorm.DB
		}{
			{cfg.StoreCenter.Database, dependencies.OpenStoreCenter, &storeDB},
		} {
			pool, openErr := target.open(startupContext, target.config)
			if openErr != nil {
				return fmt.Errorf("open store center database: %w", openErr)
			}
			if pool == nil {
				return errors.New("store center database unavailable")
			}
			for _, existing := range []*gorm.DB{sourceAccountDB, commercialOwnerDB, moneyOwnerDB, productDB, agentDB, agentReviewDB, agentAssetDB, imageDB, referralDB, membershipDB, storeDB} {
				if pool == existing {
					return errors.New("store center requires independently owned pools")
				}
			}
			*target.dest = pool
			defer func(db *gorm.DB) { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(db)) }(pool)
			if err := startupContext.Err(); err != nil {
				return err
			}
		}
	}
	var supplyAssetDB *gorm.DB
	var supplyWorkflow client.Client
	var supplyWorker supplyapp.OperationWorker
	if s := cfg.SupplyChain; s != nil {
		supplyAssetDB, err = dependencies.OpenSupplyAssets(startupContext, s.AssetDatabase)
		if err != nil || supplyAssetDB == nil {
			return errors.New("open supply Asset runtime owner failed")
		}
		for _, existing := range []*gorm.DB{sourceAccountDB, commercialOwnerDB, moneyOwnerDB, productDB, agentDB, agentReviewDB, agentAssetDB, imageDB, storeDB, notificationDB} {
			if supplyAssetDB == existing {
				return errors.New("supply requires its narrow independently opened Asset pool")
			}
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(supplyAssetDB)) }()
		var closeWorkflow func() error
		supplyWorkflow, closeWorkflow, err = dependencies.DialSupplyWorkflow(startupContext, s.TemporalAddress, s.TemporalNamespace)
		if err != nil || supplyWorkflow == nil || closeWorkflow == nil {
			return errors.New("supply workflow runtime unavailable")
		}
		defer func() { resultErr = errors.Join(resultErr, closeWorkflow()) }()
	}
	var trialDB *gorm.DB
	if cfg.LocalTrial != nil {
		trialDB, err = dependencies.OpenLocalTrial(startupContext, cfg.LocalTrial.Database)
		if err != nil {
			return fmt.Errorf("open isolated local trial database: %w", err)
		}
		if trialDB == nil {
			return errors.New("isolated local trial database unavailable")
		}
		for _, existing := range []*gorm.DB{sourceAccountDB, commercialOwnerDB, moneyOwnerDB, productDB, agentDB, agentReviewDB, agentAssetDB, workbenchDB, imageDB, auditImageDB, auditProductDB, referralDB, membershipDB, storeDB} {
			if trialDB == existing {
				return errors.New("local trial requires its dedicated runtime pool")
			}
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(trialDB)) }()
	}
	if dependencies.Listen == nil {
		return errors.New("current application serving lifecycle unavailable")
	}
	var knowledgeService *knowledge.Service
	var ecoservicesRuntime *EcoservicesRuntime
	if cfg.Ecoservices != nil && cfg.Ecoservices.Enabled {
		pool, openErr := dependencies.OpenEcoservices(startupContext, cfg.Ecoservices.Database)
		if openErr != nil || pool == nil {
			return errors.New("ecoservices owner database unavailable")
		}
		for _, existing := range []*gorm.DB{sourceAccountDB, commercialOwnerDB, moneyOwnerDB, productDB, agentDB, agentReviewDB, agentAssetDB, workbenchDB, imageDB, auditImageDB, auditProductDB, referralDB, membershipDB, storeDB, trialDB, notificationDB, supplyAssetDB} {
			if pool == existing {
				return errors.New("ecoservices requires an independent owner pool")
			}
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(pool)) }()
		ecoservicesRuntime, err = dependencies.NewEcoservices(startupContext, pool, cfg.Ecoservices, logger)
		if err != nil || ecoservicesRuntime == nil || ecoservicesRuntime.DB != pool {
			return errors.New("ecoservices runtime dependencies unavailable")
		}
	}
	var knowledgeProcessor *knowledge.Processor
	var knowledgeDB *gorm.DB
	if cfg.Knowledge != nil && cfg.Knowledge.Enabled {
		pool, openErr := dependencies.OpenKnowledge(startupContext, cfg.Knowledge.Database)
		if openErr != nil || pool == nil {
			return errors.New("knowledge database unavailable")
		}
		for _, existing := range []*gorm.DB{sourceAccountDB, commercialOwnerDB, moneyOwnerDB, productDB, agentDB, agentReviewDB, agentAssetDB, imageDB, referralDB, membershipDB, storeDB} {
			if existing == pool {
				return errors.New("knowledge requires an independently owned pool")
			}
		}
		if ecoservicesRuntime != nil && ecoservicesRuntime.DB == pool {
			return errors.New("knowledge and ecoservices require independent owner pools")
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(pool)) }()
		knowledgeService, knowledgeProcessor, err = dependencies.NewKnowledge(startupContext, pool, cfg.Knowledge, logger)
		knowledgeDB = pool
		if err != nil || knowledgeService == nil || knowledgeProcessor == nil {
			return errors.New("knowledge dependencies unavailable")
		}
	}
	if dependencies.NewApplicationWithFeatures == nil && commercialOwnerDB == nil && productDB == nil && referralDB == nil && membershipDB == nil && dependencies.NewApplication == nil {
		return errors.New("current application serving lifecycle unavailable")
	}
	var sourceMediaStorage productsourcing.SourceMediaStorage
	if cfg.SourceMedia != nil {
		sourceMediaStorage, err = NewSourceMediaStorage(*cfg.SourceMedia, logger)
		if err != nil {
			return errors.New("source media storage unavailable")
		}
	}
	var customizationDB *gorm.DB
	if cfg.AgentCustomizationDatabase != nil {
		customizationDB, err = dependencies.OpenAgentCustomization(startupContext, *cfg.AgentCustomizationDatabase)
		if err != nil || customizationDB == nil {
			return errors.New("agent customization owner database unavailable")
		}
		others := []*gorm.DB{sourceAccountDB, commercialOwnerDB, moneyOwnerDB, productDB, agentDB, agentReviewDB, agentAssetDB, workbenchDB, imageDB, auditImageDB, auditProductDB, referralDB, membershipDB, storeDB, trialDB, notificationDB, supplyAssetDB, knowledgeDB}
		if ecoservicesRuntime != nil {
			others = append(others, ecoservicesRuntime.DB)
		}
		for _, other := range others {
			if other == customizationDB {
				return errors.New("agent customization requires an independent owner pool")
			}
		}
		defer func() { resultErr = errors.Join(resultErr, dependencies.CloseDatabase(customizationDB)) }()
		if err := startupContext.Err(); err != nil {
			return fmt.Errorf("current application startup canceled: %w", err)
		}
	}
	var server *http.Server
	if dependencies.NewApplicationWithFeatures != nil {
		server, err = dependencies.NewApplicationWithFeatures(startupContext, sourceAccountDB, ApplicationFeatures{AgentCustomizationDB: customizationDB, Ecoservices: ecoservicesRuntime, NotificationCenterDB: notificationDB, Knowledge: knowledgeService, OfficialStoreApplications: officialApplications, StoreCenterDB: storeDB, LocalTrialDB: trialDB, MoneyOwnerDB: moneyOwnerDB, ProductAgentDB: agentDB, ProductReviewDB: agentReviewDB, ProductAgentAssetDB: agentAssetDB, ProductAgent: cfg.ProductAgent, AIWorkbenchDB: workbenchDB, AIWorkbench: cfg.AIWorkbench, CommercialOwnerDB: commercialOwnerDB, ProductAcquisitionDB: productDB, ProductCollections: cfg.ProductCollections, SourceMediaStorage: sourceMediaStorage, SupplyAssetDB: supplyAssetDB, SupplyWorkflow: supplyWorkflow, SupplyWorker: &supplyWorker, ImageAgentDB: imageDB, AccountAuditImageDB: auditImageDB, AccountAuditProductDB: auditProductDB, ImageAgentWorkflow: imageWorkflow, ReferralDB: referralDB, MembershipDB: membershipDB, Membership: cfg.Membership, RuntimeContext: ctx}, core, logger)
	} else if membershipDB != nil {
		server, err = dependencies.NewApplicationWithMembership(startupContext, sourceAccountDB, membershipDB, core, cfg.Membership, logger)
	} else if referralDB != nil {
		server, err = dependencies.NewReferralsApplication(startupContext, sourceAccountDB, referralDB, core, logger)
	} else {
		server, err = dependencies.NewApplication(startupContext, sourceAccountDB, core, logger)
	}
	if err != nil {
		return fmt.Errorf("construct current application: %w", err)
	}
	if server == nil {
		return errors.New("current application server unavailable")
	}
	if knowledgeProcessor != nil {
		processingContext, stopProcessing := context.WithCancel(ctx)
		processingDone := make(chan struct{})
		go func() { defer close(processingDone); _ = knowledgeProcessor.Run(processingContext) }()
		defer func() { stopProcessing(); <-processingDone }()
	}
	if err := startupContext.Err(); err != nil {
		return fmt.Errorf("current application startup canceled: %w", err)
	}
	listener, err := dependencies.Listen("tcp", cfg.ListenAddress())
	if err != nil {
		return fmt.Errorf("listen for current application: %w", err)
	}
	server.Addr = cfg.ListenAddress()
	if cfg.SupplyChain != nil {
		if supplyWorker == nil {
			_ = listener.Close()
			return errors.New("supply worker was not assembled")
		}
		if err := supplyWorker.Start(); err != nil {
			_ = listener.Close()
			return errors.New("start supply worker failed")
		}
		defer supplyWorker.Stop()
	}
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
