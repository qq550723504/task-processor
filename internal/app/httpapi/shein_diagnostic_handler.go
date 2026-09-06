package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"

	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/listing/record"
	contract "task-processor/internal/marketplace/validator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const sheinDiagnosticPath = sheinRecordPath + "/:record_id/offline-diagnostic"
const maxDiagnosticQueryBytes = 1024
const maxDiagnosticResponseBytes = 2 << 20

func sheinDiagnosticRoutes(service *record.DiagnosticService) []httproute.Descriptor {
	if service == nil {
		return nil
	}
	return []httproute.Descriptor{{Method: http.MethodGet, Path: sheinDiagnosticPath, Module: "listing-record", Permission: authz.PermissionListingKitAdminRead, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead, Handler: func(c *gin.Context) { readSheinDiagnostic(c, service) }}}
}

func readSheinDiagnostic(c *gin.Context, service *record.DiagnosticService) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), record.Timeout)
	defer cancel()
	// Read at most one byte even for chunked/unknown-length bodies. The actual
	// application ReadTimeout bounds a peer that never completes its body.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	var transport net.Error
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || (errors.As(err, &transport) && transport.Timeout()) {
		writeSheinDiagnosticError(c, context.DeadlineExceeded)
		return
	}
	id := c.Param("record_id")
	_, idErr := uuid.Parse(id)
	if len(c.Request.URL.RawQuery) > maxDiagnosticQueryBytes {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	query, queryErr := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil || len(body) != 0 || idErr != nil || len(id) != 36 || len(c.Request.URL.RawQuery) > maxDiagnosticQueryBytes || queryErr != nil {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	for key, values := range query {
		if (key != "action" && key != "expected_digest") || len(values) != 1 {
			c.JSON(400, gin.H{"error": "invalid_request"})
			return
		}
	}
	action := contract.Action(query.Get("action"))
	if action != contract.SaveDraft && action != contract.Publish {
		c.JSON(400, gin.H{"error": "unsupported_action"})
		return
	}
	expected := query.Get("expected_digest")
	if _, present := query["expected_digest"]; present && !contract.ValidContentDigest(expected) {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	result, err := service.Diagnose(ctx, id, action, expected)
	if err != nil {
		writeSheinDiagnosticError(c, err)
		return
	}
	wire, err := json.Marshal(projectSheinDiagnostic(result))
	if ctx.Err() != nil {
		writeSheinDiagnosticError(c, ctx.Err())
		return
	}
	if err != nil || len(wire) > maxDiagnosticResponseBytes {
		c.JSON(500, gin.H{"error": "evaluation_failed"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", wire)
}

func writeSheinDiagnosticError(c *gin.Context, err error) {
	status, code := 503, "unavailable"
	var typed *contract.Error
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code = 504, "deadline_exceeded"
	case errors.Is(err, record.ErrForbidden):
		status, code = 403, "permission_denied"
	case errors.Is(err, record.ErrNotFound):
		status, code = 404, "not_found"
	case errors.Is(err, record.ErrTooLarge):
		status, code = 413, "input_too_large"
	case errors.Is(err, record.ErrInvalid):
		status, code = 400, "invalid_request"
	case errors.As(err, &typed):
		code = string(typed.Code)
		switch typed.Code {
		case contract.StaleInput:
			status = 409
		case contract.InvalidInput:
			status = 422
		case contract.UnsupportedAction, contract.UnsupportedTarget:
			status = 400
		default:
			status = 500
		}
	}
	response := gin.H{"error": code}
	if typed != nil && typed.Code == contract.StaleInput && typed.Freshness != nil {
		response["freshness"] = diagnosticFailureDTO{Status: string(typed.Freshness.Status), Coverage: append([]string{}, typed.Freshness.Coverage...), Causes: append([]string{}, typed.Freshness.Causes...)}
	}
	c.JSON(status, response)
}

type diagnosticFailureDTO struct {
	Status   string   `json:"status"`
	Coverage []string `json:"coverage"`
	Causes   []string `json:"causes"`
}
