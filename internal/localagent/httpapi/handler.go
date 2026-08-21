package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/localagent"
	"task-processor/internal/product/sourcing"
)

type Handler struct {
	service *localagent.Service
}

func NewHandler(service *localagent.Service) *Handler { return &Handler{service: service} }

type createJobRequest struct {
	URL string `json:"url"`
}

type submitResultRequest struct {
	ExecutionToken  string              `json:"execution_token"`
	ProductSnapshot json.RawMessage     `json:"product_snapshot"`
	Failure         *localagent.Failure `json:"failure"`
}

type jobResponse struct {
	JobID          string                   `json:"job_id"`
	TenantID       string                   `json:"tenant_id"`
	URL            string                   `json:"url"`
	State          localagent.JobState      `json:"state"`
	ExpiresAt      time.Time                `json:"expires_at"`
	LeaseExpiresAt time.Time                `json:"lease_expires_at,omitempty"`
	Envelope       *sourcing.SourceEnvelope `json:"envelope,omitempty"`
	Failure        *localagent.Failure      `json:"failure,omitempty"`
}

type claimResponse struct {
	JobID          string    `json:"job_id"`
	ExecutionToken string    `json:"execution_token"`
	URL            string    `json:"url"`
	ExpiresAt      time.Time `json:"expires_at"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.service == nil {
		writeError(c, http.StatusServiceUnavailable, "service_unavailable", "local-agent service is not configured")
		return
	}
	actor, ok := verifiedActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "identity_required", "verified identity is required")
		return
	}
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	job, err := h.service.Create(actor, req.URL)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, responseFromJob(job))
}

func (h *Handler) Claim(c *gin.Context) {
	if h == nil || h.service == nil {
		writeError(c, http.StatusServiceUnavailable, "service_unavailable", "local-agent service is not configured")
		return
	}
	actor, ok := verifiedActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "identity_required", "verified identity is required")
		return
	}
	claim, err := h.service.Claim(actor)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if claim == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, claimResponse{JobID: claim.Job.ID, ExecutionToken: claim.ExecutionToken, URL: claim.Job.URL, ExpiresAt: claim.Job.ExpiresAt, LeaseExpiresAt: claim.Job.LeaseExpiresAt})
}

func (h *Handler) SubmitResult(c *gin.Context) {
	if h == nil || h.service == nil {
		writeError(c, http.StatusServiceUnavailable, "service_unavailable", "local-agent service is not configured")
		return
	}
	actor, ok := verifiedActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "identity_required", "verified identity is required")
		return
	}
	var req submitResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	jobID := c.Param("job_id")
	var (
		job localagent.Job
		err error
	)
	if len(req.ProductSnapshot) > 0 && string(req.ProductSnapshot) != "null" && req.Failure != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "submit exactly one product_snapshot or failure")
		return
	}
	if len(req.ProductSnapshot) > 0 && string(req.ProductSnapshot) != "null" {
		var snapshot sourcing.Alibaba1688ProductSnapshot
		if err := json.Unmarshal(req.ProductSnapshot, &snapshot); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		job, err = h.service.SubmitSuccess(actor, jobID, req.ExecutionToken, &snapshot)
	} else if req.Failure != nil {
		job, err = h.service.SubmitFailure(actor, jobID, req.ExecutionToken, *req.Failure)
	} else {
		writeError(c, http.StatusBadRequest, "invalid_request", "submit exactly one product_snapshot or failure")
		return
	}
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, responseFromJob(job))
}

func responseFromJob(job localagent.Job) jobResponse {
	return jobResponse{JobID: job.ID, TenantID: job.TenantID, URL: job.URL, State: job.State, ExpiresAt: job.ExpiresAt, LeaseExpiresAt: job.LeaseExpiresAt, Envelope: job.Envelope, Failure: job.Failure}
}

func verifiedActor(c *gin.Context) (localagent.Actor, bool) {
	if c == nil || c.Request == nil {
		return localagent.Actor{}, false
	}
	identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
	if !ok {
		return localagent.Actor{}, false
	}
	return localagent.Actor{TenantID: identity.TenantID, UserID: identity.UserID}, true
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, localagent.ErrIdentityRequired), errors.Is(err, localagent.ErrInvalidURL), errors.Is(err, localagent.ErrFailureInvalid), errors.Is(err, localagent.ErrSnapshotTooLarge):
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, localagent.ErrClaimExpired), errors.Is(err, localagent.ErrTerminalJob):
		writeError(c, http.StatusConflict, "job_not_active", err.Error())
	case errors.Is(err, localagent.ErrInvalidClaim):
		writeError(c, http.StatusForbidden, "claim_denied", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "local_agent_error", err.Error())
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": code, "message": strings.TrimSpace(message)})
}
