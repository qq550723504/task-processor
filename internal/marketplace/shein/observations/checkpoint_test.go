package observations

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOrderWindowsCoverBoundarySecondsAndRejectUnboundedRange(t *testing.T) {
	end := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	start := end.Add(-30 * 24 * time.Hour)
	w, e := OrderWindows(start, end)
	require.NoError(t, e)
	require.NotEmpty(t, w)
	require.Equal(t, start, w[0].Start)
	require.Equal(t, end, w[len(w)-1].End)
	for i, x := range w {
		require.LessOrEqual(t, x.End.Sub(x.Start), 48*time.Hour)
		if i > 0 {
			require.Equal(t, w[i-1].End, x.Start)
		}
	}
	_, e = OrderWindows(end, start)
	require.ErrorIs(t, e, ErrInvalid)
	_, e = OrderWindows(end.Add(-32*24*time.Hour), end)
	require.ErrorIs(t, e, ErrInvalid)
}
func TestProductsDoNotCompleteOnDriftingCountOrMissingRows(t *testing.T) {
	n := 11
	s := Sync{Kind: Products, Progress: Checkpoint{Page: 1}}
	c, status, e := Advance(s, 10, &n)
	require.NoError(t, e)
	require.Equal(t, "running", status)
	require.Equal(t, 2, c.Page)
	s.Progress = c
	n = 12
	c, status, e = Advance(s, 1, &n)
	require.NoError(t, e)
	require.Equal(t, "partial", status)
	require.True(t, c.Incomplete)
}
func TestOrderCountIsNeverMistakenForWindowTotalAndIncompleteIsSticky(t *testing.T) {
	end := time.Now().Truncate(time.Second)
	n := 30
	s := Sync{Kind: Orders, Progress: Checkpoint{Page: 1, Windows: []Window{{end.Add(-time.Hour), end}}}}
	c, status, e := Advance(s, 30, &n)
	require.NoError(t, e)
	require.Equal(t, "running", status)
	require.Equal(t, 2, c.Page)
	c.Note("site_missing")
	s.Progress = c
	c, status, e = Advance(s, 0, nil)
	require.NoError(t, e)
	require.Equal(t, "partial", status)
	require.True(t, c.Incomplete)
}
func TestSaturatedOrderWindowSplitsWithoutSkippingEqualSecondOrders(t *testing.T) {
	start := time.Now().Truncate(time.Second)
	c := Checkpoint{Page: 333, Windows: []Window{{start, start.Add(time.Hour)}}}
	c, status := SplitWindow(c)
	require.Equal(t, "running", status)
	require.Len(t, c.Windows, 2)
	require.Equal(t, c.Windows[0].End, c.Windows[1].Start)
	require.Equal(t, 1, c.Page)
	c = Checkpoint{Page: 333, Windows: []Window{{start, start}}}
	c, status = SplitWindow(c)
	require.Equal(t, "partial", status)
	require.True(t, c.Incomplete)
}
