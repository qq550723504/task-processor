package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/aiworkbench"
)

func TestWorkbenchRevisionConflictDiffersFromStaleProposal(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		err        error
	}{
		{"metadata or task CAS", "REVISION_MISMATCH", aiworkbench.ErrRevisionMismatch},
		{"proposal confirmation", "PROPOSAL_STALE", aiworkbench.ErrProposalStale},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			writeAIWorkbenchError(ctx, tc.err)
			require.Equal(t, http.StatusConflict, w.Code)
			require.JSONEq(t, `{"code":"`+tc.want+`"}`, w.Body.String())
		})
	}
}
