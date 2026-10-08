package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	domain "task-processor/internal/organization/membership"
)

type roleCommandService interface {
	MutateRole(context.Context, string, domain.RoleMutation) (domain.RoleMutationResult, error)
}

func (h *Handler) ListRoles(c *gin.Context) {
	if err := validateMutationScope(c); err != nil {
		writeError(c, err)
		return
	}
	result, err := h.service.Roles(c.Request.Context())
	respond(c, result, err)
}

func (h *Handler) MutateRole(c *gin.Context) {
	if err := validateMutationScope(c); err != nil {
		writeError(c, err)
		return
	}
	if h.commands == nil || len(c.Request.Header.Values("Idempotency-Key")) != 1 {
		writeError(c, domain.ErrInvalidRequest)
		return
	}
	var input struct {
		Name            string    `json:"name,omitempty"`
		Modules         *[]string `json:"modules"`
		ExpectedVersion int64     `json:"expectedVersion,omitempty"`
	}
	if decodeCommand(c.Request, &input) != nil || input.Modules == nil {
		writeError(c, domain.ErrInvalidRequest)
		return
	}
	commands, err := h.commands(c.Request)
	if err != nil {
		writeError(c, err)
		return
	}
	roles, ok := commands.(roleCommandService)
	if !ok {
		writeError(c, domain.ErrUnavailable)
		return
	}
	result, err := roles.MutateRole(c.Request.Context(), c.GetHeader("Idempotency-Key"), domain.RoleMutation{RoleID: c.Param("role_id"), Name: input.Name, Modules: *input.Modules, ExpectedVersion: input.ExpectedVersion})
	respond(c, result, err)
}
