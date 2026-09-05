package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/storecenter"
)

type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type protocolError struct {
	Status      int          `json:"-"`
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	RequestID   string       `json:"requestId"`
	FieldErrors []FieldError `json:"fieldErrors"`
}

func mapStoreError(err error) protocolError {
	response := protocolError{Status: http.StatusServiceUnavailable, Code: "DEPENDENCY_UNAVAILABLE", Message: "A required dependency is unavailable", FieldErrors: []FieldError{}}
	switch {
	case errors.Is(err, storecenter.ErrNotFound):
		response.Status, response.Code, response.Message = http.StatusNotFound, "STORE_NOT_FOUND", "Store was not found"
	case errors.Is(err, storecenter.ErrAlreadyExists):
		response.Status, response.Code, response.Message = http.StatusConflict, "STORE_ALREADY_EXISTS", "Store already exists"
	case errors.Is(err, storecenter.ErrVersionConflict):
		response.Status, response.Code, response.Message = http.StatusConflict, "STORE_VERSION_CONFLICT", "Store has changed"
	case errors.Is(err, storecenter.ErrServiceResumeRequired):
		response.Status, response.Code, response.Message = http.StatusConflict, "STORE_SERVICE_RESUME_REQUIRED", "Store service must be resumed"
	case errors.Is(err, storecenter.ErrInvalidServiceState):
		response.Status, response.Code, response.Message = http.StatusConflict, "STORE_SERVICE_STATE_CORRUPT", "Store service state is invalid"
	case errors.Is(err, storecenter.ErrConnectionUnavailable), errors.Is(err, storecenter.ErrConnectionSnapshotChanged):
		response.Status, response.Code, response.Message = http.StatusServiceUnavailable, "STORE_CONNECTION_UNAVAILABLE", "Store connection status is unavailable"
	case errors.Is(err, storecenter.ErrConnectionNotFresh):
		response.Status, response.Code, response.Message = http.StatusUnprocessableEntity, "STORE_CONNECTION_NOT_CONNECTED", "Store connection is not connected"
	case errors.Is(err, storecenter.ErrServiceQuantityInvalid), errors.Is(err, storecenter.ErrServiceQuantityExceeded):
		response.Status, response.Code, response.Message = http.StatusUnprocessableEntity, "RESOURCE_QUANTITY_INVALID", "Resource quantity is invalid"
	case errors.Is(err, orgresource.ErrInsufficientBalance):
		response.Status, response.Code, response.Message = http.StatusConflict, "RESOURCE_INSUFFICIENT_BALANCE", "Resource balance is insufficient"
	case errors.Is(err, orgresource.ErrIdempotencyKeyConflict):
		response.Status, response.Code, response.Message = http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "Idempotency key conflicts with an earlier request"
	case errors.Is(err, orgresource.ErrConcurrencyRetry):
		response.Status, response.Code, response.Message = http.StatusServiceUnavailable, "RESOURCE_CONCURRENCY_RETRY", "Resource concurrency retry is required"
	case errors.Is(err, storecenter.ErrInvalidServiceTransition), errors.Is(err, storecenter.ErrServiceAlreadyActive), errors.Is(err, storecenter.ErrServiceExpired), errors.Is(err, storecenter.ErrServiceNotExpired), errors.Is(err, storecenter.ErrServiceSuspended):
		response.Status, response.Code, response.Message = http.StatusUnprocessableEntity, "STORE_INVALID_STATE", "Store state does not allow this operation"
	case errors.Is(err, storecenter.ErrInvalidTransition):
		response.Status, response.Code, response.Message = http.StatusUnprocessableEntity, "STORE_INVALID_STATE", "Store state does not allow this operation"
	case errors.Is(err, listingsubscription.ErrSubscriptionRequired):
		response.Status, response.Code, response.Message = http.StatusConflict, "SUBSCRIPTION_REQUIRED", "A subscription is required"
	case errors.Is(err, storecenter.ErrLimitReached):
		response.Status, response.Code, response.Message = http.StatusConflict, "STORE_LIMIT_REACHED", "Store limit has been reached"
	}
	return response
}

func writeStoreError(c *gin.Context, err error) {
	response := mapStoreError(err)
	writeError(c, response.Status, response.Code, response.Message, response.FieldErrors)
}

func writeInvalid(c *gin.Context, field, code string) {
	writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "Request is invalid", []FieldError{{Field: field, Code: code}})
}

func writeError(c *gin.Context, status int, code, message string, fields []FieldError) {
	if fields == nil {
		fields = []FieldError{}
	}
	c.AbortWithStatusJSON(status, protocolError{
		Code: code, Message: message, RequestID: strings.TrimSpace(c.GetHeader("X-Request-ID")), FieldErrors: fields,
	})
}
