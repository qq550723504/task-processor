package httpapi

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"time"

	"task-processor/internal/agent"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"task-processor/internal/httproute"
	einoruntime "task-processor/internal/integration/agent/eino"
	"task-processor/internal/integration/agent/grsaitext"
	"task-processor/internal/integration/commercetoolauth"
	"task-processor/internal/integration/openai"
	agentstore "task-processor/internal/integration/persistence/agent"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/listing/readiness/tools/readinessinspect"
	"task-processor/internal/product/asset/tools/assetinspect"
	"task-processor/internal/product/catalog/tools/canonicalinspect"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/product/sourcing/tools/sourceevidenceinspect"
	"task-processor/internal/workbenchcontext"

	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
)

// ProductAgentDependencies are supplied by the current application owner. Pools
// and the manager are caller-owned. Construction never installs schema, creates
// entitlements, calls the model or edits a Product.
type ProductAgentDependencies struct {
	RunDB, AssetDB, ReviewDB *gorm.DB
	Manager                  *openai.Manager
	Ledger                   grsaitext.AgentInvocationLedger
	TextPolicy               grsaitext.AgentTextPolicy
	Enabled                  bool
	AllowedOrganizationIDs   []string
	Limits                   agent.Limits
}

type productAgentApplication struct {
	runtime    *einoruntime.Runtime
	store      *agentstore.Store
	reviews    *review.Service
	receipts   sourcing.PublishedAcquisitionReader
	resolver   organizationIdentityResolver
	authorizer *authz.ListingKitAuthorizer
	config     ProductAgentDependencies
	canonical  *canonicalinspect.Invoker
	sources    *sourceevidenceinspect.Invoker
	assets     *assetinspect.Invoker
	readiness  *readinessinspect.Invoker
}

