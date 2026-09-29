package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	registration "task-processor/internal/app/referralregistration"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	moneystore "task-processor/internal/integration/persistence/money"
	referralstore "task-processor/internal/integration/persistence/referral"
	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	"task-processor/internal/integration/zitadelregistration"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/knowledge"
	knowledgehttp "task-processor/internal/knowledge/httpapi"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
	verificationhttp "task-processor/internal/subjectverification/httpapi"
)

type currentApplicationRoute struct {
	Method string
	Path   string
}

var currentWorkbenchApplicationRoutes = []currentApplicationRoute{
	{Method: http.MethodGet, Path: "/api/v1/workbench/context"},
	{Method: http.MethodPut, Path: "/api/v1/workbench/context/effective-organization"},
	{Method: http.MethodGet, Path: "/api/v1/account/profile"},
	{Method: http.MethodGet, Path: "/api/v1/account/organization"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/overview"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/source-accounts"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/source-accounts"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/source-accounts/:source_account_id"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/source-accounts/:source_account_id/disable"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/source-accounts/:source_account_id/enable"},
}

var currentAccountProfileApplicationRoutes = []currentApplicationRoute{
	{Method: http.MethodGet, Path: "/api/v1/account/preferences"},
	{Method: http.MethodPut, Path: "/api/v1/account/preferences"},
	{Method: http.MethodGet, Path: "/api/v1/account/identity/facts"},
	{Method: http.MethodGet, Path: accountBusinessProfilePath},
	{Method: http.MethodPut, Path: accountBusinessProfilePath},
	{Method: http.MethodGet, Path: accountIdentityProfilePath},
	{Method: http.MethodPut, Path: accountIdentityProfilePath},
	{Method: http.MethodPut, Path: accountIdentityEmailPath},
	{Method: http.MethodPost, Path: accountIdentityEmailResendPath},
	{Method: http.MethodPost, Path: accountIdentityEmailVerifyPath},
	{Method: http.MethodPut, Path: accountIdentityPhonePath},
	{Method: http.MethodPost, Path: accountIdentityPhoneResendPath},
	{Method: http.MethodPost, Path: accountIdentityPhoneVerifyPath},
	{Method: http.MethodPut, Path: accountIdentityPasswordPath},
}

var currentCommercialBillingApplicationRoutes = []currentApplicationRoute{
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/resource-offers"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/wallet"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/wallet/entries"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/commercial/quotes"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/commercial/orders"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/orders"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/orders/:order_id"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/orders/summary"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/commercial/wallet/top-up-intents"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/wallet/top-up-options"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/commercial/orders/:order_id/checkout"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/commercial/orders/:order_id/cancel-payment"},
	{Method: http.MethodPost, Path: "/api/v1/admin/commercial/top-up-orders/:order_id/refunds"},
	{Method: http.MethodPost, Path: "/api/v1/payments/alipay/notify"},
	{Method: http.MethodPost, Path: "/api/v1/payments/wechat/notify"},
}

type currentApplicationFactories struct {
	buildStoreCenter         func(context.Context, *gorm.DB, *authz.ListingKitAuthorizer, orgresource.ConsumerChargePort, storecenter.OfficialConnectionProvider, storecenter.OfficialCredentialProtection) (kernelmodule.Module, error)
	buildResourceCharges     func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (*orgresource.ConsumerChargeService, error)
	buildCommercialResources func(context.Context, *gorm.DB) (kernelmodule.Module, error)
	buildWorkbench           workbenchContextModuleBuilder
	buildSourceAccount       func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildCommercial          func(*gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildCommercialBilling   func(context.Context, *gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer, *config.Config) (kernelmodule.Module, error)
	buildAcquisition         func(*authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
	buildAcquisitionImage    func(*authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
	buildBrowserCapture      func(*authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
	buildMembership          func(context.Context, *authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
	buildMemberPointLimits   func(context.Context, *config.Config, *gorm.DB, MembershipDependencies, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildMemberResources     func(context.Context, *config.Config, *gorm.DB, *gorm.DB, MembershipDependencies, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildAccountAudit        func(*gorm.DB, *gorm.DB, *gorm.DB, invocationAuditSources, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildAccountProfile      func(*gorm.DB) (kernelmodule.Module, error)
	buildAccountIdentity     func(*config.Config) (kernelmodule.Module, error)
	buildSubjectVerification func(*gorm.DB, *config.Config) (kernelmodule.Module, error)
}

type CurrentApplicationOption func(*currentApplicationOptions)
type currentApplicationOptions struct {
	knowledgeServices       int
	knowledge               *knowledge.Service
	storeCenters            int
	storeCenterDB           *gorm.DB
	officialStoreProvider   storecenter.OfficialConnectionProvider
	officialStoreProtection storecenter.OfficialCredentialProtection
	officialStoreConfigs    int
	runtimeContext          context.Context
	commercialOwnerDB       *gorm.DB
	moneyOwnerDB            *gorm.DB
	referralDB              *gorm.DB
	productAcquisitionDB    *gorm.DB
	imageAgentDB            *gorm.DB
	imageAgentWorkflows     imageagent.WorkflowClient
	membership              *MembershipDependencies
	referrals               int
	productAcquisitions     int
	imageAgents             int
	memberships             int
	browserCaptures         int
	productAgent            *ProductAgentDependencies
	productAgents           int
}

// WithRuntimeContext supplies the long-lived application context for bounded
// background recovery. Construction itself continues to use the caller's
// bounded startup context.
func WithRuntimeContext(ctx context.Context) CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.runtimeContext = ctx }
}

// WithCommercialOwnerDatabase supplies the independently owned commercial
// pool used by commercial billing and the canonical resource owner.
func WithCommercialOwnerDatabase(db *gorm.DB) CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.commercialOwnerDB = db }
}
func WithMoneyOwnerDatabase(db *gorm.DB) CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.moneyOwnerDB = db }
}

// WithBrowserCapture enables the #399 exact-click browser capture ingress. It
// shares the independently owned product pool supplied by WithProductAcquisition.
func WithBrowserCapture() CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.browserCaptures++ }
}

// WithReferrals supplies an independently owned pool. The caller closes it.
func WithReferrals(db *gorm.DB) CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.referrals++; options.referralDB = db }
}

