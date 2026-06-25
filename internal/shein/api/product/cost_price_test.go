package product

import (
	"fmt"
	"net/http"
	"testing"

	sheinapi "task-processor/internal/shein/api"

	"github.com/stretchr/testify/require"
)

func TestIsCostPriceUnavailableRecognizesForbiddenAPIError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("query cost price: %w", &sheinapi.APIError{
		StatusCode: http.StatusForbidden,
		Message:    "Forbidden",
	})

	require.True(t, IsCostPriceUnavailable(err))
	require.False(t, IsCostPriceUnavailable(&sheinapi.APIError{
		StatusCode: http.StatusUnauthorized,
		Message:    "Unauthorized",
	}))
}
