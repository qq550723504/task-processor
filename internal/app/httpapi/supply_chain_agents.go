package httpapi

import (
	"context"
	"errors"
	"strconv"

	"gorm.io/gorm"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/app/productsourcing"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
	"task-processor/internal/integration/commercetoolauth"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/product/sourcing/tools/sourceevidenceinspect"
)

type supplyAgentExecutionKey struct{}
type supplyAgentCommand struct {
	owner  collection.Scope
	record record.TargetRecord
	input  preparation.OperationInput
}

func supplyAgentContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(supplyAgentExecutionKey{}).(supplyAgentCommand)
	return ok
}

type supplyProductAgent struct {
	agent         *productAgentApplication
	app           *supplyapp.Application
	authorization supplyapp.OrganizationExecutionAuthorizer
}
type supplyAgentPrincipal struct {
	identity agent.ExecutionIdentity
	roles    []string
	binding  agent.Binding
	original sourcing.PublicationExecutionScope
}

func (s *supplyProductAgent) current(ctx context.Context) (supplyAgentPrincipal, error) {
	var result supplyAgentPrincipal
	command, ok := ctx.Value(supplyAgentExecutionKey{}).(supplyAgentCommand)
	if !ok || s == nil || s.agent == nil || s.app == nil || command.owner.Validate() != nil {
		return result, review.ErrForbidden
	}
	if _, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx); authenticated {
		return result, review.ErrForbidden
	}
	roles, err := s.authorization.ResolveAgentExecution(ctx, command.owner)
	if err != nil {
		return result, review.ErrForbidden
	}
	allowed := false
	for _, org := range s.agent.config.AllowedOrganizationIDs {
		allowed = allowed || org == command.owner.OrganizationID
	}
	if !allowed || !s.agent.config.Enabled {
		return result, agent.ErrUnavailable
	}
	saved, err := s.app.Records.ReadTargetRecord(ctx, command.owner, command.record.ID)
	if err != nil || collection.Digest(saved) != collection.Digest(command.record) {
		return result, record.ErrConflict
	}
	head, err := s.app.Records.ReadTargetHead(ctx, command.owner, saved.TargetID)
	if err != nil || head.ID != saved.ID || head.Revision != saved.Revision {
		return result, record.ErrConflict
	}
	merchant, rules, err := s.app.Rules.ReadTargetRules(ctx, command.owner, saved.Input.StoreID, saved.Input.Draft)
	if err != nil {
		return result, err
	}
	if merchant != saved.Merchant || collection.Digest(rules) != saved.RulesHash {
		return result, record.ErrConflict
	}
	selected, err := s.app.Sources.SelectForExecution(ctx, command.owner, saved.Source.ID)
	if err != nil {
		return result, err
	}
	scope, source, original, err := selected.Read(ctx)
	if err != nil || scope != command.owner || source != saved.Source {
		return result, record.ErrConflict
	}
	effective, err := s.app.Products.ReadEffectiveTargetProduct(ctx, selected, saved.EffectiveVersion, saved.ApplyReceiptID)
	if err != nil || collection.Digest(effective.Snapshot) != saved.ProductHash {
		return result, record.ErrConflict
	}
	result.identity = agent.ExecutionIdentity{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID}
	result.roles = roles
	result.binding = agent.Binding{ContextKind: "collection", ContextID: source.ID, ProductKey: source.Source.ProductKey, CatalogVersion: strconv.FormatUint(effective.Version, 10), PublicationID: effective.PublicationID, TargetPlatform: "shein"}
	result.original = sourcing.PublicationExecutionScope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, PublicationID: original.PublicationID, ProductKey: original.Identity.ProductKey, CatalogVersion: original.Version}
	return result, nil
}
func (a *productAgentApplication) resolveExecutionIdentity(ctx context.Context) (agent.ExecutionIdentity, error) {
	if a == nil || a.supplyExecution == nil {
		return agent.ExecutionIdentity{}, review.ErrForbidden
	}
	current, err := a.supplyExecution.current(ctx)
	return current.identity, err
}
func (a *productAgentApplication) authorizeSupplyModel(ctx context.Context, org, actor, member string) error {
	identity, err := a.resolveExecutionIdentity(ctx)
	if err != nil || identity.OrganizationID != org || identity.ActorID != actor || identity.MemberID != member {
		return review.ErrForbidden
	}
	return nil
}
func (a *productAgentApplication) authorizeSupplyBinding(ctx context.Context, b agent.Binding) (agent.Scope, error) {
	if a == nil || a.supplyExecution == nil || !supplyAgentContext(ctx) {
		return agent.Scope{}, review.ErrForbidden
	}
	current, err := a.supplyExecution.current(ctx)
	if err != nil {
		return agent.Scope{}, err
	}
	if current.binding != b {
		return agent.Scope{}, agent.ErrConflict
	}
	return agent.Scope{OrganizationID: current.identity.OrganizationID, ActorID: current.identity.ActorID}, nil
}
func (a *productAgentApplication) requestOrExecutionScope(ctx context.Context) (agent.Scope, error) {
	if supplyAgentContext(ctx) {
		i, err := a.resolveExecutionIdentity(ctx)
		return agent.Scope{OrganizationID: i.OrganizationID, ActorID: i.ActorID}, err
	}
	i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok {
		return agent.Scope{}, review.ErrForbidden
	}
	return agent.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}, nil
}

