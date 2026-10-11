package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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
	Knowledge               *productKnowledgeDTO  `json:"knowledge,omitempty"`
}

// A Task may execute only with the title route frozen at T1. Route resolution
// also checks current credential enablement, version, and endpoint identity.
func (a *productAgentApplication) frozenTitleProfileReady(ctx context.Context, scope agent.Scope, request agent.Request) bool {
	if a == nil || a.configuration == nil || a.selectTitleProfile == nil || request.ConfigurationSnapshotRef.Absent() {
		return false
	}
	snapshot, err := a.configuration.LoadSnapshot(ctx, scope, request.ConfigurationSnapshotRef)
	if err != nil || snapshot.Scope != scope || snapshot.ExecutionModelProfile.Validate() != nil {
		return false
	}
	current, err := a.selectTitleProfile(ctx, scope.OrganizationID)
	return err == nil && current.Validate() == nil && current == snapshot.ExecutionModelProfile
}

func (a *aiWorkbenchApplication) taskView(ctx context.Context, scope aiworkbench.Scope, task aiworkbench.BusinessTask, includeKnowledge bool) (workbenchTaskView, error) {
	view := workbenchTaskView{ID: task.ID, ConversationID: task.ConversationID, ProposalID: task.ProposalID,
		Title: task.Title, GoalSummary: task.GoalSummary, CreatedAt: task.CreatedAt}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TenantID != scope.OrganizationID || identity.UserID != scope.ActorID || task.Scope != scope {
		return workbenchTaskView{}, aiworkbench.ErrNotFound
	}
	if a.agent == nil {
		return view, nil
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
	projection, err := a.taskProjectionReader().Read(ctx, scope, task)
	if err != nil {
		return view, nil
	}
	found := projection.Run != nil
	var run agent.Record
	if found {
		run = *projection.Run
		view.AgentRunID, view.AgentPhase, view.AgentRevision = run.State.RunID, run.State.Phase, strconv.FormatUint(run.State.Revision, 10)
	}
	if projection.Review.ID != "" {
		view.ReviewID, view.ReviewState = projection.Review.ID, projection.Review.State
	}
	view.ProjectionAvailable, view.State, view.Reason = true, projection.State, projection.Reason
	if _, err := a.agent.freshIdentity(ctx); err == nil {
		titleReady := true
		if projection.CanStart || projection.CanResume {
			titleReady = a.agent.frozenTitleProfileReady(ctx, agent.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, request)
			if !titleReady {
				view.Reason = "TITLE_PROFILE_UNAVAILABLE"
			}
		}
		view.CanStart = projection.CanStart && view.ProductDetailsAvailable && titleReady
		view.CanReconcile = projection.CanReconcile
		view.CanResume = projection.CanResume && view.ProductDetailsAvailable && titleReady
		view.CanReview = projection.CanReview && view.ProductDetailsAvailable && found && agentRunReviewable(run.State)
	}
	// Knowledge is an independently protected owner projection. Loss of a
	// Product binding must neither hide readable citations nor authorize them.
	if found && includeKnowledge {
		if provenance, err := agentContextProvenance(run.State); err == nil {
			view.Knowledge = a.agent.projectKnowledge(ctx, provenance)
		}
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
		view, err := a.taskView(ctx, scope, task, false)
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
	view, err := a.taskView(ctx, scope, task, true)
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
	key, err := workbenchKey(c)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	input := aiworkbench.TaskActionInput{TaskID: id, Key: key}
	switch action {
	case "task-start":
		input.Action = aiworkbench.TaskActionStart
	case "task-review":
		input.Action = aiworkbench.TaskActionReview
	case "task-resume":
		input.Action = aiworkbench.TaskActionResume
	default:
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	if input.Action == aiworkbench.TaskActionResume {
		var body struct {
			Revision string `json:"revision"`
			Feedback string `json:"feedback"`
		}
		if workbenchJSON(c, &body) != nil {
			writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
			return
		}
		input.Revision, err = strconv.ParseUint(body.Revision, 10, 64)
		if err != nil || input.Revision == 0 || strconv.FormatUint(input.Revision, 10) != body.Revision {
			writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
			return
		}
		input.Feedback = body.Feedback
	} else if workbenchEmptyBody(c) != nil {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	receipt, replay, err := a.store.BeginTaskAction(ctx, scope, input)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	if replay {
		switch receipt.State {
		case aiworkbench.TaskActionComplete:
			view, viewErr := a.taskView(ctx, scope, task, false)
			if viewErr != nil {
				writeAIWorkbenchError(c, aiworkbench.ErrTaskOutcomeUnknown)
				return
			}
			workbenchReply(c, http.StatusOK, gin.H{"task": view, "replay": true})
		case aiworkbench.TaskActionFailed:
			if receipt.ErrorCode == "REVISION_MISMATCH" {
				writeAIWorkbenchError(c, aiworkbench.ErrRevisionMismatch)
			} else {
				writeAIWorkbenchError(c, aiworkbench.ErrUnavailable)
			}
		default:
			writeAIWorkbenchError(c, aiworkbench.ErrTaskOutcomeUnknown)
		}
		return
	}
	crossedOwner, actionErr := a.performTaskAction(ctx, scope, task, input)
	state := aiworkbench.TaskActionComplete
	var code []string
	if actionErr != nil {
		state = aiworkbench.TaskActionUnknown
		if !crossedOwner {
			state = aiworkbench.TaskActionFailed
			failureCode := "DEPENDENCY_UNAVAILABLE"
			if errors.Is(actionErr, aiworkbench.ErrRevisionMismatch) || errors.Is(actionErr, agent.ErrConflict) {
				failureCode = "REVISION_MISMATCH"
				actionErr = aiworkbench.ErrRevisionMismatch
			} else {
				// A second authorization can fail inside Runtime after the
				// Workbench gate. Keep first response and durable replay aligned.
				actionErr = aiworkbench.ErrUnavailable
			}
			code = []string{failureCode}
		}
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	finishErr := a.store.FinishTaskAction(finishCtx, scope, key, state, code...)
	cancel()
	if finishErr != nil || state == aiworkbench.TaskActionUnknown {
		writeAIWorkbenchError(c, aiworkbench.ErrTaskOutcomeUnknown)
		return
	}
	if actionErr != nil {
		writeAIWorkbenchError(c, actionErr)
		return
	}
	view, err := a.taskView(ctx, scope, task, false)
	if err != nil {
		writeAIWorkbenchError(c, aiworkbench.ErrTaskOutcomeUnknown)
		return
	}
	workbenchReply(c, http.StatusOK, gin.H{"task": view, "replay": false})
}

// crossedOwner means the Agent or Review mutation may have started. An error
// after that point cannot authorize automatic same-key redispatch.
func (a *aiWorkbenchApplication) performTaskAction(ctx context.Context, scope aiworkbench.Scope,
	task aiworkbench.BusinessTask, input aiworkbench.TaskActionInput) (crossedOwner bool, err error) {
	if input.Action == aiworkbench.TaskActionStart {
		return (workbenchExecution{agent: a.agent}).startTask(ctx, task)
	}
	var request agent.Request
	if json.Unmarshal(task.ExecutionRequest, &request) != nil || request.Key != task.ExecutionRequestKey {
		return false, aiworkbench.ErrUnavailable
	}
	binding, err := a.agent.binding(ctx, task.OperationID, task.TargetPlatform)
	if err != nil || binding != request.Binding {
		return false, aiworkbench.ErrRevisionMismatch
	}
	run, found, err := a.agent.store.Lookup(ctx, agent.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, binding, request.Key)
	if err != nil || !found {
		return false, aiworkbench.ErrUnavailable
	}
	if run.State.Request != request {
		return false, aiworkbench.ErrRevisionMismatch
	}
	ctx, err = knowledgeRequestContext(ctx)
	if err != nil {
		return false, err
	}
	if input.Action == aiworkbench.TaskActionResume {
		if input.Revision != run.State.Revision {
			return false, aiworkbench.ErrRevisionMismatch
		}
		if !a.agent.frozenTitleProfileReady(ctx, run.State.Scope, request) {
			return false, aiworkbench.ErrUnavailable
		}
		_, claimAttempted, err := a.agent.runtime.ResumeWithClaimAttempt(ctx, request, input.Revision, input.Feedback)
		return claimAttempted, err
	}
	if !agentRunReviewable(run.State) {
		return false, aiworkbench.ErrRevisionMismatch
	}
	reviewInput := agentReviewInput(binding, run.State.Request.PolicyVersion, run.State.Candidate)
	reviewInput.ContextProvenance, err = agentContextProvenance(run.State)
	if err != nil {
		return false, err
	}
	_, transactionAttempted, err := a.agent.reviews.CreateFromCandidateWithTransactionAttempt(ctx, "agent:"+run.State.RunID, reviewInput)
	return transactionAttempted, err
}
