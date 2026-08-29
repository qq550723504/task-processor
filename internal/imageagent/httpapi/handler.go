package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
)

const projectionEventSchemaVersion = "image-agent.projection.v1"

const (
	defaultSSEPollInterval      = 500 * time.Millisecond
	defaultSSEHeartbeatInterval = 15 * time.Second
	eventPageSize               = 100
	maxRequestBodyBytes         = 1 << 20
)

type Application interface {
	Start(context.Context, imageagent.StartRunInput) error
	RestartFailed(context.Context, string) error
	Get(context.Context, string) (imageagent.RunProjection, error)
	ReplacePlan(context.Context, string, int64, imageagent.Plan, string) error
	RecoverEffect(context.Context, string, string, int, int64, string) error
	RetrySlot(context.Context, string, string, int64, string) error
	ApproveResults(context.Context, string, int64, string, string) error
	Cancel(context.Context, string, int64, string) error
	Resume(context.Context, string, string) (imageagent.CommandAcknowledgement, error)
	ListEvents(context.Context, string, int64, int) ([]imageagent.RunEvent, error)
}

type Handler struct {
	application       Application
	publicURLs        imageagent.DurableAssetPublicURLResolver
	pollInterval      time.Duration
	heartbeatInterval time.Duration
}

type HandlerOption func(*Handler) error

func WithDurableAssetPublicURLResolver(resolver imageagent.DurableAssetPublicURLResolver) HandlerOption {
	return func(handler *Handler) error {
		if resolver == nil {
			return fmt.Errorf("image agent durable asset public URL resolver is required")
		}
		handler.publicURLs = resolver
		return nil
	}
}

func NewHandler(application Application, options ...HandlerOption) (*Handler, error) {
	if application == nil {
		return nil, fmt.Errorf("image agent application is required")
	}
	handler := &Handler{application: application, pollInterval: defaultSSEPollInterval, heartbeatInterval: defaultSSEHeartbeatInterval}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("image agent handler option is required")
		}
		if err := option(handler); err != nil {
			return nil, err
		}
	}
	return handler, nil
}

type createRunRequest struct {
	RunID              string             `json:"run_id"`
	BusinessTaskID     string             `json:"business_task_id"`
	Mode               imageagent.RunMode `json:"mode"`
	IdempotencyKey     string             `json:"idempotency_key"`
	Plan               planDTO            `json:"plan"`
	Budget             budgetInputDTO     `json:"budget"`
	MaxConcurrentSlots int                `json:"max_concurrent_slots"`
}

func (h *Handler) Create(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	var request createRunRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeInvalidJSON(c, err)
		return
	}
	budget, err := request.Budget.domain()
	if err != nil {
		writeInvalidJSON(c, err)
		return
	}
	if err := h.application.Start(c.Request.Context(), imageagent.StartRunInput{
		RunID: request.RunID, BusinessTaskID: request.BusinessTaskID, Mode: request.Mode,
		IdempotencyKey: request.IdempotencyKey, Plan: request.Plan.domain(), Budget: budget,
		MaxConcurrentSlots: request.MaxConcurrentSlots,
	}); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"run_id": request.RunID, "status": "accepted"})
}

func (h *Handler) Get(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	projection, err := h.application.Get(c.Request.Context(), c.Param("run_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, newRunProjectionResponse(projection, h.publicURLs))
}

func (h *Handler) RestartFailed(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	if err := h.application.RestartFailed(c.Request.Context(), c.Param("run_id")); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"run_id": c.Param("run_id"), "status": "accepted"})
}

type replacePlanRequest struct {
	ExpectedRevision int64   `json:"expected_revision"`
	ActionID         string  `json:"action_id"`
	Plan             planDTO `json:"plan"`
}

