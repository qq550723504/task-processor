package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	sigjson "sigs.k8s.io/json"

	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/aicapability"
	"task-processor/internal/aiworkbench"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	governed "task-processor/internal/integration/aicapability/einomodel"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/knowledge"
	"task-processor/internal/product/review"
)

const workbenchChatBase = "/api/v1/workbench/chat/conversations"
const workbenchTaskBase = "/api/v1/workbench/tasks"

type aiWorkbenchModule struct {
	routes      []httproute.Descriptor
	application *aiWorkbenchApplication
}

func (aiWorkbenchModule) Name() string                { return "ai-workbench" }
func (aiWorkbenchModule) Enabled(*config.Config) bool { return true }
func (m aiWorkbenchModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}

func (m aiWorkbenchModule) PlanningReadiness(ctx context.Context, organizationID string) string {
	if m.application == nil || m.application.plan == nil || m.application.plan.routes == nil ||
		m.application.agent == nil || m.application.agent.titleRoutes == nil {
		return "UNAVAILABLE"
	}
	planning := m.application.plan.routes.Readiness(ctx, aicapability.TextInputIdentity{
		OrganizationID: organizationID, Operation: aicapability.OperationAIWorkbenchChatPlan,
	})
	title := governed.RouteReadiness(m.TitleReadiness(ctx, organizationID))
	if planning == governed.RouteUnavailable || title == governed.RouteUnavailable {
		return string(governed.RouteUnavailable)
	}
	if planning == governed.RouteAvailable && title == governed.RouteAvailable {
		return string(governed.RouteAvailable)
	}
	return string(governed.RouteNeedsConfiguration)
}

// TitleReadiness is the current execution route projection for an existing
// proposal. It is independent of whether a new Chat plan may be started.
func (m aiWorkbenchModule) TitleReadiness(ctx context.Context, organizationID string) string {
	if m.application == nil || m.application.agent == nil || m.application.agent.titleRoutes == nil {
		return string(governed.RouteUnavailable)
	}
	return string(m.application.agent.titleRoutes.Readiness(ctx, aicapability.TextInputIdentity{
		OrganizationID: organizationID, Operation: aicapability.OperationProductAgentDecision,
	}))
}

func WithAIWorkbench(deps AIWorkbenchDependencies) CurrentApplicationOption {
	return func(o *currentApplicationOptions) { o.aiWorkbench = &deps; o.aiWorkbenches++ }
}

func buildAIWorkbenchModule(ctx context.Context, cfg AIWorkbenchDependencies, agentRuntime *productAgentApplication) (kernelmodule.Module, error) {
	app, err := buildAIWorkbenchApplication(ctx, cfg, agentRuntime)
	if err != nil {
		return nil, err
	}
	return aiWorkbenchModule{routes: aiWorkbenchRoutes(app), application: app}, nil
}

func workbenchJSON(c *gin.Context, target any) error {
	media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) ||
		c.GetHeader("Content-Encoding") != "" || c.Request.Body == nil {
		return aiworkbench.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 16<<10+1))
	if err != nil || len(raw) == 0 || len(raw) > 16<<10 || !utf8.Valid(raw) {
		return aiworkbench.ErrInvalid
	}
	violations, err := sigjson.UnmarshalStrict(raw, target, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil || len(violations) != 0 {
		return aiworkbench.ErrInvalid
	}
	return nil
}

func workbenchEmptyBody(c *gin.Context) error {
	if c.Request.Body == nil {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	if err != nil || len(raw) != 0 {
		return aiworkbench.ErrInvalid
	}
	return nil
}

func workbenchKey(c *gin.Context) (string, error) {
	values := c.Request.Header.Values("Idempotency-Key")
	if len(values) != 1 || !acquisitionHTTPUUID(values[0]) {
		return "", aiworkbench.ErrInvalid
	}
	return values[0], nil
}

func workbenchSize(c *gin.Context) (int, error) {
	value := c.Query("limit")
	if value == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 50 || strconv.Itoa(n) != value {
		return 0, aiworkbench.ErrInvalid
	}
	return n, nil
}

func (a *aiWorkbenchApplication) bind(c *gin.Context, permission string) (context.Context, aiworkbench.Scope, error) {
	ctx, err := (productReviewCapabilityBinder{now: time.Now}).Bind(c.Request.Context(), c.GetHeader("Authorization"))
	if err != nil {
		return nil, aiworkbench.Scope{}, err
	}
	i, err := a.agent.freshWorkbenchIdentity(ctx, permission)
	if err != nil {
		return nil, aiworkbench.Scope{}, err
	}
	ctx = authidentity.WithAuthenticatedIdentity(ctx, i)
	return ctx, aiworkbench.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}, nil
}

