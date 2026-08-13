package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	a1688 "task-processor/internal/product/sourcehandoff/a1688"
	"task-processor/internal/product/sourcing"
)

type TaskCommandService interface {
	CreateTask(context.Context, a1688.CreateTaskCommand) (*a1688.CreateTaskResult, error)
}

type Handler struct {
	service TaskCommandService
}

func NewHandler(service TaskCommandService) *Handler {
	return &Handler{service: service}
}

type CreateListingKitTaskRequest struct {
	URL             string                        `json:"url"`
	Product         *alibaba1688model.Product1688 `json:"product"`
	RawSnapshot     string                        `json:"raw_snapshot"`
	SourceRunID     string                        `json:"source_run_id"`
	RequestID       string                        `json:"request_id"`
	SourceError     string                        `json:"source_error"`
	SourceAccountID int64                         `json:"source_account_id"`

	Platforms          []string                    `json:"platforms"`
	Country            string                      `json:"country"`
	Language           string                      `json:"language"`
	SheinStoreID       int64                       `json:"shein_store_id"`
	TargetCategoryHint string                      `json:"target_category_hint"`
	Options            *listingkit.GenerateOptions `json:"options"`
}

type CreateListingKitTaskResponse struct {
	TaskID         string                   `json:"task_id,omitempty"`
	TenantID       string                   `json:"tenant_id,omitempty"`
	Status         core.TaskStatus          `json:"status,omitempty"`
	SourceIdentity sourcing.SourceIdentity  `json:"source_identity"`
	SourceWarnings []sourcing.SourceWarning `json:"source_warnings,omitempty"`
	ProductURL     string                   `json:"product_url,omitempty"`
}

func (h *Handler) CreateListingKitTask(c *gin.Context) {
	if h == nil || h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable", "message": "1688 listingkit task service is not configured"})
		return
	}
	var req CreateListingKitTaskRequest
	if err := rejectLegacySourceStoreID(c); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	ctx, identity, err := verifiedRequestContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.service.CreateTask(ctx, req.toCommand(identity))
	if err != nil {
		if writeStoreAccessError(c, err) {
			return
		}
		status := http.StatusInternalServerError
		if isBadRequestError(err) {
			status = http.StatusBadRequest
		}
		body := gin.H{"error": "task_creation_failed", "message": err.Error()}
		if result != nil && result.Handoff != nil {
			body["source_identity"] = result.Handoff.Envelope.Identity
			body["source_warnings"] = result.Handoff.Envelope.Warnings
		}
		c.JSON(status, body)
		return
	}
	c.JSON(http.StatusOK, responseFromCreateTaskResult(result))
}

func rejectLegacySourceStoreID(c *gin.Context) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return errors.New("request body is required")
	}
	payload, err := c.GetRawData()
	if err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return err
	}
	if _, ok := fields["source_store_id"]; ok {
		return errors.New("source_store_id is not supported; use source_account_id")
	}
	return nil
}

func writeStoreAccessError(c *gin.Context, err error) bool {
	code := listingkit.StoreAccessErrorCode(err)
	if code == "" {
		return false
	}
	message := "Choose an enabled 1688 login account and SHEIN target store available to your tenant and try again."
	if code == listingkit.StoreAccessDisabled {
		message = "Enable the selected 1688 login account or SHEIN target store, or choose another available option and try again."
	}
	c.JSON(http.StatusForbidden, gin.H{"error": code, "message": message})
	return true
}

func verifiedRequestContext(c *gin.Context) (context.Context, listingkit.RequestIdentity, error) {
	if c == nil || c.Request == nil {
		return nil, listingkit.RequestIdentity{}, errors.New("verified request identity is required")
	}
	authenticatedIdentity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok || strings.TrimSpace(authenticatedIdentity.UserID) == "" {
		return nil, listingkit.RequestIdentity{}, errors.New("verified request identity is required")
	}
	identity := listingkit.RequestIdentity{TenantID: authenticatedIdentity.TenantID, UserID: authenticatedIdentity.UserID}
	ctx := listingkit.WithTenantID(c.Request.Context(), identity.TenantID)
	return listingkit.WithRequestIdentity(ctx, identity), identity, nil
}

func (r CreateListingKitTaskRequest) toCommand(identity listingkit.RequestIdentity) a1688.CreateTaskCommand {
	var sourceErr error
	if message := strings.TrimSpace(r.SourceError); message != "" {
		sourceErr = errors.New(message)
	}
	return a1688.CreateTaskCommand{
		URL:                r.URL,
		Product:            r.Product,
		RawSnapshot:        r.RawSnapshot,
		SourceRunID:        r.SourceRunID,
		RequestID:          r.RequestID,
		Error:              sourceErr,
		SourceAccountID:    r.SourceAccountID,
		TenantID:           identity.TenantID,
		UserID:             identity.UserID,
		Platforms:          r.Platforms,
		Country:            r.Country,
		Language:           r.Language,
		SheinStoreID:       r.SheinStoreID,
		TargetCategoryHint: r.TargetCategoryHint,
		Options:            r.Options,
	}
}

func responseFromCreateTaskResult(result *a1688.CreateTaskResult) CreateListingKitTaskResponse {
	var response CreateListingKitTaskResponse
	if result == nil {
		return response
	}
	if result.Task != nil {
		response.TaskID = result.Task.ID
		response.TenantID = result.Task.TenantID
		response.Status = result.Task.Status
	}
	if result.Handoff != nil {
		response.SourceIdentity = result.Handoff.Envelope.Identity
		response.SourceWarnings = result.Handoff.Envelope.Warnings
		response.ProductURL = result.Handoff.Request.ProductURL
	}
	return response
}

func isBadRequestError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "required") ||
		strings.Contains(message, "invalid") ||
		strings.Contains(message, "cannot create listingkit task")
}
