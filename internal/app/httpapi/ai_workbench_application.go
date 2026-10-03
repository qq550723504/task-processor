package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/aicapability"
	"task-processor/internal/aiworkbench"
	"task-processor/internal/authidentity"
	governed "task-processor/internal/integration/aicapability/einomodel"
	"task-processor/internal/integration/aiworkbench/einoplanner"
	"task-processor/internal/integration/openai"
	workstore "task-processor/internal/integration/persistence/aiworkbench"
	"task-processor/internal/knowledge"
	"task-processor/internal/product/review"
)

type AIWorkbenchDependencies struct {
	DB                   *gorm.DB
	PlanningTextPolicies map[string]governed.RoutePolicy
}

type aiWorkbenchApplication struct {
	store   *workstore.Store
	service *aiworkbench.Service
	agent   *productAgentApplication
	plan    *workbenchPlanner
}

type workbenchPlanner struct {
	agent  *productAgentApplication
	routes *governed.OrganizationRouteResolver
	model  einoplanner.Planner
}

type titleGatedPlanningText struct {
	executor           *governed.Executor
	selectTitleProfile func(context.Context, string) (aicapability.ModelProfile, error)
}

func (t *titleGatedPlanningText) Generate(ctx context.Context, input aicapability.TextInputIdentity, quote aicapability.TextQuote,
	validate func(string) error) (governed.TextOutput, error) {
	if t == nil || t.executor == nil || t.selectTitleProfile == nil {
		return governed.TextOutput{}, governed.ErrNotDispatched
	}
	return t.executor.GenerateWithGate(ctx, input, quote, validate, func(gateCtx context.Context) (func(), error) {
		if _, err := t.selectTitleProfile(gateCtx, input.OrganizationID); err != nil {
			return nil, governed.ErrNotDispatched
		}
		return nil, nil
	})
}

type workbenchExecution struct{ agent *productAgentApplication }

func (p *workbenchPlanner) AuthorizeReceipt(ctx context.Context, scope aiworkbench.Scope) error {
	i, err := p.agent.freshChatIdentity(ctx)
	if err != nil || i.TenantID != scope.OrganizationID || i.UserID != scope.ActorID {
		return review.ErrForbidden
	}
	return nil
}

func (x workbenchExecution) AuthorizeReceipt(ctx context.Context, scope aiworkbench.Scope) error {
	i, err := x.agent.freshChatIdentity(ctx)
	if err != nil || i.TenantID != scope.OrganizationID || i.UserID != scope.ActorID {
		return review.ErrForbidden
	}
	return nil
}

func buildAIWorkbenchApplication(ctx context.Context, cfg AIWorkbenchDependencies, a *productAgentApplication) (*aiWorkbenchApplication, error) {
	if cfg.DB == nil || a == nil || a.config.RunDB == nil || a.config.Ledger == nil || a.textAdmission == nil || len(cfg.PlanningTextPolicies) == 0 {
		return nil, aiworkbench.ErrUnavailable
	}
	if err := workstore.VerifySchema(ctx, cfg.DB); err != nil {
		return nil, err
	}
	store, err := workstore.New(cfg.DB)
	if err != nil {
		return nil, err
	}
	routes := make(map[governed.RouteKey]governed.RoutePolicy, len(cfg.PlanningTextPolicies))
	for org, policy := range cfg.PlanningTextPolicies {
		allowed := false
		for _, admitted := range a.config.AllowedOrganizationIDs {
			allowed = allowed || admitted == org
		}
		if !allowed || policy.ShapeProfile().Validate() != nil || policy.Profile.PromptVersion != "ai-workbench-chat-plan-v1" ||
			policy.Profile.OutputSchemaVersion != "ai-workbench-plan-decision-v1" || policy.Profile.Currency != a.config.Limits.Currency {
			return nil, aiworkbench.ErrUnavailable
		}
		routes[governed.RouteKey{OrganizationID: org, Operation: aicapability.OperationAIWorkbenchChatPlan}] = policy
	}
	resolver, err := governed.NewOrganizationRouteResolver(openai.NewGormCredentialResolver(a.config.RunDB), routes)
	if err != nil {
		return nil, err
	}
	executor := &governed.Executor{Ledger: a.config.Ledger, Admission: a.textAdmission, Resolve: resolver.Resolve,
		Authorize: func(ctx context.Context, in aicapability.TextInputIdentity) error {
			identity, err := a.freshChatIdentity(ctx)
			if err != nil || identity.TenantID != in.OrganizationID || identity.UserID != in.ActorID || identity.EffectiveMemberID != in.MemberID {
				return review.ErrForbidden
			}
			return nil
		}}
	planner := &workbenchPlanner{agent: a, routes: resolver, model: einoplanner.Planner{Text: &titleGatedPlanningText{
		executor: executor, selectTitleProfile: a.selectTitleProfile,
	}}}
	service := &aiworkbench.Service{Store: store, Plan: planner, Execute: workbenchExecution{agent: a}}
	return &aiWorkbenchApplication{store: store, service: service, agent: a, plan: planner}, nil
}

