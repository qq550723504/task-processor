package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	domain "task-processor/internal/organization/membership"
)

type CommandService interface {
	Execute(context.Context, string, domain.CommandInput) (domain.Operation, error)
	ReadOperation(context.Context, string) (domain.Operation, error)
	Verify(context.Context, string) (domain.Operation, error)
}
type CommandFactory func(*http.Request) (CommandService, error)

func NewCommandHandler(service *domain.Service, commands CommandFactory) *Handler {
	return &Handler{service: service, commands: commands}
}

func ResolveMutationTarget(r *http.Request) (string, error) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery || len(r.Header.Values("X-Requested-Organization-ID")) != 1 {
		return "", domain.ErrInvalidRequest
	}
	org := r.Header.Get("X-Requested-Organization-ID")
	if !authidentity.IsBoundedIdentifier(org) {
		return "", domain.ErrInvalidRequest
	}
	return org, nil
}
func validateMutationScope(c *gin.Context) error {
	org, err := ResolveMutationTarget(c.Request)
	if err != nil {
		return err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return domain.ErrAuthentication
	}
	if identity.EffectiveOrganizationID != org {
		return domain.ErrPermission
	}
	return nil
}

func (h *Handler) ChangeRole(c *gin.Context) { h.execute(c, domain.CommandRole) }
func (h *Handler) Remove(c *gin.Context)     { h.execute(c, domain.CommandRemove) }
func (h *Handler) Invite(c *gin.Context)     { h.execute(c, domain.CommandInvite) }

func (h *Handler) execute(c *gin.Context, kind domain.CommandKind) {
	if err := validateMutationScope(c); err != nil {
		writeError(c, err)
		return
	}
	if len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		writeError(c, domain.ErrInvalidRequest)
		return
	}
	key := c.GetHeader("Idempotency-Key")
	id, err := uuid.Parse(key)
	if err != nil || id == uuid.Nil || id.String() != key {
		writeError(c, domain.ErrInvalidRequest)
		return
	}
	input := domain.CommandInput{Kind: kind, AuthorizationID: c.Param("member_id")}
	switch kind {
	case domain.CommandInvite:
		var payload struct {
			Email     string `json:"email"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
			Role      string `json:"role"`
		}
		err = decodeCommand(c.Request, &payload)
		input.Role = payload.Role
		input.Invitation = &domain.Invitation{Email: payload.Email, FirstName: payload.FirstName, LastName: payload.LastName}
	case domain.CommandRole:
		var payload struct {
			Role            string `json:"role"`
			ExpectedVersion string `json:"expectedVersion"`
		}
		err = decodeCommand(c.Request, &payload)
		input.Role = payload.Role
		input.ExpectedVersion = payload.ExpectedVersion
	case domain.CommandRemove:
		var payload struct {
			ExpectedVersion string `json:"expectedVersion"`
		}
		err = decodeCommand(c.Request, &payload)
		input.ExpectedVersion = payload.ExpectedVersion
	}
	if err != nil {
		writeError(c, domain.ErrInvalidRequest)
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
	op, err := commands.Execute(c.Request.Context(), key, input)
	h.respondOperation(c, op, err)
}

func decodeCommand(r *http.Request, value any) error {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" || r.Body == nil {
		return domain.ErrInvalidRequest
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 16385))
	if err != nil || len(body) > 16384 {
		return domain.ErrInvalidRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return domain.ErrInvalidRequest
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return domain.ErrInvalidRequest
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return domain.ErrInvalidRequest
		}
		seen[name] = true
		var raw json.RawMessage
		if decoder.Decode(&raw) != nil {
			return domain.ErrInvalidRequest
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return domain.ErrInvalidRequest
	}
	if _, err = decoder.Token(); err != io.EOF {
		return domain.ErrInvalidRequest
	}
	decoder = json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		return domain.ErrInvalidRequest
	}
	return nil
}

func (h *Handler) GetOperation(c *gin.Context)    { h.operation(c, false) }
func (h *Handler) VerifyOperation(c *gin.Context) { h.operation(c, true) }
func (h *Handler) operation(c *gin.Context, verify bool) {
	if err := validateMutationScope(c); err != nil {
		writeError(c, err)
		return
	}
	if c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) != 0 {
		writeError(c, domain.ErrInvalidRequest)
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
	var op domain.Operation
	if verify {
		op, err = commands.Verify(c.Request.Context(), c.Param("operation_id"))
	} else {
		op, err = commands.ReadOperation(c.Request.Context(), c.Param("operation_id"))
	}
	h.respondOperation(c, op, err)
}

func (h *Handler) respondOperation(c *gin.Context, op domain.Operation, err error) {
	if err != nil {
		writeError(c, err)
		return
	}
	if c.Request.Context().Err() != nil {
		writeError(c, domain.ErrUnavailable)
		return
	}
	status := "pending"
	switch op.Phase {
	case domain.PhaseDispatched:
		status = "unknown"
	case domain.PhaseCompleted:
		status = "acknowledged"
	case domain.PhaseRejected:
		status = "rejected"
	}
	var observed *domain.Member
	observation := "unavailable"
	memberID := op.AuthorizationID
	if op.Kind == domain.CommandInvite && op.Phase == domain.PhaseCompleted && op.Acknowledgment != nil {
		memberID = op.Acknowledgment.ID
	}
	if h.service != nil && memberID != "" {
		result, readErr := h.service.Read(c.Request.Context(), memberID)
		if readErr == nil && len(result.Items) == 1 {
			observed = &result.Items[0]
			observation = "observed"
		} else if readErr == domain.ErrNotFound {
			observation = "not_visible"
		}
	}
	if c.Request.Context().Err() != nil {
		writeError(c, domain.ErrUnavailable)
		return
	}
	responseHeaders(c)
	c.JSON(http.StatusOK, gin.H{"schemaVersion": "membership-operation-v1", "userId": op.Scope.ActorID, "organizationId": op.Scope.OrganizationID, "id": op.Key, "kind": op.Kind, "step": op.Step, "status": status, "targetUserId": op.TargetUserID, "authorizationId": memberID, "userEvidence": op.UserEvidence, "userAcknowledgment": op.UserAcknowledgment, "acknowledgment": op.Acknowledgment, "observation": observation, "observed": observed})
}