func (h *Handler) ReplacePlan(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	var request replacePlanRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeInvalidJSON(c, err)
		return
	}
	if err := h.application.ReplacePlan(c.Request.Context(), c.Param("run_id"), request.ExpectedRevision, request.Plan.domain(), request.ActionID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

type revisionCommandRequest struct {
	PlanRevision int64  `json:"plan_revision"`
	ActionID     string `json:"action_id"`
}

type approvalRequest struct {
	PlanRevision int64  `json:"plan_revision"`
	ResultDigest string `json:"result_digest"`
	ActionID     string `json:"action_id"`
}

func (h *Handler) RetrySlot(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	var request revisionCommandRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeInvalidJSON(c, err)
		return
	}
	if err := h.application.RetrySlot(c.Request.Context(), c.Param("run_id"), c.Param("slot_id"), request.PlanRevision, request.ActionID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *Handler) RecoverEffect(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	var request revisionCommandRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeInvalidJSON(c, err)
		return
	}
	attempt, err := strconv.Atoi(strings.TrimSpace(c.Param("attempt")))
	if err != nil || attempt <= 0 {
		writeInvalidJSON(c, fmt.Errorf("attempt must be a positive integer"))
		return
	}
	if err := h.application.RecoverEffect(c.Request.Context(), c.Param("run_id"), c.Param("slot_id"), attempt, request.PlanRevision, request.ActionID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *Handler) ApproveResults(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	var request approvalRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeInvalidJSON(c, err)
		return
	}
	if err := h.application.ApproveResults(c.Request.Context(), c.Param("run_id"), request.PlanRevision, request.ResultDigest, request.ActionID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *Handler) Cancel(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	var request revisionCommandRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		writeInvalidJSON(c, err)
		return
	}
	if err := h.application.Cancel(c.Request.Context(), c.Param("run_id"), request.PlanRevision, request.ActionID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *Handler) Resume(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	acknowledgement, err := h.application.Resume(c.Request.Context(), c.Param("run_id"), c.Param("action_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"run_id": acknowledgement.RunID, "plan_revision": acknowledgement.PlanRevision,
		"action_id": acknowledgement.ActionID, "status": acknowledgement.Status,
	})
}

type projectionEventEnvelope struct {
	SchemaVersion     string `json:"schema_version"`
	Type              string `json:"type"`
	ProjectionVersion int64  `json:"projection_version"`
}

func (h *Handler) Events(c *gin.Context) {
	if !requireVerifiedIdentity(c) {
		return
	}
	after, err := parseEventCursor(c.Request)
	if err != nil {
		writeInvalidJSON(c, err)
		return
	}
	projection, err := h.application.Get(c.Request.Context(), c.Param("run_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	if after > projection.LastEventID {
		writeInvalidJSON(c, fmt.Errorf("Last-Event-ID is ahead of the current snapshot"))
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sse_unavailable", "message": "streaming is not supported"})
		return
	}
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	flusher.Flush()

	cursor := after
	emitAvailable := func() bool {
		for {
			events, listErr := h.application.ListEvents(c.Request.Context(), c.Param("run_id"), cursor, eventPageSize)
			if listErr != nil {
				return false
			}
			for _, event := range events {
				if event.Cursor <= cursor || event.ProjectionVersion != event.Cursor {
					return false
				}
				encoded, encodeErr := json.Marshal(projectionEventEnvelope{
					SchemaVersion: projectionEventSchemaVersion, Type: event.Type,
					ProjectionVersion: event.ProjectionVersion,
				})
				if encodeErr != nil {
					return false
				}
				if _, writeErr := fmt.Fprintf(c.Writer, "id: %d\nevent: projection\ndata: %s\n\n", event.Cursor, encoded); writeErr != nil {
					return false
				}
				cursor = event.Cursor
				flusher.Flush()
			}
			if len(events) < eventPageSize {
				return true
			}
		}
	}
	if !emitAvailable() {
		return
	}
	poll := time.NewTicker(h.pollInterval)
	heartbeat := time.NewTicker(h.heartbeatInterval)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-poll.C:
			if !emitAvailable() {
				return
			}
		case <-heartbeat.C:
			if _, err := io.WriteString(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, imageagent.ErrIdentityRequired):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identity_required", "message": "verified identity is required"})
	case errors.Is(err, imageagent.ErrValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
	case errors.Is(err, imageagent.ErrCommandBlocked):
		c.JSON(http.StatusConflict, gin.H{"error": "command_blocked", "message": "image agent command is not valid in the current state"})
	case errors.Is(err, imageagent.ErrRevisionConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "revision_conflict", "message": "image agent command revision is stale"})
	case errors.Is(err, imageagent.ErrRunNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "run_not_found", "message": "image agent run was not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "image_agent_failed", "message": "image agent request failed"})
	}
}

func requireVerifiedIdentity(c *gin.Context) bool {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identity_required", "message": "verified identity is required"})
		return false
	}
	return true
}

func decodeStrictJSON(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

func writeInvalidJSON(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
}

func parseEventCursor(request *http.Request) (int64, error) {
	headerValues := request.Header.Values("Last-Event-ID")
	if len(headerValues) > 1 {
		return 0, fmt.Errorf("multiple Last-Event-ID values are not allowed")
	}
	queryValues, queryPresent := request.URL.Query()["after_cursor"]
	if queryPresent && len(queryValues) != 1 {
		return 0, fmt.Errorf("multiple after_cursor values are not allowed")
	}
	headerValue := strings.TrimSpace(request.Header.Get("Last-Event-ID"))
	queryValue := ""
	if queryPresent {
		queryValue = strings.TrimSpace(queryValues[0])
		if queryValue == "" {
			return 0, fmt.Errorf("after_cursor must be a non-negative integer")
		}
	}
	if headerValue != "" && queryValue != "" && headerValue != queryValue {
		return 0, fmt.Errorf("Last-Event-ID and after_cursor must match when both are provided")
	}
	value, label := headerValue, "Last-Event-ID"
	if queryValue != "" {
		value, label = queryValue, "after_cursor"
	}
	if value == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cursor < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", label)
	}
	return cursor, nil
}