func (p *workbenchPlanner) Admission(ctx context.Context, scope aiworkbench.Scope, input aiworkbench.MessageInput) (aiworkbench.PlanPreparer, error) {
	identity, err := p.agent.freshChatIdentity(ctx)
	if err != nil || scope != (aiworkbench.Scope{OrganizationID: identity.TenantID, ActorID: identity.UserID}) {
		return nil, review.ErrForbidden
	}
	if _, err = p.agent.bindingForIdentity(ctx, identity, input.OperationID, input.TargetPlatform); err != nil {
		return nil, err
	}
	if input.TemplateID != "" {
		version, parseErr := strconv.ParseUint(input.TemplateRevision, 10, 63)
		if parseErr != nil || version == 0 || strconv.FormatUint(version, 10) != input.TemplateRevision {
			return nil, aiworkbench.ErrInvalid
		}
		template, readErr := p.agent.configuration.ReadTemplate(ctx, agent.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, p.agent.definition.ID, input.TemplateID, version)
		if readErr != nil || template.Lifecycle != "ACTIVE" || template.TargetPlatform != input.TargetPlatform {
			return nil, agentconfig.ErrChanged
		}
	} else if input.TemplateRevision != "" {
		return nil, aiworkbench.ErrInvalid
	}
	if input.KnowledgeBaseID != "" {
		if p.agent.context == nil {
			return nil, knowledge.ErrUnavailable
		}
		knowledgeCtx, bindErr := knowledgeRequestContext(authidentity.WithAuthenticatedIdentity(ctx, identity))
		if bindErr != nil {
			return nil, bindErr
		}
		if _, readErr := p.agent.context.ObserveSelection(knowledgeCtx,
			knowledge.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, input.KnowledgeBaseID); readErr != nil {
			return nil, readErr
		}
	}
	unavailable := func() (aiworkbench.PlanPreparer, error) {
		return func([]aiworkbench.Message, string) (aiworkbench.PreparedPlan, error) {
			return aiworkbench.PreparedPlan{MemberID: identity.EffectiveMemberID, Unavailable: true}, nil
		}, nil
	}
	// The paid planner must not run when its only approved consumer cannot
	// currently form a title proposal. Both checks use execution's route owners.
	if p.agent.selectTitleProfile == nil {
		return unavailable()
	}
	if _, err := p.agent.selectTitleProfile(ctx, scope.OrganizationID); err != nil {
		return unavailable()
	}
	route, routeErr := p.routes.Resolve(ctx, aicapability.TextInputIdentity{OrganizationID: scope.OrganizationID, Operation: aicapability.OperationAIWorkbenchChatPlan})
	if routeErr != nil {
		return unavailable()
	}
	profile := route.Profile
	return func(history []aiworkbench.Message, invocationID string) (aiworkbench.PreparedPlan, error) {
		prepared, err := einoplanner.Prepare(einoplanner.Request{Scope: scope, MemberID: identity.EffectiveMemberID,
			InvocationID: invocationID, OperationID: input.OperationID, TargetPlatform: input.TargetPlatform,
			TemplateID: input.TemplateID, TemplateRevision: input.TemplateRevision, KnowledgeBaseID: input.KnowledgeBaseID,
			History: history, Profile: profile})
		if err != nil {
			return aiworkbench.PreparedPlan{}, err
		}
		raw, err := json.Marshal(profile)
		if err != nil {
			return aiworkbench.PreparedPlan{}, aiworkbench.ErrUnavailable
		}
		return aiworkbench.PreparedPlan{MemberID: identity.EffectiveMemberID, InputHash: prepared.Quote.InputHash,
			ModelProfile: raw, Deadline: time.Now().Add(profile.DeadlineBound)}, nil
	}, nil
}

