package aicapability

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestModelPointPricingIntegerSumCeilingAndOverflow(t *testing.T) {
	price := ModelPointTariff{PriceVersion: "synthetic-v1", InputPointsPerMillionTokens: 3, OutputPointsPerMillionTokens: 7}
	points, err := price.Points(500000, 500000)
	require.NoError(t, err)
	require.EqualValues(t, 5, points)
	points, err = price.Points(1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, points)
	_, err = price.Points(-1, 1)
	require.Error(t, err)
	price.InputPointsPerMillionTokens = math.MaxInt64
	_, err = price.Points(math.MaxInt64, 1)
	require.Error(t, err)
	price.PriceVersion = ""
	_, err = price.Points(1, 1)
	require.Error(t, err)
}