// WithProductAcquisition supplies the independently owned product pool.
func WithProductAcquisition(db *gorm.DB) CurrentApplicationOption {
	return func(options *currentApplicationOptions) {
		options.productAcquisitions++
		options.productAcquisitionDB = db
	}
}

// WithAcquisitionImageAgent enables only the receipt-backed, single-main-image
// current Product entry. The caller owns the ImageAgent pool and Temporal client.
func WithAcquisitionImageAgent(db *gorm.DB, workflows imageagent.WorkflowClient) CurrentApplicationOption {
	return func(options *currentApplicationOptions) {
		options.imageAgents++
		options.imageAgentDB = db
		options.imageAgentWorkflows = workflows
	}
}

// WithMembership supplies the independently owned membership receipt pool and provider credentials.
func WithMembership(deps MembershipDependencies) CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.memberships++; options.membership = &deps }
}

func defaultCurrentApplicationFactories(ctx context.Context, projectIDs ...string) currentApplicationFactories {
	projectID := ""
	if len(projectIDs) > 0 {
		projectID = projectIDs[0]
	}
	return currentApplicationFactories{
		buildWorkbench:   buildDefaultWorkbenchContextModule,
		buildStoreCenter: buildCurrentStoreCenterModule,
		buildSourceAccount: func(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			if err := sourceaccountstore.VerifyRuntimePermissions(ctx, db); err != nil {
				return nil, err
			}
			return buildSourceAccountModule(ctx, db, authorizer)
		},
		buildCommercial: func(resourceDB, storeDB *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return buildUnifiedCommercialRead(ctx, resourceDB, storeDB, authorizer)
		},
		buildCommercialBilling:   buildCommercialBillingModule,
		buildCommercialResources: buildCommercialResourcesModule,
		buildAccountAudit: func(sourceDB, membershipDB, resourceDB *gorm.DB, sources invocationAuditSources, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return buildAccountAuditModule(ctx, sourceDB, membershipDB, resourceDB, sources, authorizer, projectID)
		},
		buildAccountProfile: func(db *gorm.DB) (kernelmodule.Module, error) { return buildAccountProfileModule(db) },
		buildSubjectVerification: func(db *gorm.DB, cfg *config.Config) (kernelmodule.Module, error) {
			return buildSubjectVerificationModule(ctx, db, cfg)
		},
		buildAccountIdentity: func(cfg *config.Config) (kernelmodule.Module, error) {
			return accountIdentityModule{client: zitadelruntime.NewSelfServiceClient(cfg.ListingKit.Zitadel.IssuerURL, &http.Client{Timeout: 5 * time.Second})}, nil
		},
		buildMemberPointLimits: buildMemberPointLimitModule,
		buildMemberResources:   buildMemberResourcesModule,
	}
}

// NewCurrentApplication assembles the admitted RUN-1 application shell. The
// caller owns both existing database pools, the listener and server lifecycle.
// Construction does not migrate, seed, repair or invoke default legacy feature
// composition.
func NewCurrentApplication(ctx context.Context, sourceAccountDB *gorm.DB, cfg *config.Config, logger *logrus.Logger) (*http.Server, error) {
	return NewCurrentApplicationWithOptions(ctx, sourceAccountDB, cfg, logger)
}

func NewCurrentApplicationWithOptions(ctx context.Context, sourceAccountDB *gorm.DB, cfg *config.Config, logger *logrus.Logger, options ...CurrentApplicationOption) (*http.Server, error) {
	if ctx == nil || cfg == nil {
		return nil, errors.New("current application startup context unavailable")
	}
	return buildCurrentApplication(ctx, sourceAccountDB, cfg, logger, defaultCurrentApplicationFactories(ctx, cfg.ListingKit.Zitadel.ProjectID), options...)
}