func (p *workbenchPlanner) Decide(ctx context.Context, command aiworkbench.PlanningCommand, history []aiworkbench.Message) (aiworkbench.PlanTerminal, error) {
	var profile aicapability.ModelProfile
	if ctx == nil || json.Unmarshal(command.ModelProfile, &profile) != nil || profile.Validate() != nil || !time.Now().Before(command.Deadline) {
		return aiworkbench.PlanTerminal{}, governed.ErrNotDispatched
	}
	if p.agent == nil || p.agent.selectTitleProfile == nil {
		return aiworkbench.PlanTerminal{}, governed.ErrNotDispatched
	}
	if _, err := p.agent.selectTitleProfile(ctx, command.Scope.OrganizationID); err != nil {
		return aiworkbench.PlanTerminal{}, governed.ErrNotDispatched
	}
	selected := command.WorkScope
	prepared, err := einoplanner.Prepare(einoplanner.Request{Scope: command.Scope, MemberID: command.MemberID,
		InvocationID: command.PlannerInvocationID, OperationID: selected.OperationID, TargetPlatform: selected.TargetPlatform,
		TemplateID: selected.TemplateID, TemplateRevision: selected.TemplateRevision, KnowledgeBaseID: selected.KnowledgeBaseID,
		History: history, Profile: profile})
	if err != nil || prepared.Quote.InputHash != command.InputHash {
		return aiworkbench.PlanTerminal{}, governed.ErrNotDispatched
	}
	callCtx, cancel := context.WithDeadline(ctx, command.Deadline)
	defer cancel()
	decision, err := p.model.Decide(callCtx, prepared)
	if err != nil {
		return aiworkbench.PlanTerminal{}, err
	}
	terminal := aiworkbench.PlanTerminal{AssistantText: decision.AssistantText, Mode: decision.Mode, GoalSummary: decision.GoalSummary}
	if decision.Mode == aiworkbench.PlanReady {
		proposal, buildErr := p.buildProposal(ctx, command, decision.GoalSummary)
		if buildErr != nil {
			return aiworkbench.PlanTerminal{AssistantText: "当前商品或配置已变化，请刷新选择后重新提交。", Mode: aiworkbench.PlanClarify}, nil
		}
		terminal.Proposal = &proposal
	}
	return terminal, nil
}

func (p *workbenchPlanner) buildProposal(ctx context.Context, c aiworkbench.PlanningCommand, goal string) (aiworkbench.ExecutionProposal, error) {
	i, err := p.agent.freshChatIdentity(ctx)
	if err != nil || i.TenantID != c.Scope.OrganizationID || i.UserID != c.Scope.ActorID {
		return aiworkbench.ExecutionProposal{}, review.ErrForbidden
	}
	binding, err := p.agent.bindingForIdentity(ctx, i, c.WorkScope.OperationID, c.WorkScope.TargetPlatform)
	if err != nil {
		return aiworkbench.ExecutionProposal{}, err
	}
	scope := agent.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}
	configuration, err := p.agent.configuration.ReadAgent(ctx, scope, p.agent.definition.ID)
	if err != nil || configuration.Activation != "ENABLED" {
		return aiworkbench.ExecutionProposal{}, agentconfig.ErrNotEnabled
	}
	profile, err := p.agent.selectTitleProfile(ctx, i.TenantID)
	if err != nil {
		return aiworkbench.ExecutionProposal{}, err
	}
	rawProfile, err := json.Marshal(profile)
	if err != nil {
		return aiworkbench.ExecutionProposal{}, err
	}
	proposal := aiworkbench.ExecutionProposal{Kind: "product.title.optimize", Scope: c.Scope, GoalSummary: goal,
		OperationID: c.WorkScope.OperationID, ProductKey: binding.ProductKey, CatalogVersion: binding.CatalogVersion,
		PublicationID: binding.PublicationID, TargetPlatform: binding.TargetPlatform,
		AgentID: p.agent.definition.ID, AgentVersion: p.agent.definition.Version,
		ObservedAgentRevision: configuration.Revision, ObservedActivationEpoch: configuration.ActivationEpoch,
		TemplateID: c.WorkScope.TemplateID, TemplateRevision: c.WorkScope.TemplateRevision,
		KnowledgeBaseID: c.WorkScope.KnowledgeBaseID, ExecutionModelProfile: rawProfile}
	if proposal.KnowledgeBaseID != "" {
		knowledgeCtx, e := knowledgeRequestContext(authidentity.WithAuthenticatedIdentity(ctx, i))
		if e != nil {
			return aiworkbench.ExecutionProposal{}, e
		}
		selection, e := p.agent.context.ObserveSelection(knowledgeCtx, knowledge.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}, proposal.KnowledgeBaseID)
		if e != nil {
			return aiworkbench.ExecutionProposal{}, e
		}
		proposal.KnowledgeRevisionSetDigest = selection.Digest
	}
	return proposal, nil
}

