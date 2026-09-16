package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	registration "task-processor/internal/app/referralregistration"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	a1688 "task-processor/internal/integration/acquisition/a1688"
	referralstore "task-processor/internal/integration/persistence/referral"
	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	"task-processor/internal/integration/zitadelregistration"
	kernelmodule "task-processor/internal/kernel/module"
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

type currentApplicationFactories struct {
	buildWorkbench     workbenchContextModuleBuilder
	buildSourceAccount func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildCommercial    func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildAcquisition   func(*authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
	buildMembership    func(context.Context, *authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
	buildAccountAudit  func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
}

type CurrentApplicationOption func(*currentApplicationOptions)
type currentApplicationOptions struct {
	referralDB           *gorm.DB
	productAcquisitionDB *gorm.DB
	membership           *MembershipDependencies
	referrals            int
	productAcquisitions  int
	memberships          int
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

// WithMembership supplies the independently owned membership receipt pool and provider credentials.
func WithMembership(deps MembershipDependencies) CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.memberships++; options.membership = &deps }
}

func defaultCurrentApplicationFactories(ctx context.Context) currentApplicationFactories {
	return currentApplicationFactories{
		buildWorkbench: buildDefaultWorkbenchContextModule,
		buildSourceAccount: func(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			if err := sourceaccountstore.VerifyRuntimePermissions(ctx, db); err != nil {
				return nil, err
			}
			return buildSourceAccountModule(ctx, db, authorizer)
		},
		buildCommercial: func(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return buildCommercialReadModuleFromDatabase(ctx, db, authorizer)
		},
		buildAccountAudit: func(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return buildAccountAuditModule(ctx, db, authorizer)
		},
	}
}

// NewCurrentApplication assembles the admitted RUN-1 application shell. The
// caller owns both existing database pools, the listener and server lifecycle.
// Construction does not migrate, seed, repair or invoke default legacy feature
// composition.
func NewCurrentApplication(ctx context.Context, sourceAccountDB, commercialDB *gorm.DB, cfg *config.Config, logger *logrus.Logger) (*http.Server, error) {
	return NewCurrentApplicationWithOptions(ctx, sourceAccountDB, commercialDB, cfg, logger)
}

func NewCurrentApplicationWithOptions(ctx context.Context, sourceAccountDB, commercialDB *gorm.DB, cfg *config.Config, logger *logrus.Logger, options ...CurrentApplicationOption) (*http.Server, error) {
	if ctx == nil {
		return nil, errors.New("current application startup context unavailable")
	}
	return buildCurrentApplication(ctx, sourceAccountDB, commercialDB, cfg, logger, defaultCurrentApplicationFactories(ctx), options...)
}

func buildCurrentApplication(ctx context.Context, sourceAccountDB, commercialDB *gorm.DB, cfg *config.Config, logger *logrus.Logger, factories currentApplicationFactories, options ...CurrentApplicationOption) (*http.Server, error) {
	if sourceAccountDB == nil || commercialDB == nil || cfg == nil || logger == nil || !cfg.Workbench.Enabled {
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
	if supplied.referrals > 1 || supplied.productAcquisitions > 1 || supplied.memberships > 1 {
		return nil, errors.New("current application feature pool supplied more than once")
	}
	if supplied.productAcquisitionDB != nil && (supplied.productAcquisitionDB == sourceAccountDB || supplied.productAcquisitionDB == commercialDB) {
		return nil, errors.New("product acquisition requires an independent pool")
	}
	if supplied.referralDB != nil && (supplied.referralDB == sourceAccountDB || supplied.referralDB == commercialDB || supplied.referralDB == supplied.productAcquisitionDB) {
		return nil, errors.New("referrals requires an independent pool")
	}
	if supplied.membership != nil && (supplied.membership.ReceiptDB == nil || supplied.membership.ReceiptDB == sourceAccountDB || supplied.membership.ReceiptDB == commercialDB || supplied.membership.ReceiptDB == supplied.productAcquisitionDB || supplied.membership.ReceiptDB == supplied.referralDB) {
		return nil, errors.New("membership requires an independent receipt pool")
	}
	if supplied.productAcquisitionDB != nil {
		if factories.buildAcquisition != nil {
			return nil, errors.New("product acquisition factory and pool cannot both be supplied")
		}
		productDB := supplied.productAcquisitionDB
		factories.buildAcquisition = func(authorizer *authz.ListingKitAuthorizer, dependencies routeAuthDependencies) (kernelmodule.Module, error) {
			return buildProductAcquisitionModule(ctx, productDB, dependencies, authorizer, a1688.New())
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
	authorizer, err := authz.NewListingKitAuthorizer(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles)
	if err != nil {
		return nil, fmt.Errorf("build current application authorizer: %w", err)
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
	commercial, err := factories.buildCommercial(commercialDB, authorizer)
	if err != nil {
		return nil, fmt.Errorf("build current commercial module: %w", err)
	}
	modules := []kernelmodule.Module{workbench.module, commercial, sourceAccount}
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
	if cfg.Referrals.Enabled {
		if supplied.referralDB == nil {
			return nil, errors.New("referrals dependencies unavailable")
		}
		module, err := buildReferralHTTPModule(ctx, supplied.referralDB, cfg)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	} else if supplied.referralDB != nil {
		return nil, errors.New("disabled referrals must not receive a pool")
	}
	if factories.buildAccountAudit != nil {
		audit, auditErr := factories.buildAccountAudit(sourceAccountDB, authorizer)
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
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, err
	}
	if err := validateCurrentApplicationRoutesWithFeatures(bundle.routes, factories.buildAccountAudit != nil, factories.buildAcquisition != nil, cfg.Referrals.Enabled, factories.buildMembership != nil); err != nil {
		return nil, err
	}
	return buildCurrentApplicationHTTPServer(bundle.routes, *workbench.authDependencies), nil
}

func validateCurrentApplicationRoutes(routes []httproute.Descriptor, includeAudit bool, includeReferrals ...bool) error {
	referrals := len(includeReferrals) > 0 && includeReferrals[0]
	return validateCurrentApplicationRoutesWithFeatures(routes, includeAudit, false, referrals, false)
}

func validateCurrentApplicationRoutesForAcquisition(routes []httproute.Descriptor, acquisition bool) error {
	return validateCurrentApplicationRoutesWithFeatures(routes, false, acquisition, false, false)
}

func validateCurrentApplicationRoutesWithFeatures(routes []httproute.Descriptor, includeAudit, includeAcquisition, includeReferrals, includeMembership bool) error {
	admitted := append([]currentApplicationRoute(nil), currentWorkbenchApplicationRoutes...)
	if includeAcquisition {
		admitted = append(admitted,
			currentApplicationRoute{Method: http.MethodPost, Path: productAcquisitionBase},
			currentApplicationRoute{Method: http.MethodPost, Path: productAcquisitionBase + "/verify"},
			currentApplicationRoute{Method: http.MethodGet, Path: productAcquisitionBase + "/:operation_id"},
		)
	}
	expected := make(map[currentApplicationRoute]struct{}, len(admitted))
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
		if descriptor.Path == accountAuditPath && (descriptor.Method != http.MethodGet || descriptor.AuthPolicy != httproute.AuthPolicyVerifiedIdentity || descriptor.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || descriptor.Permission != authz.PermissionWorkbenchSourceAccountRead || descriptor.OrganizationTargetResolver == nil || !descriptor.RejectUnreadRequestBody || descriptor.RequestTimeout != 10*time.Second) {
			return errors.New("current account audit descriptor does not preserve fresh read authorization")
		}
		route := currentApplicationRoute{Method: descriptor.Method, Path: descriptor.Path}
		if want, ok := referralRoutes[route]; ok && (descriptor.AuthPolicy != want.AuthPolicy || descriptor.OrganizationAccessPolicy != want.OrganizationAccessPolicy || descriptor.Permission != "" || descriptor.OrganizationTargetResolver != nil || descriptor.RequestTimeout != want.RequestTimeout || descriptor.RejectUnreadRequestBody != want.RejectUnreadRequestBody) {
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

func buildReferralHTTPModule(ctx context.Context, db *gorm.DB, cfg *config.Config) (kernelmodule.Module, error) {
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
	provider, err := zitadelregistration.New(zitadelregistration.Config{Origin: r.ProviderOrigin, LoginOrigin: r.OfficialLoginOrigin, Organization: r.SignupOrganizationID, HTTPClient: secrets.HTTPClient, Token: func(context.Context) (string, error) { return secrets.ProviderToken, nil }})
	if err != nil {
		return nil, err
	}
	service := &registration.Service{Store: repository, Provider: provider, Issuer: r.Issuer, Instance: r.InstanceID, Organization: r.SignupOrganizationID, Now: time.Now, Keys: registration.Keys{Active: r.KeyID, Lookup: secrets.Lookup, Proof: secrets.Proof, Encryption: secrets.Encryption}}
	return referralHTTPModule{commands: service, serviceCredential: secrets.ServiceCredential}, nil
}

func buildCurrentApplicationHTTPServer(routes []httproute.Descriptor, dependencies routeAuthDependencies) *http.Server {
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, routes, dependencies)
	server.ReadTimeout = 32 * time.Second
	server.WriteTimeout = 32 * time.Second
	server.IdleTimeout = 60 * time.Second
	inner := server.Handler
	server.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		inner.ServeHTTP(writer, request)
	})
	return server
}