func buildCurrentApplication(ctx context.Context, sourceAccountDB *gorm.DB, cfg *config.Config, logger *logrus.Logger, factories currentApplicationFactories, options ...CurrentApplicationOption) (*http.Server, error) {
	if sourceAccountDB == nil || cfg == nil || logger == nil || !cfg.Workbench.Enabled {
		return nil, errors.New("current application dependencies unavailable")
	}
	if factories.buildWorkbench == nil || factories.buildSourceAccount == nil || factories.buildCommercial == nil {
		return nil, errors.New("current application factories unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("current application startup canceled: %w", err)
	}
	var supplied currentApplicationOptions
	for _, option := range options {
		if option == nil {
			return nil, errors.New("current application option unavailable")
		}
		option(&supplied)
	}
	if supplied.storeCenters > 1 || supplied.referrals > 1 || supplied.productAcquisitions > 1 || supplied.imageAgents > 1 || supplied.memberships > 1 || supplied.productAgents > 1 {
		return nil, errors.New("current application feature pool supplied more than once")
	}
	if supplied.knowledgeServices > 1 || supplied.knowledgeServices > 0 && supplied.knowledge == nil {
		return nil, errors.New("knowledge service unavailable or supplied more than once")
	}
	if supplied.storeCenters > 0 {
		if supplied.storeCenterDB == nil || factories.buildStoreCenter == nil || supplied.commercialOwnerDB == nil {
			return nil, errors.New("store center dependencies unavailable")
		}
		others := []*gorm.DB{sourceAccountDB, supplied.commercialOwnerDB, supplied.moneyOwnerDB, supplied.referralDB, supplied.productAcquisitionDB, supplied.imageAgentDB}
		if supplied.membership != nil {
			others = append(others, supplied.membership.ReceiptDB)
		}
		if supplied.productAgent != nil {
			others = append(others, supplied.productAgent.RunDB, supplied.productAgent.AssetDB, supplied.productAgent.ReviewDB)
		}
		for _, db := range others {
			if supplied.storeCenterDB == db {
				return nil, errors.New("store center requires independent pools")
			}
		}
	}
	if supplied.commercialOwnerDB != nil && (supplied.commercialOwnerDB == sourceAccountDB) {
		return nil, errors.New("commercial owner requires an independent pool")
	}
	if supplied.productAgent != nil && supplied.productAcquisitionDB == nil {
		return nil, errors.New("product agent requires current acquisition owner")
	}
	if supplied.productAcquisitionDB != nil && (supplied.productAcquisitionDB == sourceAccountDB) {
		return nil, errors.New("product acquisition requires an independent pool")
	}
	if supplied.imageAgents > 0 && (supplied.imageAgentDB == nil || supplied.imageAgentWorkflows == nil || supplied.productAcquisitionDB == nil || supplied.imageAgentDB == sourceAccountDB || supplied.imageAgentDB == supplied.productAcquisitionDB || supplied.imageAgentDB == supplied.commercialOwnerDB || supplied.imageAgentDB == supplied.referralDB) {
		return nil, errors.New("acquisition image agent requires its owner pool, organization workflow, and product acquisition pool")
	}
	if supplied.referralDB != nil && (supplied.referralDB == sourceAccountDB || supplied.referralDB == supplied.productAcquisitionDB) {
		return nil, errors.New("referrals requires an independent pool")
	}
	if supplied.membership != nil && (supplied.membership.ReceiptDB == nil || supplied.membership.ReceiptDB == sourceAccountDB || supplied.membership.ReceiptDB == supplied.productAcquisitionDB || supplied.membership.ReceiptDB == supplied.referralDB) {
		return nil, errors.New("membership requires an independent receipt pool")
	}
	var consumerCharges *orgresource.ConsumerChargeService
	var modelPointRecovery func(context.Context) (int, error)
	authorizer, err := authz.NewListingKitAuthorizer(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles)
	if err != nil {
		return nil, fmt.Errorf("build current application authorizer: %w", err)
	}
	if supplied.officialStoreConfigs > 1 || (supplied.officialStoreConfigs > 0 && supplied.storeCenters == 0) || (supplied.officialStoreProvider == nil) != (supplied.officialStoreProtection == nil) {
		return nil, errors.New("official Store configuration must accompany its owner and protection")
	}
	if supplied.productAcquisitionDB != nil || supplied.storeCenters > 0 {
		if supplied.commercialOwnerDB == nil {
			return nil, errors.New("resource consumers require their resource owner pool")
		}
		builder := factories.buildResourceCharges
		if builder == nil {
			builder = buildCurrentResourceCharges
		}
		consumerCharges, err = builder(ctx, supplied.productAcquisitionDB, supplied.storeCenterDB, supplied.commercialOwnerDB, authorizer)
		if err != nil {
			return nil, err
		}
		if consumerCharges == nil {
			return nil, errors.New("resource consumer service unavailable")
		}
	}
	if supplied.productAcquisitionDB != nil {
		if supplied.commercialOwnerDB == nil {
			return nil, errors.New("product acquisition requires its resource owner pool")
		}
		if factories.buildAcquisition != nil {
			return nil, errors.New("product acquisition factory and pool cannot both be supplied")
		}
		productDB := supplied.productAcquisitionDB
		factories.buildAcquisition = func(authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
			// The provider is config-gated: with no collector endpoint/credential
			// the existing anonymous public HTTP provider is used unchanged.
			provider, browserService, err := publicAcquisitionProvider(cfg)
			if err != nil {
				return nil, err
			}
			return buildProductAcquisitionModule(ctx, productDB, dependencies, authorizer, provider, browserService, consumerCharges)
		}
	}
	if supplied.browserCaptures > 1 {
		return nil, errors.New("browser capture supplied more than once")
	}
	if supplied.browserCaptures > 0 {
		if supplied.productAcquisitionDB == nil {
			return nil, errors.New("browser capture requires the product acquisition pool")
		}
		if factories.buildBrowserCapture != nil {
			return nil, errors.New("browser capture factory and option cannot both be supplied")
		}
		browserDB := supplied.productAcquisitionDB
		factories.buildBrowserCapture = func(authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
			return buildBrowserCaptureModule(ctx, browserDB, dependencies, authorizer, consumerCharges)
		}
	}
	if supplied.imageAgents > 0 {
		if factories.buildAcquisitionImage != nil {
			return nil, errors.New("acquisition image agent factory and option cannot both be supplied")
		}
		productDB, imageDB, workflows := supplied.productAcquisitionDB, supplied.imageAgentDB, supplied.imageAgentWorkflows
		factories.buildAcquisitionImage = func(authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
			receipts, err := buildPublishedAcquisitionReader(ctx, productDB, cfg, dependencies, authorizer)
			if err != nil {
				return nil, err
			}
			return buildAcquisitionImageModule(ctx, receipts, imageDB, supplied.commercialOwnerDB, workflows, cfg)
		}
	}
	if supplied.membership != nil {
		if factories.buildMembership != nil {
			return nil, errors.New("membership factory and dependencies cannot both be supplied")
		}
		membershipDeps := *supplied.membership
		factories.buildMembership = func(startup context.Context, authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
			return buildMembershipModule(startup, cfg, membershipDeps, authorizer, dependencies)
		}
	}
	workbench, err := factories.buildWorkbench(cfg, logger)
	if err != nil {
		return nil, err
	}
	if workbench.module == nil || workbench.authDependencies == nil {
		return nil, errors.New("current application workbench dependencies unavailable")
	}
	workbench.authDependencies.authorizer = authorizer
	sourceAccount, err := factories.buildSourceAccount(sourceAccountDB, authorizer)
	if err != nil {
		return nil, fmt.Errorf("build current source account module: %w", err)
	}
	commercial, err := factories.buildCommercial(supplied.commercialOwnerDB, supplied.storeCenterDB, authorizer)
	if err != nil {
		return nil, fmt.Errorf("build current commercial module: %w", err)
	}
	modules := []kernelmodule.Module{workbench.module, commercial, sourceAccount}
	if supplied.knowledgeServices > 0 {
		handler, err := knowledgehttp.NewHandler(supplied.knowledge)
		if err != nil {
			return nil, err
		}
		modules = append(modules, knowledgehttp.NewModule(handler))
	}
	if supplied.storeCenters > 0 {
		stores, err := factories.buildStoreCenter(ctx, supplied.storeCenterDB, authorizer, consumerCharges, supplied.officialStoreProvider, supplied.officialStoreProtection)
		if err != nil {
			return nil, fmt.Errorf("build current store center: %w", err)
		}
		if stores == nil {
			return nil, errors.New("current store center unavailable")
		}
		modules = append(modules, stores)
	}
	sms, err := buildZitadelSMSModule(ctx)
	if err != nil {
		return nil, err
	}
	modules = append(modules, sms)
	var resourceRecovery func(context.Context) error
	includeResources := supplied.commercialOwnerDB != nil && factories.buildCommercialResources != nil
	if includeResources {
		resources, err := factories.buildCommercialResources(ctx, supplied.commercialOwnerDB)
		if err != nil {
			return nil, fmt.Errorf("build current commercial resources: %w", err)
		}
		if resources == nil {
			return nil, errors.New("current commercial resources unavailable")
		}
		modules = append(modules, resources)
	}
	var topUpRecovery func(context.Context) error
	// Billing requires both its commercial owner and the canonical money owner.
	if (cfg.WalletTopUp.Alipay.Enabled || cfg.WalletTopUp.WeChat.Enabled) && supplied.moneyOwnerDB == nil {
		return nil, errors.New("wallet top-up requires the canonical money owner pool")
	}
	moneyDB := supplied.moneyOwnerDB
	if supplied.commercialOwnerDB != nil && moneyDB != nil && factories.buildCommercialBilling != nil {
		commercialBilling, billingErr := factories.buildCommercialBilling(ctx, supplied.commercialOwnerDB, moneyDB, authorizer, cfg)
		if billingErr != nil {
			return nil, fmt.Errorf("build current commercial billing module: %w", billingErr)
		}
		if commercialBilling == nil {
			return nil, errors.New("current commercial billing module unavailable")
		}
		modules = append(modules, commercialBilling)
		if typed, ok := commercialBilling.(commercialBillingModule); ok {
			resourceRecovery = typed.reconcileResources
			topUpRecovery = typed.reconcileTopUps
		}
	}
	var referralMaturity func(context.Context, time.Time) error
	var personalKYC personalKYCReader
	if factories.buildSubjectVerification != nil {
		verification, err := factories.buildSubjectVerification(sourceAccountDB, cfg)
		if err != nil {
			return nil, err
		}
		if verification == nil {
			return nil, errors.New("subject verification module unavailable")
		}
		modules = append(modules, verification)
		if verificationModule, ok := verification.(subjectVerificationModule); ok {
			personalKYC = verificationModule.personalKYC
		}
	}
	includeAccountProfile := factories.buildAccountProfile != nil
	if includeAccountProfile {
		accountProfile, profileErr := factories.buildAccountProfile(sourceAccountDB)
		if profileErr != nil {
			return nil, fmt.Errorf("build current account profile module: %w", profileErr)
		}
		if accountProfile == nil {
			return nil, errors.New("current account profile module unavailable")
		}
		modules = append(modules, accountProfile)
		if factories.buildAccountIdentity != nil {
			accountIdentity, identityErr := factories.buildAccountIdentity(cfg)
			if identityErr != nil {
				return nil, fmt.Errorf("build current account identity module: %w", identityErr)
			}
			if accountIdentity == nil {
				return nil, errors.New("current account identity module unavailable")
			}
			modules = append(modules, accountIdentity)
		}
	}
	if factories.buildAcquisition != nil {
		acquisition, err := factories.buildAcquisition(authorizer, *workbench.authDependencies)
		if err != nil {
			return nil, fmt.Errorf("build current product acquisition module: %w", err)
		}
		if acquisition == nil {
			return nil, errors.New("current product acquisition module unavailable")
		}
		modules = append(modules, acquisition)
	}
	if factories.buildAcquisitionImage != nil {
		image, imageErr := factories.buildAcquisitionImage(authorizer, *workbench.authDependencies)
		if imageErr != nil {
			return nil, fmt.Errorf("build current acquisition image module: %w", imageErr)
		}
		if image == nil {
			return nil, errors.New("current acquisition image module unavailable")
		}
		modules = append(modules, image)
	}
	if supplied.productAgent != nil {
		agentConfig := *supplied.productAgent
		agentConfig.Knowledge = supplied.knowledge
		agentConfig.PointAccountingDB = supplied.commercialOwnerDB
		agentModule, agentErr := buildProductAgentModule(ctx, supplied.productAcquisitionDB, *workbench.authDependencies, authorizer, agentConfig, cfg)
		if agentErr != nil {
			return nil, fmt.Errorf("build current product agent: %w", agentErr)
		}
		modules = append(modules, agentModule)
		modelPointRecovery = agentModule.(productAgentModule).recoverPoints
	}
	if factories.buildBrowserCapture != nil {
		browser, err := factories.buildBrowserCapture(authorizer, *workbench.authDependencies)
		if err != nil {
			return nil, fmt.Errorf("build current Browser capture module: %w", err)
		}
		if browser == nil {
			return nil, errors.New("current Browser capture module unavailable")
		}
		modules = append(modules, browser)
	}
	if cfg.Referrals.Enabled {
		if supplied.referralDB == nil {
			return nil, errors.New("referrals dependencies unavailable")
		}
		module, err := buildReferralHTTPModule(ctx, supplied.referralDB, cfg, personalKYC)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
		if referralModule, ok := module.(referralHTTPModule); ok && referralModule.economics != nil {
			referralMaturity = referralModule.economics.Mature
		}
	} else if supplied.referralDB != nil {
		return nil, errors.New("disabled referrals must not receive a pool")
	}
	if factories.buildAccountAudit != nil {
		var audit kernelmodule.Module
		var auditErr error
		var membershipDB *gorm.DB
		if supplied.membership != nil {
			membershipDB = supplied.membership.ReceiptDB
		}
		sources := invocationAuditSources{}
		if supplied.imageAgentDB != nil {
			sources["image"] = supplied.imageAgentDB
		}
		if supplied.productAgent != nil && supplied.productAgent.RunDB != nil {
			sources["product"] = supplied.productAgent.RunDB
		}
		audit, auditErr = factories.buildAccountAudit(sourceAccountDB, membershipDB, supplied.commercialOwnerDB, sources, authorizer)

		if auditErr != nil {
			return nil, fmt.Errorf("build current account audit module: %w", auditErr)
		}
		if audit == nil || reflect.ValueOf(audit).Kind() == reflect.Ptr && reflect.ValueOf(audit).IsNil() {
			return nil, errors.New("current account audit module unavailable")
		}
		modules = append(modules, audit)
	}
	if factories.buildMembership != nil {
		membership, err := factories.buildMembership(ctx, authorizer, *workbench.authDependencies)
		if err != nil {
			return nil, fmt.Errorf("build current membership module: %w", err)
		}
		if membership == nil {
			return nil, errors.New("current membership module unavailable")
		}
		modules = append(modules, membership)
	}
	includeAccountAllocation := false
	includeMemberPoints := factories.buildMemberPointLimits != nil && supplied.membership != nil && supplied.commercialOwnerDB != nil
	if includeMemberPoints {
		points, err := factories.buildMemberPointLimits(ctx, cfg, supplied.commercialOwnerDB, *supplied.membership, authorizer)
		if err != nil {
			return nil, fmt.Errorf("build member AI point limits: %w", err)
		}
		if points == nil {
			return nil, errors.New("member AI point limits unavailable")
		}
		modules = append(modules, points)
	}
	includeMemberResources := factories.buildMemberResources != nil && supplied.membership != nil && supplied.commercialOwnerDB != nil
	if includeMemberResources {
		resources, err := factories.buildMemberResources(ctx, cfg, supplied.commercialOwnerDB, supplied.storeCenterDB, *supplied.membership, authorizer)
		if err != nil {
			return nil, fmt.Errorf("build member resources: %w", err)
		}
		if resources == nil {
			return nil, errors.New("member resources unavailable")
		}
		modules = append(modules, resources)
	}
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, err
	}
	routeFeatures := currentApplicationOptionalRoutes{
		ZitadelSMS:          true,
		StoreCenter:         supplied.storeCenters > 0,
		Knowledge:           supplied.knowledgeServices > 0,
		AcquisitionImage:    factories.buildAcquisitionImage != nil,
		ProductAgent:        supplied.productAgent != nil,
		MemberPoints:        includeMemberPoints,
		MemberResources:     includeMemberResources,
		Resources:           includeResources,
		SubjectVerification: factories.buildSubjectVerification != nil,
	}
	if err := validateCurrentApplicationRoutesInternal(bundle.routes, factories.buildAccountAudit != nil, factories.buildAcquisition != nil, cfg.Referrals.Enabled, factories.buildMembership != nil, includeAccountProfile, includeAccountAllocation, factories.buildBrowserCapture != nil, routeFeatures); err != nil {
		return nil, err
	}
	server := buildCurrentApplicationHTTPServer(bundle.routes, *workbench.authDependencies)
	if resourceRecovery != nil {
		runtimeContext := supplied.runtimeContext
		if runtimeContext == nil {
			runtimeContext = context.Background()
		}
		startCommercialRecoveryLoop(runtimeContext, server, resourceRecovery, 30*time.Second, "resource purchase", logger)
	}
	if referralMaturity != nil {
		// Referral maturity remains independent of channel recovery.
		startReferralMaturityLoop(server, referralMaturity, time.Hour, logger)
	}
	if topUpRecovery != nil {
		runtimeContext := supplied.runtimeContext
		if runtimeContext == nil {
			runtimeContext = context.Background()
		}
		startCommercialRecoveryLoop(runtimeContext, server, topUpRecovery, 30*time.Second, "wallet top-up", logger)
	}
	if consumerCharges != nil || modelPointRecovery != nil {
		runtimeContext := supplied.runtimeContext
		if runtimeContext == nil {
			runtimeContext = context.Background()
		}
		startCommercialRecoveryLoop(runtimeContext, server, func(ctx context.Context) error {
			var failures []error
			if consumerCharges != nil {
				_, err := consumerCharges.RecoverDue(ctx)
				failures = append(failures, err)
			}
			if modelPointRecovery != nil {
				_, err := modelPointRecovery(ctx)
				failures = append(failures, err)
			}
			return errors.Join(failures...)
		}, 30*time.Second, "resource consumers", logger)
	}
	return server, nil
}

