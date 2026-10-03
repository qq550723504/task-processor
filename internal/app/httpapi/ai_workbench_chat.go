package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"task-processor/internal/aiworkbench"
)

func (a *aiWorkbenchApplication) listConversations(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	values := c.Request.URL.Query()
	for name, entries := range values {
		if len(entries) != 1 || (name != "after" && name != "limit" && name != "saved" && name != "archived") {
			writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
			return
		}
	}
	if values.Has("after") && len(c.Query("after")) != 32 ||
		(values.Has("saved") && c.Query("saved") != "true") ||
		(values.Has("archived") && c.Query("archived") != "true") || values.Has("saved") && values.Has("archived") {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	limit, err := workbenchSize(c)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	items, next, err := a.store.ListConversations(ctx, scope, c.Query("after"), limit, c.Query("saved") == "true", c.Query("archived") == "true")
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	workbenchReply(c, http.StatusOK, gin.H{"conversations": items, "next": next})
}

func (a *aiWorkbenchApplication) createConversation(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	if c.Request.URL.RawQuery != "" {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	key, err := workbenchKey(c)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	var body struct {
		Favorite bool `json:"favorite"`
	}
	if err = workbenchJSON(c, &body); err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	conversation, replay, err := a.store.Create(ctx, scope, key, aiworkbench.CreateInput{Favorite: body.Favorite})
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	workbenchReply(c, http.StatusOK, gin.H{"conversation": conversation, "replay": replay})
}

func (a *aiWorkbenchApplication) getConversation(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	id := c.Param("conversation_id")
	if !acquisitionHTTPUUID(id) {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	if len(c.Request.URL.Query()) > 2 {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	limit, err := workbenchSize(c)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	var before uint64
	if value := c.Query("before"); value != "" {
		before, err = strconv.ParseUint(value, 10, 63)
		if err != nil || before == 0 || strconv.FormatUint(before, 10) != value {
			writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
			return
		}
	}
	conversation, err := a.store.Get(ctx, scope, id)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	messages, next, err := a.store.ListMessagesPage(ctx, scope, id, before, limit)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	// Only the latest proposal can match the latest USER turn and be confirmed.
	// Older proposal cards are not rendered by the current Conversation view.
	proposals, err := a.store.ListProposals(ctx, scope, id, 1)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	cards := make([]workbenchProposalCard, 0, len(proposals))
	for _, proposal := range proposals {
		cards = append(cards, a.proposalCard(ctx, proposal))
	}
	for {
		page := gin.H{"conversation": conversation, "messages": messages, "proposals": cards, "before": next}
		wire, marshalErr := json.Marshal(page)
		if marshalErr != nil {
			writeAIWorkbenchError(c, aiworkbench.ErrUnavailable)
			return
		}
		if len(wire) <= workbenchResponseMaxBytes {
			c.Header("Cache-Control", "private, no-store")
			c.Data(http.StatusOK, "application/json; charset=utf-8", wire)
			return
		}
		if len(messages) <= 1 {
			writeAIWorkbenchError(c, aiworkbench.ErrUnavailable)
			return
		}
		// The store returns ascending sequences. Omitted older messages must be
		// reachable on the next page, even if the original 50-row page had no
		// database cursor.
		messages = messages[1:]
		next = strconv.FormatUint(messages[0].Sequence, 10)
	}
}

func (a *aiWorkbenchApplication) changeConversation(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	id := c.Param("conversation_id")
	if !acquisitionHTTPUUID(id) || c.Request.URL.RawQuery != "" {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	version := c.GetHeader("If-Match")
	revision, err := strconv.ParseUint(version, 10, 63)
	if err != nil || revision == 0 || strconv.FormatUint(revision, 10) != version {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	var body struct {
		Title    *string `json:"title,omitempty"`
		Favorite *bool   `json:"favorite,omitempty"`
		Archived *bool   `json:"archived,omitempty"`
	}
	if err := workbenchJSON(c, &body); err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	updated, err := a.store.SetMetadata(ctx, scope, id, revision, aiworkbench.MetadataChange{Title: body.Title, Favorite: body.Favorite, Archived: body.Archived})
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	workbenchReply(c, http.StatusOK, gin.H{"conversation": updated})
}

func (a *aiWorkbenchApplication) postMessage(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	id := c.Param("conversation_id")
	if !acquisitionHTTPUUID(id) || c.Request.URL.RawQuery != "" {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	key, err := workbenchKey(c)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	var body workbenchMessageBody
	if err := workbenchJSON(c, &body); err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	command, err := a.service.Message(ctx, scope, id, key, body.input())
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	response := gin.H{"state": command.State, "userMessageId": command.UserMessageID,
		"sourceSequence": command.SourceSequence, "assistantMessageId": command.AssistantMessageID,
		"proposalId": command.ProposalID}
	if command.ProposalID != "" {
		proposal, err := a.store.GetProposal(ctx, scope, command.ProposalID)
		if err != nil {
			writeAIWorkbenchError(c, err)
			return
		}
		response["proposal"] = a.proposalCard(ctx, proposal)
	}
	if accessErr := a.plan.AuthorizeReceipt(ctx, scope); accessErr != nil {
		writeAIWorkbenchError(c, accessErr)
		return
	}
	status := http.StatusOK
	if command.State == aiworkbench.PlanningReadyToDispatch || command.State == aiworkbench.PlanningUnknown {
		status = http.StatusAccepted
	}
	workbenchReply(c, status, response)
}

func (a *aiWorkbenchApplication) confirmProposal(c *gin.Context, ctx context.Context, scope aiworkbench.Scope) {
	conversationID, proposalID := c.Param("conversation_id"), c.Param("proposal_id")
	if !acquisitionHTTPUUID(conversationID) || !acquisitionHTTPUUID(proposalID) || c.Request.URL.RawQuery != "" || workbenchEmptyBody(c) != nil {
		writeAIWorkbenchError(c, aiworkbench.ErrInvalid)
		return
	}
	key, err := workbenchKey(c)
	if err != nil {
		writeAIWorkbenchError(c, err)
		return
	}
	task, replay, err := a.service.Confirm(ctx, scope, conversationID, proposalID, key)
	if err != nil && task.ID == "" {
		writeAIWorkbenchError(c, err)
		return
	}
	view, viewErr := a.taskView(ctx, scope, task)
	if viewErr != nil {
		writeAIWorkbenchError(c, viewErr)
		return
	}
	if accessErr := (workbenchExecution{agent: a.agent}).AuthorizeReceipt(ctx, scope); accessErr != nil {
		writeAIWorkbenchError(c, accessErr)
		return
	}
	workbenchReply(c, http.StatusOK, gin.H{"task": view, "replay": replay})
}