func buildProductAgentApplication(productDB *gorm.DB, receipts sourcing.PublishedAcquisitionReader, resolver organizationIdentityResolver, auth *authz.ListingKitAuthorizer, cfg ProductAgentDependencies) (*productAgentApplication, error) {
	if !cfg.Enabled || len(cfg.AllowedOrganizationIDs) == 0 || !cfg.Limits.Valid() || cfg.Limits.Runtime > 2*time.Minute || cfg.Limits.Steps > 16 || cfg.Limits.ModelCalls > 8 || productDB == nil || cfg.AssetDB == nil || receipts == nil || resolver == nil || auth == nil {
		return nil, agent.ErrUnavailable
	}
	for _, table := range []string{"product_agent_runs", "product_agent_tool_calls"} {
		if cfg.RunDB == nil || !cfg.RunDB.Migrator().HasTable(table) {
			return nil, agent.ErrUnavailable
		}
	}
	if !productReviewSchemaReady(productDB) {
		return nil, agent.ErrUnavailable
	}
	a := &productAgentApplication{receipts: receipts, resolver: resolver, authorizer: auth, config: cfg}
	a.config.AllowedOrganizationIDs = append([]string(nil), cfg.AllowedOrganizationIDs...)
	var err error
	a.store, err = agentstore.New(cfg.RunDB)
	if err != nil {
		return nil, err
	}
	reader, err := catalogstore.NewBoundedSnapshotReader(productDB, 1<<20)
	if err != nil {
		return nil, err
	}
	assets, err := assetstore.NewBoundedApprovedInventoryReader(cfg.AssetDB, 1<<20)
	if err != nil {
		return nil, err
	}
	sources, err := productsourcing.NewInternalProducer(productDB, &productReviewLiveOrganizationAccess{resolver: resolver, now: time.Now}, auth)
	if err != nil {
		return nil, err
	}
	repo, err := reviewstore.NewRepository(productDB, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	if err != nil {
		return nil, err
	}
	a.reviews, err = review.NewCandidateService(reader, sources, repo, auth)
	if err != nil {
		return nil, err
	}
	definitions := []commercetool.Definition{canonicalinspect.Definition(), sourceevidenceinspect.Definition(), assetinspect.Definition(), readinessinspect.Definition()}
	definition := commercetool.AgentDefinition{ID: "product.title.agent", Version: "v1.0.0"}
	for _, tool := range definitions {
		definition.AllowedTools = append(definition.AllowedTools, tool.Ref)
	}
	fresh, err := commercetoolauth.NewFreshWorkbenchPrincipalResolver(commercetoolauth.FreshOrganizationResolverFunc(func(ctx context.Context, _ commercetoolauth.OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
		return a.freshIdentity(ctx)
	}), time.Now)
	if err != nil {
		return nil, err
	}
	permissions, err := commercetoolauth.NewCasbinAuthorizer(auth)
	if err != nil {
		return nil, err
	}
	deps := commercetool.InvocationDependencies{PrincipalResolver: fresh, Authorizer: permissions, Recorder: a.store, Tracer: otel.Tracer("product-agent"), Now: time.Now, AuditTimeout: 2 * time.Second}
	a.canonical, err = canonicalinspect.NewInvoker(reader, definition, deps)
	if err != nil {
		return nil, err
	}
	a.sources, err = sourceevidenceinspect.NewInvoker(reader, sources, definition, deps)
	if err != nil {
		return nil, err
	}
	a.assets, err = assetinspect.NewInvoker(reader, assets, fresh, definition, deps)
	if err != nil {
		return nil, err
	}
	a.readiness, err = readinessinspect.NewInvoker(reader, assets, fresh, definition, deps)
	if err != nil {
		return nil, err
	}
	model, err := grsaitext.NewAgentTextModel(cfg.Manager, cfg.Ledger, cfg.TextPolicy, definition.AllowedTools, a.freshIdentity)
	if err != nil {
		return nil, err
	}
	a.runtime, err = einoruntime.New(einoruntime.Config{Definition: definition, Tools: definitions, Model: model, Gateway: a, Validator: a, Authorizer: a, Store: a.store})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *productAgentApplication) freshIdentity(ctx context.Context) (authidentity.AuthenticatedIdentity, error) {
	original, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	capability, bound := ctx.Value(productReviewCapabilityContextKey{}).(productReviewRequestCapability)
	if !ok || !bound || capability.actorID != original.UserID || capability.effectiveOrganizationID != original.EffectiveOrganizationID || original.TenantID != original.EffectiveOrganizationID || !capability.tokenExpiresAt.After(time.Now()) {
		return authidentity.AuthenticatedIdentity{}, review.ErrForbidden
	}
	allowed := false
	for _, org := range a.config.AllowedOrganizationIDs {
		allowed = allowed || org == original.EffectiveOrganizationID
	}
	if !a.config.Enabled || !allowed {
		return authidentity.AuthenticatedIdentity{}, agent.ErrUnavailable
	}
	identity, err := a.resolver.Resolve(ctx, httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: authidentity.AuthenticatedIdentity{UserID: capability.actorID, HomeOrganizationID: capability.homeOrganizationID, TokenExpiresAt: capability.tokenExpiresAt}, BearerToken: capability.bearerToken, RequestedOrganizationID: capability.effectiveOrganizationID})
	if err != nil || identity.UserID != original.UserID || identity.TenantID != original.TenantID || identity.EffectiveOrganizationID != original.TenantID || !agent.ValidID(identity.EffectiveMemberID) || !a.authorizer.Authorize("", identity.Roles, authz.PermissionListingKitAdminWrite) {
		return authidentity.AuthenticatedIdentity{}, review.ErrForbidden
	}
	return identity, nil
}

func validAgentTargetPlatform(platform string) bool {
	// These keys select existing exact Asset inventories, not marketplace
	// publishing capability or a promise that marketplace rules were evaluated.
	return platform == "shein" || platform == "temu" || platform == "amazon"
}

func (a *productAgentApplication) binding(ctx context.Context, operationID, targetPlatform string) (agent.Binding, error) {
	if !validAgentTargetPlatform(targetPlatform) {
		return agent.Binding{}, agent.ErrInvalid
	}
	i, err := a.freshIdentity(ctx)
	if err != nil {
		return agent.Binding{}, err
	}
	p, err := a.receipts.ReadPublished(authidentity.WithAuthenticatedIdentity(ctx, i), operationID)
	if err != nil {
		return agent.Binding{}, err
	}
	op := p.Result.Operation
	if op.ID != operationID || op.State != sourcing.AcquisitionPublished || op.Scope.OrganizationID != i.TenantID || op.Scope.ActorID != i.UserID || p.Result.Publication == nil {
		return agent.Binding{}, agent.ErrConflict
	}
	r := p.Result.Publication.Receipt
	if r.OrganizationID != i.TenantID || r.ActorID != i.UserID || r.ProductKey != p.Snapshot.Identity.ProductKey || p.Snapshot.Identity.TenantID != i.TenantID || r.CatalogVersion != p.Snapshot.Version || r.CatalogPublicationID != p.Snapshot.PublicationID || !reflect.DeepEqual(p.Result.Publication.Snapshot, p.Snapshot.Snapshot) {
		return agent.Binding{}, agent.ErrConflict
	}
	return agent.Binding{ContextKind: "acquisition", ContextID: operationID, ProductKey: r.ProductKey, CatalogVersion: strconv.FormatUint(r.CatalogVersion, 10), PublicationID: r.CatalogPublicationID, TargetPlatform: targetPlatform}, nil
}