func validateCurrentApplicationRoutes(routes []httproute.Descriptor, includeAudit bool, includeReferrals ...bool) error {
	referrals := len(includeReferrals) > 0 && includeReferrals[0]
	return validateCurrentApplicationRoutesWithFeatures(routes, includeAudit, false, referrals, false)
}

func validateCurrentApplicationRoutesForAcquisition(routes []httproute.Descriptor, acquisition bool) error {
	return validateCurrentApplicationRoutesForSourcing(routes, acquisition, false)
}

func validateCurrentApplicationRoutesForSourcing(routes []httproute.Descriptor, acquisition, browser bool) error {
	return validateCurrentApplicationRoutesWithBrowser(routes, false, acquisition, false, false, browser)
}

func validateCurrentApplicationRoutesWithFeatures(routes []httproute.Descriptor, includeAudit, includeAcquisition, includeReferrals, includeMembership bool, includeAllocation ...bool) error {
	allocation := len(includeAllocation) > 0 && includeAllocation[0]
	return validateCurrentApplicationRoutesInternal(routes, includeAudit, includeAcquisition, includeReferrals, includeMembership, false, allocation, false, currentApplicationOptionalRoutes{})
}

func validateCurrentApplicationRoutesWithAccountProfile(routes []httproute.Descriptor, includeAudit, includeAcquisition, includeReferrals, includeMembership bool, includeAllocation ...bool) error {
	allocation := len(includeAllocation) > 0 && includeAllocation[0]
	return validateCurrentApplicationRoutesInternal(routes, includeAudit, includeAcquisition, includeReferrals, includeMembership, true, allocation, false, currentApplicationOptionalRoutes{})
}

