package browsercollector

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/integration/acquisition/a1688/browser"
)

// A challenge whose automatic attempt exhausts the budget arrives as a joined error
// that matches both the challenge sentinel and the deadline. The page is known to be
// challenged, so it has to be reported as a source failure; reporting the budget
// would tell the caller 1688 is merely slow, and the cooldown depends on the caller
// seeing the source failure.
func TestWriteAcquireErrorPrefersChallengeOverExhaustedBudget(t *testing.T) {
	joined := errors.Join(browser.ErrChallenge, context.DeadlineExceeded)

	rec := httptest.NewRecorder()
	writeAcquireError(rec, joined)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), CodeSourceUnavailable)

	// A budget that expired without any challenge is still a budget failure.
	rec = httptest.NewRecorder()
	writeAcquireError(rec, context.DeadlineExceeded)
	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
	require.Contains(t, rec.Body.String(), CodeBudgetExceeded)

	// A cancellation carrying a challenge is likewise the source, not a timeout.
	rec = httptest.NewRecorder()
	writeAcquireError(rec, errors.Join(browser.ErrChallenge, context.Canceled))
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), CodeSourceUnavailable)
}
