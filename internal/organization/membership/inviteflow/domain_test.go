package inviteflow

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConsentTransitionsAndUnknownReservation(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	inv := Invitation{State: Pending, Revision: 1, ExpiresAt: now.Add(time.Hour)}
	claimed, err := Transition(inv, Change{State: Accepting, RecipientID: "recipient", DispatchID: "dispatch", Now: now})
	require.NoError(t, err)
	require.Equal(t, Accepting, claimed.State)
	require.EqualValues(t, 2, claimed.Revision)
	_, err = Transition(claimed, Change{State: Cancelled, Now: now})
	require.ErrorIs(t, err, ErrConflict)
	_, err = Transition(claimed, Change{State: Expired, Now: now.Add(2 * time.Hour)})
	require.ErrorIs(t, err, ErrConflict)
	accepted, err := Transition(claimed, Change{State: Accepted, RecipientID: "recipient", AuthorizationID: "grant", Now: now})
	require.NoError(t, err)
	require.Equal(t, "grant", accepted.AuthorizationID)
	_, err = Transition(accepted, Change{State: Accepting, RecipientID: "recipient", DispatchID: "again", Now: now})
	require.ErrorIs(t, err, ErrConflict)
	_, err = Transition(inv, Change{State: Accepting, RecipientID: "recipient", DispatchID: "dispatch", Now: now.Add(2 * time.Hour)})
	require.ErrorIs(t, err, ErrConflict)
	for _, terminal := range []string{Declined, Cancelled} {
		changed, err := Transition(inv, Change{State: terminal, RecipientID: "recipient", Now: now})
		require.NoError(t, err)
		require.Equal(t, terminal, changed.State)
	}
}
