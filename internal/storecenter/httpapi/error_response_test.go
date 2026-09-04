package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingsubscription"
	"task-processor/internal/storecenter"
)

func TestMapStoreErrorUsesStableRedactedProtocolContract(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "not found", err: fmt.Errorf("sql secret: %w", storecenter.ErrNotFound), status: http.StatusNotFound, code: "STORE_NOT_FOUND"},
		{name: "already exists", err: fmt.Errorf("provider secret: %w", storecenter.ErrAlreadyExists), status: http.StatusConflict, code: "STORE_ALREADY_EXISTS"},
		{name: "version", err: fmt.Errorf("row secret: %w", storecenter.ErrVersionConflict), status: http.StatusConflict, code: "STORE_VERSION_CONFLICT"},
		{name: "service resume", err: fmt.Errorf("state secret: %w", storecenter.ErrServiceResumeRequired), status: http.StatusConflict, code: "STORE_SERVICE_RESUME_REQUIRED"},
		{name: "lifecycle", err: fmt.Errorf("state secret: %w", storecenter.ErrInvalidTransition), status: http.StatusUnprocessableEntity, code: "STORE_INVALID_STATE"},
		{name: "subscription", err: fmt.Errorf("entitlement secret: %w", listingsubscription.ErrSubscriptionRequired), status: http.StatusConflict, code: "SUBSCRIPTION_REQUIRED"},
		{name: "limit", err: &storecenter.StoreLimitReachedError{Used: 3, Committed: 3, Limit: 3}, status: http.StatusConflict, code: "STORE_LIMIT_REACHED"},
		{name: "dependency", err: fmt.Errorf("password=secret: %w", storecenter.ErrDependencyUnavailable), status: http.StatusServiceUnavailable, code: "DEPENDENCY_UNAVAILABLE"},
		{name: "unknown", err: errors.New("token=secret"), status: http.StatusServiceUnavailable, code: "DEPENDENCY_UNAVAILABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStoreError(tt.err)
			require.Equal(t, tt.status, got.Status)
			require.Equal(t, tt.code, got.Code)
			require.NotContains(t, got.Message, "secret")
			require.NotContains(t, got.Message, "password")
			require.Empty(t, got.FieldErrors)
			require.NotNil(t, got.FieldErrors)
		})
	}
}
