package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"task-processor/internal/storecenter"
)

func (h *Handler) ReadOfficialConnection(c *gin.Context) {
	identity, id, ok := h.itemIdentity(c)
	if !ok {
		return
	}
	view, err := h.officialConnections.Read(c.Request.Context(), identity.EffectiveOrganizationID, id)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, view)
}
func (h *Handler) BeginOfficialConnection(c *gin.Context) {
	command, ok := h.connectionCommand(c)
	if !ok {
		return
	}
	result, err := h.officialConnections.Begin(c.Request.Context(), command)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(http.StatusOK, result)
}
func (h *Handler) DisconnectOfficialConnection(c *gin.Context) {
	command, ok := h.connectionCommand(c)
	if !ok {
		return
	}
	result, err := h.officialConnections.Disconnect(c.Request.Context(), command)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}
func (h *Handler) connectionCommand(c *gin.Context) (storecenter.OfficialConnectionCommand, bool) {
	identity, id, ok := h.itemIdentity(c)
	if !ok {
		return storecenter.OfficialConnectionCommand{}, false
	}
	version, err := requiredIfMatch(c.Request)
	if err != nil {
		writeInvalid(c, "If-Match", "invalid")
		return storecenter.OfficialConnectionCommand{}, false
	}
	key, err := requiredCanonicalUUIDHeader(c.Request, "Idempotency-Key")
	if err != nil {
		writeInvalid(c, "Idempotency-Key", "invalid")
		return storecenter.OfficialConnectionCommand{}, false
	}
	if err := requireNoBody(c.Request.Body); err != nil {
		writeInvalid(c, "body", "not_allowed")
		return storecenter.OfficialConnectionCommand{}, false
	}
	return storecenter.OfficialConnectionCommand{OrganizationID: identity.EffectiveOrganizationID, StoreID: id, AttemptID: key, ExpectedStoreVersion: version}, true
}
func (h *Handler) CompleteOfficialConnection(c *gin.Context) {
	identity, id, ok := h.itemIdentity(c)
	if !ok {
		return
	}
	if len(c.Request.Header.Values("If-Match")) != 0 || len(c.Request.Header.Values("Idempotency-Key")) != 0 {
		writeInvalid(c, "headers", "not_allowed")
		return
	}
	values, field, err := parseStringObject(c.Request.Body, map[string]bool{"attemptId": true, "appId": true, "state": true, "tempToken": true})
	if err != nil {
		writeInvalid(c, field, "invalid")
		return
	}
	attempt, err := canonicalUUID(values["attemptId"])
	if err != nil {
		writeInvalid(c, "attemptId", "invalid")
		return
	}
	if len(values["appId"]) > 200 || len(values["state"]) > 128 || len(values["tempToken"]) > 4096 {
		writeInvalid(c, "body", "invalid")
		return
	}
	result, err := h.officialConnections.Complete(c.Request.Context(), storecenter.CompleteOfficialConnection{OrganizationID: identity.EffectiveOrganizationID, StoreID: id, AttemptID: attempt, AppID: values["appId"], State: values["state"], TempToken: values["tempToken"]})
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}