func (a *productAgentApplication) Authorize(ctx context.Context, binding agent.Binding) (agent.Scope, error) {
	exact, err := a.binding(ctx, binding.ContextID, binding.TargetPlatform)
	if err != nil {
		return agent.Scope{}, err
	}
	if exact != binding {
		return agent.Scope{}, agent.ErrConflict
	}
	i, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
	return agent.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}, nil
}

func (a *productAgentApplication) Invoke(ctx context.Context, ref commercetool.ToolRef, meta commercetool.CallMetadata, b agent.Binding) (commercetool.Result, error) {
	if _, err := a.Authorize(ctx, b); err != nil {
		return commercetool.Result{}, err
	}
	i, err := a.freshIdentity(ctx)
	if err != nil {
		return commercetool.Result{}, err
	}
	ctx = authidentity.WithAuthenticatedIdentity(ctx, i)
	capability := ctx.Value(productReviewCapabilityContextKey{}).(productReviewRequestCapability)
	ctx = commercetoolauth.WithOrganizationRequest(ctx, commercetoolauth.OrganizationRequest{Identity: i, BearerToken: capability.bearerToken, RequestedOrganizationID: i.TenantID})
	switch ref {
	case canonicalinspect.Definition().Ref:
		return a.canonical.Invoke(ctx, meta, canonicalinspect.Input{ProductKey: b.ProductKey, CatalogVersion: b.CatalogVersion})
	case sourceevidenceinspect.Definition().Ref:
		return a.sources.Invoke(ctx, meta, sourceevidenceinspect.Input{ProductKey: b.ProductKey, CatalogVersion: b.CatalogVersion})
	case assetinspect.Definition().Ref:
		return a.assets.Invoke(ctx, meta, assetinspect.Input{ProductKey: b.ProductKey, CatalogVersion: b.CatalogVersion, TargetPlatform: b.TargetPlatform})
	case readinessinspect.Definition().Ref:
		return a.readiness.Invoke(ctx, meta, readinessinspect.Input{ProductKey: b.ProductKey, CatalogVersion: b.CatalogVersion, TargetPlatform: b.TargetPlatform})
	default:
		return commercetool.Result{}, agent.ErrInvalid
	}
}

func agentReviewInput(b agent.Binding, policy string, candidate enrichment.Candidate) review.CandidateInput {
	version, _ := strconv.ParseUint(b.CatalogVersion, 10, 64)
	return review.CandidateInput{Base: review.CreateInput{ProductKey: b.ProductKey, BaseVersion: version}, PublicationID: b.PublicationID, PolicyVersion: policy, Candidate: candidate}
}

func (a *productAgentApplication) Validate(ctx context.Context, b agent.Binding, policy string, candidate enrichment.Candidate, history []agent.Observation) (agent.Validation, error) {
	if _, err := a.Authorize(ctx, b); err != nil {
		return agent.Validation{}, err
	}
	i, err := a.freshIdentity(ctx)
	if err != nil {
		return agent.Validation{}, err
	}
	if !agentCandidateEvidenceObserved(b, candidate, history) {
		return agent.Validation{PolicyVersion: policy, Unresolved: []string{"candidate_evidence_not_observed: read canonical source evidence before proposing"}}, nil
	}
	proposal, err := a.reviews.ValidateCandidate(authidentity.WithAuthenticatedIdentity(ctx, i), agentReviewInput(b, policy, candidate))
	if err != nil {
		if errors.Is(err, enrichment.ErrEvidenceInsufficient) || errors.Is(err, enrichment.ErrOutputValidation) || errors.Is(err, enrichment.ErrPolicyRejected) || errors.Is(err, review.ErrInvalid) {
			return agent.Validation{PolicyVersion: policy, Unresolved: []string{"title_or_evidence_invalid"}}, nil
		}
		return agent.Validation{}, err
	}
	return agent.Validation{Valid: proposal.Validation.Valid, PolicyVersion: policy}, nil
}
