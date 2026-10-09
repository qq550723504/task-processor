package toolmarket

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestHumanProgressDoesNotSkipOrReopen(t *testing.T) {
	require.True(t, Transition("SUBMITTED", "EVALUATING"))
	require.True(t, Transition("EVALUATING", "EVALUATING"))
	require.True(t, Transition("DEVELOPING", "DELIVERED"))
	require.True(t, Transition("SUBMITTED", "CLOSED"))
	require.False(t, Transition("SUBMITTED", "DELIVERED"))
	require.False(t, Transition("DELIVERED", "DEVELOPING"))
	require.False(t, Transition("CLOSED", "CLOSED"))
	require.False(t, Transition("invented", "CLOSED"))
}
func TestDemandIsBoundedData(t *testing.T) {
	require.True(t, (Demand{Kind: "DATA", Title: "采集需求", Description: "保存已授权的商品资料"}).Valid())
	require.False(t, (Demand{Kind: "CODE", Title: "x", Description: "x"}).Valid())
	require.False(t, (Demand{Kind: "DATA", Title: strings.Repeat("中", 121), Description: "x"}).Valid())
	require.False(t, (Demand{Kind: "DATA", Title: "x\n", Description: "x"}).Valid())
	require.False(t, (Progress{Stage: "DELIVERED", Note: ""}).Valid())
}