func writeAIWorkbenchError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, aiworkbench.ErrInvalid), errors.Is(err, agent.ErrInvalid), errors.Is(err, agentconfig.ErrInvalid):
		status, code = http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, aiworkbench.ErrNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, aiworkbench.ErrIdempotencyConflict), errors.Is(err, agentconfig.ErrConflict):
		status, code = http.StatusConflict, "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, aiworkbench.ErrRevisionMismatch), errors.Is(err, agentconfig.ErrChanged), errors.Is(err, agentconfig.ErrRevision), errors.Is(err, knowledge.ErrSelectionChanged):
		status, code = http.StatusConflict, "PROPOSAL_STALE"
	case errors.Is(err, aiworkbench.ErrConversationArchived):
		status, code = http.StatusConflict, "CONVERSATION_ARCHIVED"
	case errors.Is(err, review.ErrForbidden), errors.Is(err, agentconfig.ErrForbidden):
		status, code = http.StatusForbidden, "FORBIDDEN"
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(status, gin.H{"code": code})
}

const workbenchResponseMaxBytes = 256 << 10

func workbenchReply(c *gin.Context, status int, value any) {
	wire, err := json.Marshal(value)
	if err != nil || len(wire) > workbenchResponseMaxBytes {
		writeAIWorkbenchError(c, aiworkbench.ErrUnavailable)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Data(status, "application/json; charset=utf-8", wire)
}

type workbenchMessageBody struct {
	Content          string `json:"content"`
	OperationID      string `json:"operationId"`
	TargetPlatform   string `json:"targetPlatform"`
	TemplateID       string `json:"templateId,omitempty"`
	TemplateRevision string `json:"templateRevision,omitempty"`
	KnowledgeBaseID  string `json:"knowledgeBaseId,omitempty"`
}

func (b workbenchMessageBody) input() aiworkbench.MessageInput {
	return aiworkbench.MessageInput{Content: b.Content, OperationID: b.OperationID,
		TargetPlatform: b.TargetPlatform, TemplateID: b.TemplateID,
		TemplateRevision: b.TemplateRevision, KnowledgeBaseID: b.KnowledgeBaseID}
}

type workbenchProposalCard struct {
	ID                  string `json:"id"`
	Digest              string `json:"digest"`
	SourceSequence      uint64 `json:"sourceSequence"`
	GoalSummary         string `json:"goalSummary"`
	OperationID         string `json:"operationId,omitempty"`
	ProductKey          string `json:"productKey,omitempty"`
	TargetPlatform      string `json:"targetPlatform,omitempty"`
	TemplateID          string `json:"templateId,omitempty"`
	TemplateRevision    string `json:"templateRevision,omitempty"`
	KnowledgeBaseID     string `json:"knowledgeBaseId,omitempty"`
	ProviderID          string `json:"providerId,omitempty"`
	ModelID             string `json:"modelId,omitempty"`
	MaximumTokens       int64  `json:"maximumTokens,omitempty"`
	MaximumCostMicros   int64  `json:"maximumCostMicros,omitempty"`
	Currency            string `json:"currency,omitempty"`
	HumanReviewRequired bool   `json:"humanReviewRequired"`
	DetailsAvailable    bool   `json:"detailsAvailable"`
	TitleProfileReady   bool   `json:"titleProfileReady"`
}

func (a *aiWorkbenchApplication) proposalCard(ctx context.Context, p aiworkbench.ExecutionProposal) workbenchProposalCard {
	card := workbenchProposalCard{ID: p.ID, Digest: p.Digest, SourceSequence: p.SourceSequence,
		GoalSummary: p.GoalSummary, HumanReviewRequired: true}
	i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok {
		return card
	}
	binding, err := a.agent.bindingForIdentity(ctx, i, p.OperationID, p.TargetPlatform)
	if err != nil || binding.ProductKey != p.ProductKey || binding.CatalogVersion != p.CatalogVersion || binding.PublicationID != p.PublicationID {
		return card
	}
	card.DetailsAvailable = true
	card.OperationID, card.ProductKey, card.TargetPlatform = p.OperationID, p.ProductKey, p.TargetPlatform
	card.TemplateID, card.TemplateRevision, card.KnowledgeBaseID = p.TemplateID, p.TemplateRevision, p.KnowledgeBaseID
	var profile aicapability.ModelProfile
	if json.Unmarshal(p.ExecutionModelProfile, &profile) == nil && profile.Validate() == nil {
		cost, _ := profile.MaximumCost()
		card.ProviderID, card.ModelID, card.Currency = profile.ProviderID, profile.ModelID, profile.Currency
		card.MaximumTokens = profile.MaximumPromptTokens + profile.MaximumCompletionTokens
		card.MaximumCostMicros = cost
		if a.agent.selectTitleProfile != nil && i.TenantID == p.Scope.OrganizationID && i.UserID == p.Scope.ActorID {
			current, currentErr := a.agent.selectTitleProfile(ctx, p.Scope.OrganizationID)
			card.TitleProfileReady = currentErr == nil && current.Validate() == nil && current == profile
		}
	}
	return card
}

func aiWorkbenchRoutes(a *aiWorkbenchApplication) []httproute.Descriptor {
	type routeSpec struct {
		method, path, permission, action string
		timeout                          time.Duration
	}
	specs := []routeSpec{
		{"GET", workbenchChatBase, authz.PermissionWorkbenchChatRead, "list-conversations", 15 * time.Second},
		{"POST", workbenchChatBase, authz.PermissionWorkbenchChatUse, "create", 15 * time.Second},
		{"GET", workbenchChatBase + "/:conversation_id", authz.PermissionWorkbenchChatRead, "get-conversation", 15 * time.Second},
		{"PATCH", workbenchChatBase + "/:conversation_id", authz.PermissionWorkbenchChatUse, "metadata", 15 * time.Second},
		{"POST", workbenchChatBase + "/:conversation_id/messages", authz.PermissionWorkbenchChatUse, "message", 2 * time.Minute},
		{"POST", workbenchChatBase + "/:conversation_id/proposals/:proposal_id/confirm", authz.PermissionWorkbenchChatUse, "confirm", 2 * time.Minute},
		{"GET", workbenchTaskBase, authz.PermissionWorkbenchTaskRead, "list-tasks", 15 * time.Second},
		{"GET", workbenchTaskBase + "/:task_id", authz.PermissionWorkbenchTaskRead, "get-task", 15 * time.Second},
		{"POST", workbenchTaskBase + "/:task_id/start", authz.PermissionWorkbenchAgentUse, "task-start", 2 * time.Minute},
		{"POST", workbenchTaskBase + "/:task_id/resume", authz.PermissionWorkbenchAgentUse, "task-resume", 2 * time.Minute},
		{"POST", workbenchTaskBase + "/:task_id/review", authz.PermissionWorkbenchAgentUse, "task-review", 2 * time.Minute},
	}
	routes := make([]httproute.Descriptor, 0, len(specs))
	for _, spec := range specs {
		spec := spec
		routes = append(routes, httproute.Descriptor{Method: spec.method, Path: spec.path,
			Module: "ai-workbench", Permission: spec.permission, AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
			OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: spec.timeout,
			Handler: func(c *gin.Context) {
				if a == nil {
					writeAIWorkbenchError(c, aiworkbench.ErrUnavailable)
					return
				}
				ctx, scope, err := a.bind(c, spec.permission)
				if err != nil {
					writeAIWorkbenchError(c, err)
					return
				}
				if c.Request.Method == http.MethodGet && workbenchEmptyBody(c) != nil {
					writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
					return
				}
				switch spec.action {
				case "list-conversations":
					a.listConversations(c, ctx, scope)
				case "create":
					a.createConversation(c, ctx, scope)
				case "get-conversation":
					a.getConversation(c, ctx, scope)
				case "metadata":
					a.changeConversation(c, ctx, scope)
				case "message":
					a.postMessage(c, ctx, scope)
				case "confirm":
					a.confirmProposal(c, ctx, scope)
				case "list-tasks":
					a.listTasks(c, ctx, scope)
				case "get-task":
					a.getTask(c, ctx, scope)
				case "task-start", "task-resume", "task-review":
					a.taskAction(c, ctx, scope, spec.action)
				}
			}})
	}
	return routes
}
