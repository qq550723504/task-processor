package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	domain "task-processor/internal/organization/membership"
)

func parsePendingPage(u *url.URL) (domain.PendingPageRequest, error) {
	page := domain.PendingPageRequest{Limit: 20}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(u.RawQuery) > 128 || u.ForceQuery {
		return page, domain.ErrInvalidRequest
	}
	for key, values := range query {
		if len(values) != 1 || values[0] == "" {
			return page, domain.ErrInvalidRequest
		}
		switch key {
		case "limit":
			for _, ch := range values[0] {
				if ch < '0' || ch > '9' {
					return page, domain.ErrInvalidRequest
				}
			}
			page.Limit, err = strconv.Atoi(values[0])
		case "after":
			page.After = values[0]
		default:
			return page, domain.ErrInvalidRequest
		}
	}
	if err != nil || !page.Valid() {
		return page, domain.ErrInvalidRequest
	}
	return page, nil
}

func (h *Handler) ListPending(c *gin.Context) {
	if err := validateScope(c); err != nil {
		writeError(c, err)
		return
	}
	page, err := parsePendingPage(c.Request.URL)
	if err != nil {
		writeError(c, err)
		return
	}
	if h.commands == nil {
		writeError(c, domain.ErrUnavailable)
		return
	}
	commands, err := h.commands(c.Request)
	if err != nil {
		writeError(c, err)
		return
	}
	reader, ok := commands.(interface {
		ListPending(context.Context, domain.PendingPageRequest) (domain.PendingPage, error)
	})
	if !ok {
		writeError(c, domain.ErrUnavailable)
		return
	}
	result, err := reader.ListPending(c.Request.Context(), page)
	if err != nil {
		writeError(c, err)
		return
	}
	if c.Request.Context().Err() != nil {
		writeError(c, domain.ErrUnavailable)
		return
	}
	items := make([]gin.H, 0, len(result.Items))
	for _, op := range result.Items {
		items = append(items, operationReceipt(op))
	}
	responseHeaders(c)
	c.JSON(http.StatusOK, gin.H{"schemaVersion": "membership-operations-v1", "userId": result.Scope.ActorID, "organizationId": result.Scope.OrganizationID, "items": items, "next": result.Next})
}

// Pure durable projection. Listing receipts never observes or mutates members.
func operationReceipt(op domain.Operation) gin.H {
	status := "pending"
	switch op.Phase {
	case domain.PhaseDispatched:
		status = "unknown"
	case domain.PhaseCompleted:
		status = "acknowledged"
	case domain.PhaseRejected:
		status = "rejected"
	}
	memberID := op.AuthorizationID
	if op.Kind == domain.CommandInvite && op.Phase == domain.PhaseCompleted && op.Acknowledgment != nil {
		memberID = op.Acknowledgment.ID
	}
	return gin.H{"schemaVersion": "membership-operation-v1", "userId": op.Scope.ActorID, "organizationId": op.Scope.OrganizationID, "id": op.Key, "kind": op.Kind, "step": op.Step, "status": status, "targetUserId": op.TargetUserID, "authorizationId": memberID, "userEvidence": op.UserEvidence, "userAcknowledgment": op.UserAcknowledgment, "acknowledgment": op.Acknowledgment, "observation": "unavailable", "observed": nil}
}