func validateCurrentApplicationRoutesWithBrowser(routes []httproute.Descriptor, includeAudit, includeAcquisition, includeReferrals, includeMembership, includeBrowser bool) error {
	return validateCurrentApplicationRoutesInternal(routes, includeAudit, includeAcquisition, includeReferrals, includeMembership, false, false, includeBrowser, currentApplicationOptionalRoutes{})
}

func validateCurrentApplicationRoutesWithBrowserFeatures(routes []httproute.Descriptor, includeAudit, includeAcquisition, includeReferrals, includeMembership, includeBrowser, includeAccountProfile, includeAllocation bool) error {
	return validateCurrentApplicationRoutesInternal(routes, includeAudit, includeAcquisition, includeReferrals, includeMembership, includeAccountProfile, includeAllocation, includeBrowser, currentApplicationOptionalRoutes{})
}

type currentApplicationOptionalRoutes struct {
	Knowledge           bool
	StoreCenter         bool
	Resources           bool
	ZitadelSMS          bool
	SubjectVerification bool
	AcquisitionImage    bool
	ProductAgent        bool
	MemberPoints        bool
	MemberResources     bool
}

func validateCurrentApplicationRoutesInternal(routes []httproute.Descriptor, includeAudit, includeAcquisition, includeReferrals, includeMembership, includeAccountProfile, includeAllocation, includeBrowser bool, optional currentApplicationOptionalRoutes) error {
	admitted := append([]currentApplicationRoute(nil), currentWorkbenchApplicationRoutes...)
	if optional.Knowledge {
		for _, r := range knowledgehttp.Routes(&knowledgehttp.Handler{}) {
			admitted = append(admitted, currentApplicationRoute{Method: r.Method, Path: r.Path})
		}
	}
	if optional.StoreCenter {
		for _, r := range currentStoreCenterRoutes {
			admitted = append(admitted, currentApplicationRoute{Method: r.method, Path: r.path})
		}
	}
	if optional.ZitadelSMS {
		admitted = append(admitted, currentApplicationRoute{Method: http.MethodPost, Path: zitadelSMSPath})
		for _, route := range routes {
			if route.Path == zitadelSMSPath && (route.Module != "zitadel-sms" || route.AuthPolicy != httproute.AuthPolicyPublic || route.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyNone || route.Permission != "" || route.OrganizationTargetResolver != nil || route.RequestTimeout != 30*time.Second || route.Handler == nil) {
				return errors.New("ZITADEL SMS route loses signature-authenticated delivery boundary")
			}
		}
	}
	if includeAccountProfile {
		admitted = append(admitted, currentAccountProfileApplicationRoutes...)
	}
	if includeAcquisition {
		admitted = append(admitted,
			currentApplicationRoute{Method: http.MethodPost, Path: productAcquisitionBase},
			currentApplicationRoute{Method: http.MethodPost, Path: productAcquisitionBase + "/verify"},
			currentApplicationRoute{Method: http.MethodGet, Path: productAcquisitionBase + "/:operation_id"},
			currentApplicationRoute{Method: http.MethodGet, Path: productAcquisitionBase + "/:operation_id/product"},
		)
	}
	if includeBrowser {
		admitted = append(admitted,
			currentApplicationRoute{Method: http.MethodPost, Path: browserCaptureBase},
			currentApplicationRoute{Method: http.MethodPost, Path: browserCaptureBase + "/verify"},
			currentApplicationRoute{Method: http.MethodGet, Path: browserCaptureBase + "/by-key/:key"},
			currentApplicationRoute{Method: http.MethodGet, Path: browserCaptureBase + "/:operation_id"},
		)
	}
	if optional.AcquisitionImage {
		for _, route := range []currentApplicationRoute{
			{Method: http.MethodGet, Path: acquisitionImageBase + "/candidates"},
			{Method: http.MethodPost, Path: acquisitionImageBase},
			{Method: http.MethodGet, Path: acquisitionImageBase + "/runs/:run_id"},
			{Method: http.MethodPost, Path: acquisitionImageBase + "/runs/:run_id/approve"},
		} {
			admitted = append(admitted, route)
		}
	}
	if optional.SubjectVerification {
		for _, route := range (verificationhttp.PersonalHandler{}).Routes() {
			admitted = append(admitted, currentApplicationRoute{Method: route.Method, Path: route.Path})
		}
		admitted = append(admitted, currentApplicationRoute{Method: http.MethodGet, Path: verificationhttp.BasePath}, currentApplicationRoute{Method: http.MethodPost, Path: verificationhttp.BasePath + "/applications"}, currentApplicationRoute{Method: http.MethodPost, Path: verificationhttp.CallbackPath})
		for _, route := range routes {
			if route.Path == verificationhttp.PersonalBasePath || strings.HasPrefix(route.Path, verificationhttp.PersonalBasePath+"/") {
				if route.Module != "subject-verification" || route.AuthPolicy != httproute.AuthPolicyCurrentIdentity || route.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyNone || route.Permission != "" || route.OrganizationTargetResolver != nil {
					return errors.New("personal verification route loses account boundary")
				}
			}
			if route.Path == verificationhttp.BasePath || route.Path == verificationhttp.BasePath+"/applications" {
				if route.Module != "subject-verification" || route.AuthPolicy != httproute.AuthPolicyCurrentIdentity || route.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || route.Permission != authz.PermissionWorkbenchOrganizationMemberManage || route.OrganizationTargetResolver == nil || route.RequestTimeout != 15*time.Second {
					return errors.New("verification route loses live admin boundary")
				}
			}
		}
	}
	if optional.ProductAgent {
		for _, r := range productAgentRoutes(nil) {
			admitted = append(admitted, currentApplicationRoute{Method: r.Method, Path: r.Path})
		}
		for _, r := range productReviewRoutes(nil, nil) {
			if r.Method == http.MethodPost && r.Path == "/api/product/text-proposals" {
				continue
			}
			admitted = append(admitted, currentApplicationRoute{Method: r.Method, Path: r.Path})
		}
	}
	if optional.MemberPoints {
		admitted = append(admitted, currentApplicationRoute{Method: http.MethodGet, Path: memberPointLimitBase}, currentApplicationRoute{Method: http.MethodPut, Path: memberPointLimitBase + "/:member_id"})
	}
	if optional.MemberResources {
		for _, descriptor := range (memberResourcesModule{}).routes() {
			admitted = append(admitted, currentApplicationRoute{Method: descriptor.Method, Path: descriptor.Path})
		}
	}
	expected := make(map[currentApplicationRoute]struct{}, len(admitted))
	if optional.Resources {
		admitted = append(admitted, currentApplicationRoute{Method: http.MethodGet, Path: commercialResourcesPath})
		admitted = append(admitted, currentApplicationRoute{Method: http.MethodGet, Path: commercialResourceEventsPath})
	}
	for _, route := range admitted {
		expected[route] = struct{}{}
	}
	if includeAudit {
		expected[currentApplicationRoute{Method: http.MethodGet, Path: accountAuditPath}] = struct{}{}
	}
	if includeMembership {
		for _, route := range currentMembershipRoutes {
			expected[route] = struct{}{}
		}
		if err := validateMembershipDescriptors(routes); err != nil {
			return err
		}
	}
	if includeAllocation {
		expected[currentApplicationRoute{Method: http.MethodGet, Path: "/api/v1/account/organization/resources/member-allocations"}] = struct{}{}
		expected[currentApplicationRoute{Method: http.MethodPut, Path: "/api/v1/account/organization/resources/member-allocations/:member_id"}] = struct{}{}
	}
	includeCommercialBilling := false
	for _, descriptor := range routes {
		if strings.HasPrefix(descriptor.Path, "/api/v1/workbench/knowledge-") {
			if !optional.Knowledge {
				return errors.New("knowledge feature not admitted")
			}
			if err := validateKnowledgeDescriptor(descriptor); err != nil {
				return err
			}
		}
		if strings.HasPrefix(descriptor.Path, acquisitionImageBase) && (descriptor.Module != "acquisition-main-image" || descriptor.AuthPolicy != httproute.AuthPolicyVerifiedIdentity || descriptor.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || descriptor.RequestTimeout != 30*time.Second || descriptor.Permission != map[bool]string{true: authz.PermissionImageAgentRead, false: authz.PermissionImageAgentWrite}[descriptor.Method == http.MethodGet]) {
			return errors.New("current acquisition image route loses live permission boundary")
		}
		for _, candidate := range currentCommercialBillingApplicationRoutes {
			if descriptor.Method == candidate.Method && descriptor.Path == candidate.Path {
				includeCommercialBilling = true
				break
			}
		}
		if includeCommercialBilling {
			break
		}
	}
	if includeCommercialBilling {
		for _, route := range currentCommercialBillingApplicationRoutes {
			expected[route] = struct{}{}
		}
	}
	referralRoutes := map[currentApplicationRoute]httproute.Descriptor{}
	if includeReferrals {
		for _, descriptor := range (referralHTTPModule{}).routes() {
			key := currentApplicationRoute{Method: descriptor.Method, Path: descriptor.Path}
			expected[key] = struct{}{}
			referralRoutes[key] = descriptor
		}
	}
	if len(routes) != len(expected) {
		return fmt.Errorf("current application route contract mismatch: got %d routes, want %d", len(routes), len(expected))
	}
	seen := make(map[currentApplicationRoute]struct{}, len(routes))
	for _, descriptor := range routes {
		if descriptor.Path == memberResourcesBase || strings.HasPrefix(descriptor.Path, memberResourcesBase+"/") {
			permission := authz.PermissionWorkbenchOrganizationMemberRead
			if descriptor.Method != http.MethodGet {
				permission = authz.PermissionWorkbenchOrganizationMemberManage
			}
			if !optional.MemberResources || descriptor.Module != memberResourcesModuleName || descriptor.AuthPolicy != httproute.AuthPolicyCurrentIdentity || descriptor.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || descriptor.OrganizationTargetResolver == nil || !descriptor.RejectUnreadRequestBody || descriptor.RequestTimeout != 15*time.Second || descriptor.Permission != permission || descriptor.Handler == nil {
				return errors.New("member resources route loses live permission boundary")
			}
		}
		if descriptor.Path == "/api/v1/workbench/stores" || strings.HasPrefix(descriptor.Path, "/api/v1/workbench/stores/") {
			if !optional.StoreCenter {
				return errors.New("store center feature not admitted")
			}
			if err := validateCurrentStoreDescriptor(descriptor); err != nil {
				return err
			}
		}
		if (descriptor.Path == commercialResourcesPath || descriptor.Path == commercialResourceEventsPath) && (descriptor.Module != commercialResourcesModuleName || descriptor.Method != http.MethodGet || descriptor.AuthPolicy != httproute.AuthPolicyCurrentIdentity || descriptor.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || descriptor.OrganizationTargetResolver != nil || descriptor.Permission != authz.PermissionWorkbenchCommercialRead || descriptor.RequestTimeout != 15*time.Second || !descriptor.RejectUnreadRequestBody || descriptor.Handler == nil) {
			return errors.New("commercial resources route loses live read boundary")
		}
		if strings.HasPrefix(descriptor.Path, productAgentBase) && (descriptor.Module != "product-agent" || descriptor.AuthPolicy != httproute.AuthPolicyVerifiedIdentity || descriptor.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || descriptor.Permission != authz.PermissionListingKitAdminWrite || descriptor.RequestTimeout != 2*time.Minute) {
			return errors.New("product agent loses fresh permission boundary")
		}
		if strings.HasPrefix(descriptor.Path, memberPointLimitBase) {
			permission := authz.PermissionWorkbenchOrganizationMemberRead
			if descriptor.Method == http.MethodPut {
				permission = authz.PermissionWorkbenchOrganizationMemberManage
			}
			if descriptor.Module != memberPointLimitModuleName || descriptor.AuthPolicy != httproute.AuthPolicyCurrentIdentity || descriptor.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || descriptor.OrganizationTargetResolver == nil || !descriptor.RejectUnreadRequestBody || descriptor.RequestTimeout != 15*time.Second || descriptor.Permission != permission || descriptor.Handler == nil {
				return errors.New("member AI point limit route loses live permission boundary")
			}
		}
		if descriptor.Path == accountAuditPath && (descriptor.Method != http.MethodGet || descriptor.AuthPolicy != httproute.AuthPolicyVerifiedIdentity || descriptor.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || descriptor.Permission != authz.PermissionWorkbenchSourceAccountRead || descriptor.OrganizationTargetResolver == nil || !descriptor.RejectUnreadRequestBody || descriptor.RequestTimeout != 10*time.Second) {
			return errors.New("current account audit descriptor does not preserve fresh read authorization")
		}
		route := currentApplicationRoute{Method: descriptor.Method, Path: descriptor.Path}
		if want, ok := referralRoutes[route]; ok && (descriptor.AuthPolicy != want.AuthPolicy || descriptor.OrganizationAccessPolicy != want.OrganizationAccessPolicy || route.Path != accountReferralWithdrawalReviewPath && route.Path != accountReferralWithdrawalReviewQueuePath && descriptor.Permission != "" || descriptor.OrganizationTargetResolver != nil || descriptor.RequestTimeout != want.RequestTimeout || descriptor.RejectUnreadRequestBody != want.RejectUnreadRequestBody) {
			return errors.New("referrals descriptor does not preserve its authority boundary")
		}
		if _, ok := expected[route]; !ok {
			return fmt.Errorf("current application route contract contains unadmitted route %s %s", descriptor.Method, descriptor.Path)
		}
		if _, duplicate := seen[route]; duplicate {
			return fmt.Errorf("current application route contract contains duplicate route %s %s", descriptor.Method, descriptor.Path)
		}
		seen[route] = struct{}{}
	}
	return nil
}

