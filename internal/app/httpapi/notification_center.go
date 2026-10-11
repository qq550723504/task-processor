package httpapi

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	ns "task-processor/internal/integration/persistence/notificationcenter"
	kernelmodule "task-processor/internal/kernel/module"
	n "task-processor/internal/notificationcenter"
	nh "task-processor/internal/notificationcenter/httpapi"
	economics "task-processor/internal/referraleconomics"
	"task-processor/internal/storecenter"
	verification "task-processor/internal/subjectverification"
	"time"
)

// WithNotificationCenter enables the independently owned, pre-installed pool.
func WithNotificationCenter(db *gorm.DB) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.notifications++; o.notificationDB = db }
}

type notificationModule struct{ handler *nh.Handler }

func (notificationModule) Name() string                { return nh.Module }
func (notificationModule) Enabled(*config.Config) bool { return true }
func (m notificationModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(nh.Routes(m.handler)...)
	return nil
}
func buildNotificationModule(ctx context.Context, db *gorm.DB, cfg *config.Config, authorizer *authz.ListingKitAuthorizer, modules []kernelmodule.Module, options currentApplicationOptions) (kernelmodule.Module, error) {
	if cfg == nil || db == nil || options.notifications != 1 {
		return nil, errors.New("notification center requires one independent owner pool")
	}
	if options.ecoservices != nil && db == options.ecoservices.DB || options.supplyChain != nil && db == options.supplyChain.AssetDB {
		return nil, errors.New("notification center cannot share an owner pool")
	}
	for _, other := range []*gorm.DB{options.commercialOwnerDB, options.moneyOwnerDB, options.referralDB, options.productAcquisitionDB, options.imageAgentDB, options.storeCenterDB, options.agentConfigurationDB} {
		if other != nil && db == other {
			return nil, errors.New("notification center cannot share an owner pool")
		}
	}
	if err := ns.VerifySchema(ctx, db); err != nil {
		return nil, err
	}
	repository, err := ns.New(db)
	if err != nil {
		return nil, err
	}
	sources := []n.Source{ns.Announcements{Store: repository}}
	business, e := notificationBusinessSources(modules, options, authorizer)
	if e != nil {
		return nil, e
	}
	sources = append(sources, business...)
	for _, name := range []string{"inventory", "advertising", "opportunity", "report", "merchant-chat", "fulfillment"} {
		sources = append(sources, n.ReaderSource{SourceName: name, Unopened: true})
	}
	return notificationModule{handler: &nh.Handler{Realm: cfg.ListingKit.Zitadel.IssuerURL, Service: &n.Service{Repository: repository, Sources: sources}, PrepareContext: func(ctx context.Context, authorization string) (context.Context, error) {
		ctx = context.WithValue(ctx, notificationBearerContextKey{}, authorization)
		identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
		if identity.EffectiveOrganizationID == "" {
			return ctx, nil
		}
		return (productReviewCapabilityBinder{now: time.Now}).Bind(ctx, authorization)
	}}}, nil
}

func notificationIdentity(ctx context.Context, scope n.Scope, permission string, authorizer *authz.ListingKitAuthorizer) (authidentity.AuthenticatedIdentity, error) {
	i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || i.UserID != scope.Subject || i.TokenExpiresAt.IsZero() || !time.Now().Before(i.TokenExpiresAt) {
		return i, n.ErrForbidden
	}
	if permission != "" {
		if scope.OrganizationID == "" || i.TenantID != scope.OrganizationID {
			return i, n.ErrForbidden
		}
		allowed, e := authz.AuthorizeOrganization(ctx, authorizer, i.UserID, i.TenantID, i.Roles, permission)
		if e != nil {
			return i, n.ErrUnavailable
		}
		if !allowed {
			return i, n.ErrForbidden
		}
	}
	return i, nil
}

func notificationBusinessSources(modules []kernelmodule.Module, options currentApplicationOptions, authorizer *authz.ListingKitAuthorizer) ([]n.Source, error) {
	var sources []n.Source
	connected := map[string]bool{}
	add := func(source n.Source) {
		if !connected[source.Name()] {
			sources = append(sources, source)
			connected[source.Name()] = true
		}
	}
	if options.storeCenterDB != nil {
		repo, e := storecenter.NewMemberScopedStoreRepository(options.storeCenterDB, currentStoreMemberAuthorizer{authorizer: authorizer})
		if e != nil {
			return nil, e
		}
		add(storedNotificationSource(repo, authorizer))
	}
	if options.commercialOwnerDB != nil {
		repo, e := commercialstore.New(options.commercialOwnerDB)
		if e != nil {
			return nil, e
		}
		add(billingNotificationSource(repo, authorizer))
	}
	if options.knowledge != nil {
		add(knowledgeNotificationSource(options.knowledge, authorizer))
	}
	for _, module := range modules {
		if m, ok := module.(productAgentModule); ok && m.application != nil {
			add(reviewNotificationSource(m.application.reviews))
		}
		if m, ok := module.(currentMembershipModule); ok {
			for _, source := range m.notificationSources() {
				add(source)
			}
		}
		if m, ok := module.(referralHTTPModule); ok {
			if reader, ok := m.economics.(economics.NoticeReader); ok {
				for _, source := range referralNotificationSources(reader, authorizer) {
					add(source)
				}
			}
		}
		if m, ok := module.(productAcquisitionModule); ok && m.noticeReader != nil {
			add(acquisitionNotificationSource(m.noticeReader))
		}
		if m, ok := module.(commercialResourcesModule); ok {
			add(m.notificationSource(authorizer))
		}
		if m, ok := module.(memberResourcesModule); ok {
			add(m.notificationSource(authorizer))
		}
		if m, ok := module.(memberPointLimitModule); ok {
			add(m.notificationSource(authorizer))
		}
		if m, ok := module.(subjectVerificationModule); ok {
			if personal, ok := m.personal.Service.(*verification.PersonalService); ok && personal != nil {
				add(verificationNotificationSource("personal-verification", true, func(ctx context.Context, s n.Scope) (verification.NoticeFact, error) {
					return personal.AuthorizedNoticeFact(ctx, s.Subject)
				}, authorizer))
			}
			if company, ok := m.handler.Service.(*verification.Service); ok && company != nil {
				add(verificationNotificationSource("organization-verification", false, func(ctx context.Context, s n.Scope) (verification.NoticeFact, error) {
					return company.AuthorizedNoticeFact(ctx, verification.Actor{OrganizationID: s.OrganizationID, UserID: s.Subject})
				}, authorizer))
			}
		}
		if workbench, ok := module.(aiWorkbenchModule); ok && workbench.application != nil {
			for _, source := range workbench.application.notificationSources() {
				add(source)
			}
			if workbench.application.agent != nil && workbench.application.agent.reviews != nil {
				add(reviewNotificationSource(workbench.application.agent.reviews))
			}
		}
	}
	// Owner modules append their authorized readers here; an absent installed
	// owner is explicitly unavailable instead of an empty successful feed.
	for _, name := range []string{"workbench-plan", "workbench-task", "product-review", "acquisition", "store", "org-resource", "member-resource", "member-limit", "billing", "invitation-admin", "invitation-recipient", "membership", "knowledge", "organization-verification", "personal-verification", "referral-earnings", "referral-withdrawal"} {
		if connected[name] {
			continue
		}
		sources = append(sources, n.ReaderSource{SourceName: name, IsPersonal: name == "personal-verification" || name == "invitation-recipient" || name == "referral-earnings" || name == "referral-withdrawal"})
	}
	return sources, nil
}
