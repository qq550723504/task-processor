package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestConfigHealthCheckDoesNotRequireRetiredManagementServiceURL(t *testing.T) {
	check := &ConfigHealthCheck{config: &config.Config{
		Worker: config.WorkerConfig{Concurrency: 1},
	}}

	require.NoError(t, check.Check(context.Background()))
}