func (p *workbenchPlanner) FailureState(ctx context.Context, c aiworkbench.PlanningCommand, callErr error) aiworkbench.PlanningState {
	fact, err := p.agent.config.Ledger.ReadModelInvocation(ctx, c.Scope.OrganizationID, c.PlannerInvocationID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if errors.Is(callErr, governed.ErrNotDispatched) && !errors.Is(callErr, governed.ErrOutcomeUnknown) {
			return aiworkbench.PlanningFailedBeforeDispatch
		}
		return aiworkbench.PlanningUnknown
	}
	if errors.Is(callErr, governed.ErrNoDispatchReleased) && err == nil &&
		fact.InvocationID == c.PlannerInvocationID && fact.TenantID == c.Scope.OrganizationID &&
		fact.UserID == c.Scope.ActorID && fact.MemberID == c.MemberID && fact.InputHash == c.InputHash &&
		fact.Operation == aicapability.OperationAIWorkbenchChatPlan && fact.Outcome == aicapability.InvocationFailed &&
		fact.UsageKnown && fact.EstimatedCostKnown && fact.PromptTokens == 0 && fact.CompletionTokens == 0 && fact.TotalTokens == 0 &&
		(fact.ErrorCode == "reservation_failed_before_dispatch" || fact.ErrorCode == "rejected_before_dispatch") {
		return aiworkbench.PlanningFailedBeforeDispatch
	}
	if errors.Is(callErr, governed.ErrObservedInvalidSettled) && err == nil &&
		fact.InvocationID == c.PlannerInvocationID && fact.TenantID == c.Scope.OrganizationID &&
		fact.UserID == c.Scope.ActorID && fact.MemberID == c.MemberID && fact.InputHash == c.InputHash &&
		fact.Operation == aicapability.OperationAIWorkbenchChatPlan &&
		fact.Outcome == aicapability.InvocationUsageObservedFailed &&
		fact.ErrorCategory == aicapability.ErrorStructuredOutputInvalid &&
		fact.UsageKnown && fact.EstimatedCostKnown && fact.TotalTokens > 0 {
		return aiworkbench.PlanningInvalidOutput
	}
	if err == nil && fact.Operation == aicapability.OperationAIWorkbenchChatPlan &&
		time.Now().Before(c.Deadline.Add(30*time.Second)) {
		return aiworkbench.PlanningReadyToDispatch
	}
	return aiworkbench.PlanningUnknown
}

