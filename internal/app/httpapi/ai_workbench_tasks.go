package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/agent"
	"task-processor/internal/aiworkbench"
	"task-processor/internal/authidentity"
)

type workbenchTaskView struct {
	ID                      string                `json:"id"`
	ConversationID          string                `json:"conversationId"`
	ProposalID              string                `json:"proposalId"`
	Title                   string                `json:"title"`
	GoalSummary             string                `json:"goalSummary"`
	CreatedAt               time.Time             `json:"createdAt"`
	ProjectionAvailable     bool                  `json:"projectionAvailable"`
	State                   aiworkbench.TaskState `json:"state,omitempty"`
	Reason                  string                `json:"reason,omitempty"`
	CanStart                bool                  `json:"canStart"`
	CanReconcile            bool                  `json:"canReconcile"`
	CanResume               bool                  `json:"canResume"`
	CanReview               bool                  `json:"canReview"`
	ProductDetailsAvailable bool                  `json:"productDetailsAvailable"`
	OperationID             string                `json:"operationId,omitempty"`
	ProductKey              string                `json:"productKey,omitempty"`
	TargetPlatform          string                `json:"targetPlatform,omitempty"`
	AgentRunID              string                `json:"agentRunId,omitempty"`
	AgentPhase              agent.Phase           `json:"agentPhase,omitempty"`
	AgentRevision           string                `json:"agentRevision,omitempty"`
	ProviderID              string                `json:"providerId,omitempty"`
	ModelID                 string                `json:"modelId,omitempty"`
	Tokens                  int64                 `json:"tokens,omitempty"`
	EstimatedCostMicros     int64                 `json:"estimatedCostMicros,omitempty"`
	Currency                string                `json:"currency,omitempty"`
	UsageStatus             string                `json:"usageStatus,omitempty"`
	ReviewID                string                `json:"reviewId,omitempty"`
	ReviewState             string                `json:"reviewState,omitempty"`
}

func (a *aiWorkbenchApplication) taskView(ctx context.Context, scope aiworkbench.Scope, task aiworkbench.BusinessTask) (workbenchTaskView, error) {
	view := workbenchTaskView{ID: task.ID, ConversationID: task.ConversationID, ProposalID: task.ProposalID,
		Title: task.Title, GoalSummary: task.GoalSummary, CreatedAt: task.CreatedAt}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TenantID != scope.OrganizationID || identity.UserID != scope.ActorID || task.Scope != scope {
		return workbenchTaskView{}, aiworkbench.ErrNotFound
	}
	var request agent.Request
	if json.Unmarshal(task.ExecutionRequest, &request) != nil || request.Key != task.ExecutionRequestKey {
		return workbenchTaskView{}, aiworkbench.ErrUnavailable
	}
	binding, productErr := a.agent.bindingForIdentity(ctx, identity, task.OperationID, task.TargetPlatform)
	if productErr == nil && binding == request.Binding {
		view.ProductDetailsAvailable = true
		view.OperationID, view.ProductKey, view.TargetPlatform = task.OperationID, task.ProductKey, task.TargetPlatform
	}
	run, found, err := a.agent.store.Lookup(ctx, agent.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, request.Binding, request.Key)
	if err != nil {
		return view, nil
	}
	var runPtr *agent.Record
	var reviewState string
	if found {
		runPtr = &run
		view.AgentRunID, view.AgentPhase, view.AgentRevision = run.State.RunID, run.State.Phase, strconv.FormatUint(run.State.Revision, 10)
		currentReview, reviewFound, reviewErr := a.agent.reviews.FindAgentReview(ctx, run.State.RunID)
		if reviewErr != nil {
			return view, nil
		}
		if reviewFound {
			reviewState = currentReview.State
			view.ReviewID, view.ReviewState = currentReview.ID, currentReview.State
		}
	}
	projection, err := aiworkbench.ProjectTask(task, runPtr, reviewState, time.Now())
	if err != nil {
		return view, nil
	}
	view.ProjectionAvailable, view.State, view.Reason = true, projection.State, projection.Reason
	if _, err := a.agent.freshIdentity(ctx); err == nil {
		view.CanStart, view.CanReconcile, view.CanResume, view.CanReview = projection.CanStart, projection.CanReconcile, projection.CanResume, projection.CanReview
	}
	if found && view.ProductDetailsAvailable {
		if snapshot, err := a.agent.configuration.LoadSnapshot(ctx, run.State.Scope, run.State.Request.ConfigurationSnapshotRef); err == nil {
			view.ProviderID, view.ModelID = snapshot.ExecutionModelProfile.ProviderID, snapshot.ExecutionModelProfile.ModelID
		}
		view.Currency = run.State.Request.Limits.Currency
		if run.State.PendingInvocationID != "" || run.State.StopReason == agent.StopUsageUnknown ||
			run.State.StopReason == agent.StopModelUnknown || run.State.Phase == agent.Running {
			view.UsageStatus = "unknown_reserved"
		} else {
			view.UsageStatus = "observed"
			view.Tokens, view.EstimatedCostMicros = run.State.Usage.Tokens, run.State.Usage.CostMicros
		}
	}
	return view, nil
}

