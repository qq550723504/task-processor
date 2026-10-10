package collection

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSourceKindAdmitsOnlyCurrentProducers(t *testing.T) {
	for _, kind := range []string{"own", "acquisition", "market", "sds_template", "sds_finished", "amazon_data", "custom_dataset"} {
		require.True(t, ValidSourceKind(kind), kind)
	}
	for _, kind := range []string{"", "manual", "legacy_task", "sds", " MARKET "} {
		require.False(t, ValidSourceKind(kind), kind)
	}
}