func (x workbenchExecution) Prepare(ctx context.Context, p aiworkbench.ExecutionProposal, key string) (aiworkbench.PreparedTask, error) {
	a := x.agent
	i, err := a.freshIdentity(ctx)
	if err != nil || p.Scope != (aiworkbench.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}) {
		return aiworkbench.PreparedTask{}, review.ErrForbidden
	}
	binding, err := a.binding(ctx, p.OperationID, p.TargetPlatform)
	if err != nil {
		return aiworkbench.PreparedTask{}, err
	}
	if binding.ProductKey != p.ProductKey || binding.CatalogVersion != p.CatalogVersion || binding.PublicationID != p.PublicationID ||
		p.AgentID != a.definition.ID || p.AgentVersion != a.definition.Version {
		return aiworkbench.PreparedTask{}, aiworkbench.ErrRevisionMismatch
	}
	configuration, err := a.configuration.ReadAgent(ctx, agent.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}, p.AgentID)
	if err != nil || configuration.Activation != "ENABLED" || configuration.Revision != p.ObservedAgentRevision || configuration.ActivationEpoch != p.ObservedActivationEpoch {
		return aiworkbench.PreparedTask{}, aiworkbench.ErrRevisionMismatch
	}
	var profile aicapability.ModelProfile
	if json.Unmarshal(p.ExecutionModelProfile, &profile) != nil || profile.Validate() != nil {
		return aiworkbench.PreparedTask{}, aiworkbench.ErrUnavailable
	}
	var template *agentconfig.TemplateRef
	if p.TemplateID != "" {
		template = &agentconfig.TemplateRef{TemplateID: p.TemplateID, Revision: p.TemplateRevision}
	}
	requestCtx := ctx
	if p.KnowledgeBaseID != "" {
		requestCtx, err = knowledgeRequestContext(ctx)
		if err != nil {
			return aiworkbench.PreparedTask{}, err
		}
	}
	request, err := a.startRequestWithProfile(requestCtx, binding, key, p.KnowledgeBaseID,
		p.KnowledgeRevisionSetDigest, template, profile, p.GoalSummary)
	if err != nil {
		return aiworkbench.PreparedTask{}, err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return aiworkbench.PreparedTask{}, aiworkbench.ErrUnavailable
	}
	return aiworkbench.PreparedTask{ProposalDigest: p.Digest, ConfigurationSnapshotRef: request.ConfigurationSnapshotRef,
		ContextSnapshotRef: request.ContextSnapshotRef, ExecutionRequest: raw}, nil
}

func (x workbenchExecution) Start(ctx context.Context, task aiworkbench.BusinessTask) error {
	_, err := x.startTask(ctx, task)
	return err
}

// startTask reports whether the Agent owner mutation may have begun. A known
// pre-owner failure must leave the HTTP action receipt retryable with a new key.
func (x workbenchExecution) startTask(ctx context.Context, task aiworkbench.BusinessTask) (bool, error) {
	a := x.agent
	i, err := a.freshIdentity(ctx)
	if err != nil || task.Scope != (aiworkbench.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}) {
		return false, review.ErrForbidden
	}
	var request agent.Request
	if json.Unmarshal(task.ExecutionRequest, &request) != nil || request.Key != task.ExecutionRequestKey ||
		request.GoalSummary != task.GoalSummary ||
		request.ConfigurationSnapshotRef != task.ConfigurationSnapshotRef || request.ContextSnapshotRef != task.ContextSnapshotRef {
		return false, aiworkbench.ErrUnavailable
	}
	scope := agent.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}
	current, found, err := a.store.Lookup(ctx, scope, request.Binding, request.Key)
	if err != nil {
		return false, err
	}
	if found {
		if current.State.Request != request {
			return false, agent.ErrConflict
		}
		if current.State.Phase == agent.Running && time.Now().After(current.State.Deadline.Add(30*time.Second)) {
			_, err = a.store.FinalizeExpiredRunning(ctx, scope, request.Binding, request.Key, current.State.Revision, time.Now())
			return true, err
		}
		return false, nil
	}
	binding, err := a.binding(ctx, task.OperationID, task.TargetPlatform)
	if err != nil || binding != request.Binding {
		return false, aiworkbench.ErrRevisionMismatch
	}
	if !a.frozenTitleProfileReady(ctx, scope, request) {
		return false, aiworkbench.ErrUnavailable
	}
	if !request.ContextSnapshotRef.Absent() {
		ctx, err = knowledgeRequestContext(ctx)
		if err != nil {
			return false, err
		}
	}
	_, err = a.runtime.Start(ctx, request)
	return true, err
}

var _ aiworkbench.PlanningPort = (*workbenchPlanner)(nil)
var _ aiworkbench.ExecutionPort = workbenchExecution{}