func (a *aiWorkbenchApplication) listTasks(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	if len(c.Request.URL.Query()) > 2 || c.Query("after") != "" && !acquisitionHTTPUUID(c.Query("after")) {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	limit, err := workbenchSize(c)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	tasks, next, err := a.store.ListTasks(ctx, scope, c.Query("after"), limit)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	views := make([]workbenchTaskView, 0, len(tasks))
	for _, task := range tasks {
		view, err := a.taskView(ctx, scope, task)
		if err != nil {
			writeAIWorkbenchError(c, err)
			return
		}
		views = append(views, view)
	}
	workbenchReply(c, http.StatusOK, gin.H{"tasks": views, "next": next})
}

func (a *aiWorkbenchApplication) getTask(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	id := c.Param("task_id")
	if !acquisitionHTTPUUID(id) || c.Request.URL.RawQuery != "" {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	task, err := a.store.GetTask(ctx, scope, id)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	view, err := a.taskView(ctx, scope, task)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	workbenchReply(c, http.StatusOK, gin.H{"task": view})
}

func (a *aiWorkbenchApplication) taskAction(c *gin.Context, ctx context.Context, scope aiworkbench.Scope, action string) {
	id := c.Param("task_id")
	if !acquisitionHTTPUUID(id) || c.Request.URL.RawQuery != "" {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	task, err := a.store.GetTask(ctx, scope, id)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	if _, err := a.agent.freshIdentity(ctx); err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	if action == "task-start" {
		if err := workbenchEmptyBody(c); err != nil {
			writeAIWorkbenchError(c, err)
			return
		}
		err = (workbenchExecution{agent: a.agent}).Start(ctx, task)
	} else {
		var request agent.Request
		if json.Unmarshal(task.ExecutionRequest, &request) != nil || request.Key != task.ExecutionRequestKey {
			writeAIWorkbenchError(c, aiworkbench.ErrUnavailable)
			return
		}
		binding, e := a.agent.binding(ctx, task.OperationID, task.TargetPlatform)
		if e != nil || binding != request.Binding {
			writeAIWorkbenchError(c, aiworkbench.ErrRevisionMismatch)
			return
		}
		run, found, e := a.agent.store.Lookup(ctx, agent.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, binding, request.Key)
		if e != nil || !found {
			writeAIWorkbenchError(c, aiworkbench.ErrUnavailable)
			return
		}
		ctx, e = knowledgeRequestContext(ctx)
		if e != nil {
			writeAIWorkbenchError(c, e)
			return
		}
		if action == "task-resume" {
			var body struct {
				Revision string `json:"revision"`
				Feedback string `json:"feedback"`
			}
			if workbenchJSON(c, &body) != nil {
				writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
				return
			}
			revision, parseErr := strconv.ParseUint(body.Revision, 10, 64)
			if parseErr != nil || revision == 0 || strconv.FormatUint(revision, 10) != body.Revision || revision != run.State.Revision {
				writeAIWorkbenchError(c, aiworkbench.ErrRevisionMismatch)
				return
			}
			_, err = a.agent.runtime.Resume(ctx, request, revision, body.Feedback)
		} else {
			if workbenchEmptyBody(c) != nil {
				writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
				return
			}
			if !agentRunReviewable(run.State) {
				writeAIWorkbenchError(c, aiworkbench.ErrRevisionMismatch)
				return
			}
			input := agentReviewInput(binding, run.State.Request.PolicyVersion, run.State.Candidate)
			input.ContextProvenance, err = agentContextProvenance(run.State)
			if err == nil {
				_, err = a.agent.reviews.CreateFromCandidate(ctx, "agent:"+run.State.RunID, input)
			}
		}
	}
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	view, err := a.taskView(ctx, scope, task)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	workbenchReply(c, http.StatusOK, gin.H{"task": view})
}
