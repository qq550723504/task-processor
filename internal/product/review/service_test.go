package review

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTitleBoundary(t *testing.T) {
	for _, value := range []string{"", " title", "title\n", string([]byte{255}), strings.Repeat("x", 4097)} {
		require.Error(t, ValidateTitle(value))
	}
	require.NoError(t, ValidateTitle("人工修改标题"))
}
func TestReviewStateInvalidatesApproval(t *testing.T) {
	r := Record{Revision: 1, State: "pending", Title: "generated"}
	require.NoError(t, r.Decide("reviewer", DecisionInput{Action: "accept", ExpectedRevision: 1}))
	require.Equal(t, "accepted", r.State)
	require.NoError(t, r.Decide("owner", DecisionInput{Action: "edit", ExpectedRevision: 2, Title: "edited"}))
	require.Equal(t, "pending", r.State)
	require.Equal(t, uint64(3), r.Revision)
	require.Equal(t, "generated", r.History[1].Before)
	require.ErrorIs(t, r.Decide("reviewer", DecisionInput{Action: "accept", ExpectedRevision: 2}), ErrConflict)
	require.NoError(t, r.Decide("reviewer", DecisionInput{Action: "reject", ExpectedRevision: 3}))
	require.ErrorIs(t, r.Decide("owner", DecisionInput{Action: "edit", ExpectedRevision: 4, Title: "another"}), ErrConflict)
}
