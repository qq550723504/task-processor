package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

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
