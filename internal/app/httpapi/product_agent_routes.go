package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/agent"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const productAgentBase = productAcquisitionBase + "/:operation_id/product-agent/runs"

func isProductAgentHTTPPath(path string) bool {
	if !strings.HasPrefix(path, productAcquisitionBase+"/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, productAcquisitionBase+"/"), "/")
	if len(parts) < 3 || !acquisitionHTTPUUID(parts[0]) || parts[1] != "product-agent" || parts[2] != "runs" {
		return false
	}
	return len(parts) == 3 || len(parts) == 4 && acquisitionHTTPUUID(parts[3]) || len(parts) == 5 && acquisitionHTTPUUID(parts[3]) && (parts[4] == "resume" || parts[4] == "review")
}

type productAgentModule struct{ routes []httproute.Descriptor }

func (productAgentModule) Name() string                { return "product-agent" }
func (productAgentModule) Enabled(*config.Config) bool { return true }
func (m productAgentModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(m.routes...)
	return nil
}

func WithProductAgent(deps ProductAgentDependencies) CurrentApplicationOption {
	return func(options *currentApplicationOptions) { options.productAgent = &deps; options.productAgents++ }
}

func buildProductAgentModule(ctx context.Context, productDB *gorm.DB, deps routeAuthDependencies, auth *authz.ListingKitAuthorizer, cfg ProductAgentDependencies) (kernelmodule.Module, error) {
	receipts, err := buildPublishedAcquisitionReader(ctx, productDB, deps, auth)
	if err != nil {
		return nil, err
	}
	a, err := buildProductAgentApplication(cfg.ReviewDB, receipts, deps.organizationResolver, auth, cfg)
	if err != nil {
		return nil, err
	}
	routes := productAgentRoutes(a)
	// Candidate intake returns an ID understood by the existing review UI.
	// Its original decision/Apply handlers retain their current permission checks.
	binder := productReviewCapabilityBinder{now: time.Now}
	for _, route := range productReviewRoutes(a.reviews, binder.Bind) {
		// A missing fixed generator is an unavailable capability, never a fake
		// model. Keep this existing POST unmounted in the candidate-only module.
		if route.Method == http.MethodPost && route.Path == "/api/product/text-proposals" {
			continue
		}
		routes = append(routes, route)
	}
	return productAgentModule{routes: routes}, nil
}

type productAgentStepDTO struct {
	Step         int    `json:"step"`
	Tool         string `json:"tool,omitempty"`
	CallID       string `json:"callId,omitempty"`
	InvocationID string `json:"invocationId,omitempty"`
	AuditStatus  string `json:"auditStatus,omitempty"`
}
type productAgentResultDTO struct {
	RunID               string                  `json:"runId"`
	RequestKey          string                  `json:"requestKey"`
	OperationID         string                  `json:"operationId"`
	ProductKey          string                  `json:"productKey"`
	CatalogVersion      string                  `json:"catalogVersion"`
	PublicationID       string                  `json:"publicationId"`
	TargetPlatform      string                  `json:"targetPlatform"`
	Phase               agent.Phase             `json:"phase"`
	Revision            string                  `json:"revision"`
	StopReason          agent.StopReason        `json:"stopReason,omitempty"`
	HumanReviewRequired bool                    `json:"humanReviewRequired"`
	Candidate           enrichment.Candidate    `json:"candidate"`
	Confidence          []agent.FieldConfidence `json:"confidence"`
	Unresolved          []string                `json:"unresolved"`
	Steps               []productAgentStepDTO   `json:"steps"`
	Tokens              int64                   `json:"tokens"`
	EstimatedCostMicros int64                   `json:"estimatedCostMicros"`
	Currency            string                  `json:"currency"`
	UsageStatus         string                  `json:"usageStatus"`
	CanSubmitReview     bool                    `json:"canSubmitReview"`
}

func productAgentRoutes(a *productAgentApplication) []httproute.Descriptor {
	specs := []struct{ method, path, action string }{{"POST", productAgentBase, "start"}, {"GET", productAgentBase + "/:request_key", "read"}, {"POST", productAgentBase + "/:request_key/resume", "resume"}, {"POST", productAgentBase + "/:request_key/review", "review"}}
	var routes []httproute.Descriptor
	for _, spec := range specs {
		spec := spec
		routes = append(routes, httproute.Descriptor{Method: spec.method, Path: spec.path, Module: "product-agent", Permission: authz.PermissionListingKitAdminWrite, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, RequestTimeout: 2 * time.Minute, Handler: func(c *gin.Context) {
			if a == nil {
				writeProductAgentError(c, agent.ErrUnavailable)
				return
			}
			operationID := c.Param("operation_id")
			key := c.Param("request_key")
			if spec.action == "start" {
				values := c.Request.Header.Values("Idempotency-Key")
				if len(values) != 1 {
					writeProductAgentError(c, agent.ErrInvalid)
					return
				}
				key = values[0]
			}
			if !acquisitionHTTPUUID(operationID) || !acquisitionHTTPUUID(key) || c.Request.URL.RawQuery != "" {
				writeProductAgentError(c, agent.ErrInvalid)
				return
			}
			ctx, err := (productReviewCapabilityBinder{now: time.Now}).Bind(c.Request.Context(), c.GetHeader("Authorization"))
			if err != nil {
				writeProductAgentError(c, err)
				return
			}
			i, err := a.freshIdentity(ctx)
			if err != nil {
				writeProductAgentError(c, err)
				return
			}
			ctx = authidentity.WithAuthenticatedIdentity(ctx, i)
			var body struct {
				Revision       string `json:"revision,omitempty"`
				Feedback       string `json:"feedback,omitempty"`
				TargetPlatform string `json:"targetPlatform,omitempty"`
			}
			if spec.action == "read" {
				if err = readProductReviewGETBody(c); err != nil {
					writeProductAgentError(c, err)
					return
				}
			} else {
				if err = readAcquisitionImageJSON(c.Request, &body); err != nil || (spec.action != "resume" && (body.Revision != "" || body.Feedback != "")) || (spec.action != "start" && body.TargetPlatform != "") {
					writeProductAgentError(c, agent.ErrInvalid)
					return
				}
			}
			var record agent.Record
			var binding agent.Binding
			if spec.action == "start" {
				binding, err = a.binding(ctx, operationID, body.TargetPlatform)
				if err == nil {
					request := agent.Request{Key: key, Binding: binding, PolicyVersion: "title-review-v1", PromptVersion: "product-title-agent-v1", Limits: a.config.Limits}
					record, err = a.runtime.Start(ctx, request)
				}
			} else {
				record, err = a.store.Read(ctx, agent.Scope{OrganizationID: i.TenantID, ActorID: i.UserID}, operationID, key)
				if err == nil {
					binding, err = a.binding(ctx, operationID, record.State.Request.Binding.TargetPlatform)
				}
				if err == nil && record.State.Request.Binding != binding {
					err = agent.ErrConflict
				}
				if err == nil && spec.action == "resume" {
					revision, parseErr := strconv.ParseUint(body.Revision, 10, 64)
					if parseErr != nil || revision == 0 || strconv.FormatUint(revision, 10) != body.Revision {
						err = agent.ErrInvalid
					} else {
						record, err = a.runtime.Resume(ctx, record.State.Request, revision, body.Feedback)
					}
				}
			}
			if err != nil {
				writeProductAgentError(c, err)
				return
			}
			if spec.action == "review" {
				if !agentRunReviewable(record.State) {
					writeProductAgentError(c, agent.ErrConflict)
					return
				}
				view, e := a.reviews.CreateFromCandidate(ctx, "agent:"+record.State.RunID, agentReviewInput(binding, record.State.Request.PolicyVersion, record.State.Candidate))
				if e != nil {
					writeProductAgentError(c, e)
					return
				}
				c.Header("Cache-Control", "no-store")
				c.JSON(http.StatusOK, gin.H{"proposalId": view.ID, "requestKey": key, "operationId": operationID})
				return
			}
			state := record.State
			result := productAgentResultDTO{RunID: state.RunID, RequestKey: key, OperationID: operationID, ProductKey: binding.ProductKey, CatalogVersion: binding.CatalogVersion, PublicationID: binding.PublicationID, TargetPlatform: binding.TargetPlatform, Phase: state.Phase, Revision: strconv.FormatUint(state.Revision, 10), StopReason: state.StopReason, HumanReviewRequired: true, Candidate: state.Candidate, Confidence: state.Confidence, Unresolved: state.Unresolved, Steps: []productAgentStepDTO{}, Tokens: state.Usage.Tokens, EstimatedCostMicros: state.Usage.CostMicros, Currency: state.Request.Limits.Currency, UsageStatus: "observed", CanSubmitReview: agentRunReviewable(state)}
			if state.PendingInvocationID != "" || state.StopReason == agent.StopUsageUnknown || state.StopReason == agent.StopModelUnknown || state.Phase == agent.Running {
				result.UsageStatus = "unknown_reserved"
			}
			for _, step := range state.History {
				result.Steps = append(result.Steps, productAgentStepDTO{Step: step.Step, Tool: step.Tool.ID, CallID: step.CallID, InvocationID: step.InvocationID, AuditStatus: string(step.AuditStatus)})
			}
			wire, e := json.Marshal(result)
			if e != nil || len(wire) > 128<<10 {
				writeProductAgentError(c, agent.ErrUnavailable)
				return
			}
			c.Header("Cache-Control", "no-store")
			c.Data(http.StatusOK, "application/json; charset=utf-8", wire)
		}})
	}
	return routes
}

func agentRunReviewable(s agent.State) bool {
	return s.Phase == agent.HumanReviewRequired && s.Validation != nil && s.Validation.Valid && len(s.Candidate.Changes) == 1 && s.StopReason == ""
}

func writeProductAgentError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "PRODUCT_AGENT_UNAVAILABLE"
	switch {
	case errors.Is(err, agent.ErrInvalid):
		status, code = 400, "INVALID_AGENT_REQUEST"
	case errors.Is(err, review.ErrForbidden):
		status, code = 403, "FORBIDDEN"
	case errors.Is(err, agent.ErrConflict):
		status, code = 409, "AGENT_CONFLICT"
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"code": code})
}