type productAgentPrincipal struct {
	application *productAgentApplication
	request     *commercetoolauth.FreshWorkbenchPrincipalResolver
}

func (r productAgentPrincipal) ResolvePrincipal(ctx context.Context) (commercetool.Principal, error) {
	return r.ResolveFreshPrincipal(ctx)
}
func (r productAgentPrincipal) ResolveFreshPrincipal(ctx context.Context) (commercetool.Principal, error) {
	if supplyAgentContext(ctx) {
		if r.application == nil || r.application.supplyExecution == nil {
			return commercetool.Principal{}, review.ErrForbidden
		}
		current, err := r.application.supplyExecution.current(ctx)
		if err != nil {
			return commercetool.Principal{}, err
		}
		return commercetool.Principal{TenantID: current.identity.OrganizationID, UserID: current.identity.ActorID, Roles: current.roles}, nil
	}
	return r.request.ResolveFreshPrincipal(ctx)
}

type productAgentSourceGateway struct {
	application *productAgentApplication
	request     review.SourcePublicationGateway
}

func (r productAgentSourceGateway) Read(ctx context.Context, id string) (sourcing.PersistedPublication, error) {
	if supplyAgentContext(ctx) {
		if r.application.executionSource == nil {
			return sourcing.PersistedPublication{}, review.ErrForbidden
		}
		return r.application.executionSource.Read(ctx, id)
	}
	return r.request.Read(ctx, id)
}
func (r productAgentSourceGateway) AuthorizeRead(ctx context.Context) (context.Context, error) {
	if supplyAgentContext(ctx) {
		if r.application.executionSource == nil {
			return ctx, review.ErrForbidden
		}
		return r.application.executionSource.AuthorizeRead(ctx)
	}
	return r.request.AuthorizeRead(ctx)
}
func (r productAgentSourceGateway) AuthorizePublicationExecution(ctx context.Context, org, actor string) error {
	if !supplyAgentContext(ctx) || r.application == nil || r.application.executionSource == nil {
		return review.ErrForbidden
	}
	return r.application.executionSource.AuthorizePublicationExecution(ctx, org, actor)
}
func (r productAgentSourceGateway) ReadEffectiveSource(ctx context.Context, principal commercetool.Principal, published catalog.PublishedSnapshot) (sourceevidenceinspect.EffectiveSourceObservation, error) {
	var out sourceevidenceinspect.EffectiveSourceObservation
	if !supplyAgentContext(ctx) || r.application == nil || r.application.supplyExecution == nil || r.application.executionReviewLookup == nil {
		return out, sourcing.ErrPublicationForbidden
	}
	current, err := r.application.supplyExecution.current(ctx)
	if err != nil || current.identity.OrganizationID != principal.TenantID || current.identity.ActorID != principal.UserID || current.binding.ProductKey != published.Identity.ProductKey || current.binding.PublicationID != published.PublicationID || current.binding.CatalogVersion != strconv.FormatUint(published.Version, 10) {
		return out, sourcing.ErrPublicationForbidden
	}
	reader, err := catalogstore.NewBoundedSnapshotReader(r.application.config.ReviewDB, 1<<20)
	if err != nil {
		return out, err
	}
	lineage, err := review.ResolveAppliedSource(ctx, review.Scope{Org: current.identity.OrganizationID, Actor: current.identity.ActorID}, published, reader, r.application.executionSource, r.application.executionReviewLookup)
	if err != nil {
		return out, err
	}
	if lineage.Original.PublicationID != current.original.PublicationID || lineage.Original.Version != current.original.CatalogVersion {
		return out, sourcing.ErrSourcePublicationConflict
	}
	out.Requested = sourceevidenceinspect.CatalogReference{ProductKey: published.Identity.ProductKey, Version: strconv.FormatUint(published.Version, 10), PublicationID: published.PublicationID}
	out.Original = lineage.Source
	for _, applied := range lineage.Applied {
		out.Applied = append(out.Applied, sourceevidenceinspect.AppliedTitleReference{ProposalID: applied.ProposalID, BaseVersion: strconv.FormatUint(applied.BaseVersion, 10), BasePublicationID: applied.BasePublicationID, Version: strconv.FormatUint(applied.Version, 10), PublicationID: applied.PublicationID})
	}
	return out, nil
}
func (a *productAgentApplication) validateSupplyCandidate(ctx context.Context, b agent.Binding, policy string, candidate enrichment.Candidate, history []agent.Observation) (agent.Validation, error) {
	if a.executionReviews == nil {
		return agent.Validation{}, agent.ErrUnavailable
	}
	if !agentCandidateEvidenceObserved(b, candidate, history) {
		return agent.Validation{PolicyVersion: policy, Unresolved: []string{"candidate_evidence_not_observed"}}, nil
	}
	proposal, err := a.executionReviews.ValidateCandidate(ctx, agentReviewInput(b, policy, candidate))
	if err != nil {
		if errors.Is(err, enrichment.ErrEvidenceInsufficient) || errors.Is(err, enrichment.ErrOutputValidation) || errors.Is(err, enrichment.ErrPolicyRejected) || errors.Is(err, review.ErrInvalid) {
			return agent.Validation{PolicyVersion: policy, Unresolved: []string{"title_or_evidence_invalid"}}, nil
		}
		return agent.Validation{}, err
	}
	return agent.Validation{Valid: proposal.Validation.Valid, PolicyVersion: policy}, nil
}
func connectSupplyProductAgent(ctx context.Context, db *gorm.DB, application *supplyapp.Application, a *productAgentApplication, authority supplyapp.OrganizationExecutionAuthorizer) (*supplyProductAgent, error) {
	if a == nil {
		return nil, nil
	}
	bridge := &supplyProductAgent{agent: a, app: application, authorization: authority}
	gateway, err := productsourcing.NewExecutionPublicationGateway(db, func(ctx context.Context) (sourcing.PublicationExecutionScope, error) {
		current, err := bridge.current(ctx)
		return current.original, err
	})
	if err != nil {
		return nil, err
	}
	reader, err := catalogstore.NewBoundedSnapshotReader(db, 1<<20)
	if err != nil {
		return nil, err
	}
	repo, err := reviewstore.NewRepository(db, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	if err != nil {
		return nil, err
	}
	candidate, err := review.NewExecutionCandidateService(reader, gateway, repo, a.authorizer, func(ctx context.Context) (review.CandidateExecutionScope, error) {
		current, err := bridge.current(ctx)
		return review.CandidateExecutionScope{OrganizationID: current.identity.OrganizationID, ActorID: current.identity.ActorID, MemberID: current.identity.MemberID}, err
	})
	if err != nil {
		return nil, err
	}
	a.supplyExecution, a.executionSource, a.executionReviews = bridge, gateway, candidate
	a.executionReviewLookup = repo
	return bridge, nil
}
func (s *supplyProductAgent) Optimize(ctx context.Context, op preparation.Operation, item preparation.OperationItem, key string) (string, error) {
	if s == nil || item.RecordID == "" || op.Input.TitleTemplateID == "" || op.Input.ImageTemplateID != "" {
		return "", record.ErrNotReady
	}
	proof, err := s.app.Operations.AuthorizeExecution(ctx, op.Owner.OrganizationID, op.ID)
	if err != nil {
		return "", err
	}
	durable, err := proof.Read(ctx)
	if err != nil {
		return "", err
	}
	if durable.Owner != op.Owner || collection.Digest(durable.Input) != collection.Digest(op.Input) || key != preparation.ItemCommandID(op.ID, item.SourceID, preparation.OperationOptimize) {
		return "", record.ErrConflict
	}
	fixed, err := s.app.Execution.Repository.BeginOperationItem(ctx, proof, item.SourceID)
	if err != nil {
		return "", err
	}
	if preparation.ItemTerminal(fixed.Status) {
		return fixed.ResultReference, nil
	}
	if fixed.RecordID != item.RecordID || fixed.RecordRevision != item.RecordRevision {
		return "", record.ErrConflict
	}
	saved, err := s.app.Records.ReadTargetRecord(ctx, op.Owner, fixed.RecordID)
	if err != nil {
		return "", err
	}
	if saved.Source.ID != item.SourceID || saved.Input.StoreID != op.Input.StoreID || saved.Revision != fixed.RecordRevision {
		return "", record.ErrConflict
	}
	executionCtx := context.WithValue(ctx, supplyAgentExecutionKey{}, supplyAgentCommand{owner: op.Owner, record: saved, input: op.Input})
	current, err := s.current(executionCtx)
	if err != nil {
		return "", err
	}
	selected := agentconfig.TemplateRef{TemplateID: op.Input.TitleTemplateID, Revision: op.Input.TitleTemplateRevision}
	profile, err := s.validateTitleSelection(executionCtx, agent.Scope{OrganizationID: op.Owner.OrganizationID, ActorID: op.Owner.ActorID}, selected, op.Input.TitleQuoteHash)
	if err != nil {
		return "", err
	}
	request, err := s.agent.startRequestWithProfile(executionCtx, current.binding, key, "", "", &selected, profile, "")
	if err != nil {
		return "", err
	}
	run, err := s.agent.runtime.Start(executionCtx, request)
	if err != nil {
		return "", err
	}
	if !agentRunReviewable(run.State) {
		if run.State.Phase == agent.Running || run.State.PendingInvocationID != "" || run.State.StopReason == agent.StopModelUnknown || run.State.StopReason == agent.StopUsageUnknown || run.State.StopReason == agent.StopExecutionOutcomeUnknown {
			return run.State.RunID, preparation.ErrUnknown
		}
		return "", record.ErrNotReady
	}
	input := agentReviewInput(current.binding, run.State.Request.PolicyVersion, run.State.Candidate)
	input.ContextProvenance, err = agentContextProvenance(run.State)
	if err != nil {
		return "", err
	}
	view, err := s.agent.executionReviews.CreateFromCandidate(executionCtx, "agent:"+run.State.RunID, input)
	return view.ID, err
}

var _ supplyapp.OperationOptimizer = (*supplyProductAgent)(nil)