func buildReferralHTTPModule(ctx context.Context, db *gorm.DB, cfg *config.Config, personalKYC personalKYCReader) (kernelmodule.Module, error) {
	r := cfg.Referrals
	secrets := r.Prepared
	if db == nil || secrets == nil || secrets.HTTPClient == nil || r.Issuer != cfg.ListingKit.Zitadel.IssuerURL {
		return nil, errors.New("referrals prepared dependencies unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 3*time.Second {
		return nil, context.DeadlineExceeded
	}
	repository, err := referralstore.New(db)
	if err != nil {
		return nil, err
	}
	check, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var indexes int64
	err = db.WithContext(check).Raw(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_namespace n ON n.oid=c.relnamespace
	WHERE n.nspname='public' AND i.indisvalid AND i.indisready AND
	((c.relname='registration_intents_payload_expiry_idx' AND i.indrelid='public.registration_intents'::regclass AND pg_get_indexdef(i.indexrelid,1,true)='completion_expires_at' AND pg_get_expr(i.indpred,i.indrelid)='(ciphertext IS NOT NULL)') OR
	(c.relname='registration_admission_buckets_expiry_idx' AND i.indrelid='public.registration_admission_buckets'::regclass AND pg_get_indexdef(i.indexrelid,1,true)='window_start' AND i.indpred IS NULL))`).Scan(&indexes).Error
	if err != nil || indexes != 2 {
		return nil, errors.New("referrals schema verification failed")
	}
	// Verify the installer's durable identity keys, not merely table access.
	// Serving must never admit a schema where first binding can duplicate.
	var keys int64
	err = db.WithContext(check).Raw(`WITH required(table_name,kind,columns) AS (VALUES
	('referral_codes','p','issuer,subject'),('referral_codes','u','code'),
	('registration_intents','p','id'),('registration_intents','u','issuer,subject'),
	('registration_intents','u','issuer,key_hash'),('registration_intents','u','issuer,email_hash'),
	('referral_relations','p','issuer,subject'),('referral_receipts','p','intent_id'),
	('referral_receipts','u','issuer,subject'),('registration_admission_buckets','p','kind,key,window_start'))
	SELECT count(*) FROM required r WHERE EXISTS(SELECT 1 FROM pg_constraint c
	JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace
	JOIN pg_index i ON i.indexrelid=c.conindid
	WHERE n.nspname='public' AND t.relname=r.table_name AND c.contype::text=r.kind
	AND c.convalidated AND NOT c.condeferrable AND i.indisvalid AND i.indisready
	AND (SELECT string_agg(a.attname,',' ORDER BY k.ordinality) FROM unnest(c.conkey) WITH ORDINALITY k(attnum,ordinality)
	JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.attnum)=r.columns)`).Scan(&keys).Error
	if err != nil || keys != 10 {
		return nil, errors.New("referrals durable key verification failed")
	}
	payoutMethods, err := moneystore.New(db)
	if err != nil {
		return nil, err
	}
	provider, err := zitadelregistration.New(zitadelregistration.Config{Origin: r.ProviderOrigin, LoginOrigin: r.OfficialLoginOrigin, Organization: r.SignupOrganizationID, HTTPClient: secrets.HTTPClient, Token: func(context.Context) (string, error) { return secrets.ProviderToken, nil }})
	if err != nil {
		return nil, err
	}
	service := &registration.Service{Store: repository, Provider: provider, Issuer: r.Issuer, Instance: r.InstanceID, Organization: r.SignupOrganizationID, Now: time.Now, Keys: registration.Keys{Active: r.KeyID, Lookup: secrets.Lookup, Proof: secrets.Proof, Encryption: secrets.Encryption}}
	payoutEncryptionKeys := make(map[string][]byte, len(secrets.Encryption))
	for keyID, key := range secrets.Encryption {
		payoutEncryptionKeys[keyID] = append([]byte(nil), key...)
	}
	// Personal KYC admission is mandatory for new withdrawals. The rules projection
	// intentionally remains v1 until a dual-schema UI is deployed and confirmed.
	return referralHTTPModule{commands: service, economics: repository, ledgerReader: repository, withdrawals: repository, payoutMethods: payoutMethods, payoutMethodWriter: payoutMethods, payoutEncryptionKeys: payoutEncryptionKeys, payoutEncryptionKeyID: r.KeyID, profileReader: zitadelruntime.NewUserInfoClient(r.Issuer, secrets.HTTPClient), withdrawalReplay: repository, personalKYC: personalKYC, settlements: payoutMethods, serviceCredential: secrets.ServiceCredential}, nil
}

func buildCurrentApplicationHTTPServer(routes []httproute.Descriptor, dependencies routeAuthDependencies) *http.Server {
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, routes, dependencies)
	server.ReadTimeout = 32 * time.Second
	server.WriteTimeout = 32 * time.Second
	server.IdleTimeout = 60 * time.Second
	inner := server.Handler
	server.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if isProductAgentHTTPPath(request.URL.Path) {
			// Only this bounded synchronous model path needs a longer response.
			// Header/body read timeouts and all other route deadlines stay intact.
			if err := http.NewResponseController(writer).SetWriteDeadline(time.Now().Add(125 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
				http.Error(writer, "response deadline unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		inner.ServeHTTP(writer, request)
	})
	return server
}
