package httpapi

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/listing/record"

	"github.com/gin-gonic/gin"
	sigjson "sigs.k8s.io/json"
)

const sheinRecordPath = "/api/listing/shein-records"

func sheinRecordRoutes(service *record.Service) []httproute.Descriptor {
	if service == nil {
		return nil
	}
	return []httproute.Descriptor{{Method: http.MethodPost, Path: sheinRecordPath, Module: "listing-record", Permission: authz.PermissionListingKitAdminWrite, AuthPolicy: httproute.AuthPolicyVerifiedIdentity, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyLiveWrite, Handler: func(c *gin.Context) { createSheinRecord(c, service) }}}
}
func createSheinRecord(c *gin.Context, service *record.Service) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), record.Timeout)
	defer cancel()
	if c.Request.URL.RawQuery != "" {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	defer c.Request.Body.Close()
	controller := http.NewResponseController(c.Writer)
	deadline, _ := ctx.Deadline()
	// gin's writer unwrap is not supported on every transport; the actual
	// application server also has a finite ReadTimeout for slow bodies.
	_ = controller.SetReadDeadline(deadline)
	var input record.Input
	raw, err := io.ReadAll(c.Request.Body)
	var transportError net.Error
	// The socket read deadline can fire before the context timer is scheduled.
	// Preserve both cancellation and transport timeouts before JSON validation.
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &transportError) && transportError.Timeout()) {
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "deadline_exceeded"})
		return
	}
	if err == nil {
		var violations []error
		violations, err = sigjson.UnmarshalStrict(raw, &input)
		if len(violations) > 0 {
			err = record.ErrInvalid
		}
	}
	keys := c.Request.Header.Values("Idempotency-Key")
	if err != nil || len(keys) != 1 {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	receipt, err := service.Create(ctx, keys[0], input)
	if err != nil {
		status := http.StatusServiceUnavailable
		code := "unavailable"
		switch {
		case errors.Is(err, record.ErrInvalid):
			status = 400
			code = "invalid_request"
		case errors.Is(err, record.ErrForbidden):
			status = 403
			code = "permission_denied"
		case errors.Is(err, record.ErrNotFound):
			status = 404
			code = "not_found"
		case errors.Is(err, record.ErrConflict):
			status = 409
			code = "operation_conflict"
		case errors.Is(err, record.ErrTooLarge):
			status = 413
			code = "input_too_large"
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			status = 504
			code = "deadline_exceeded"
		}
		c.JSON(status, gin.H{"error": code})
		return
	}
	c.JSON(http.StatusCreated, receipt)
}
