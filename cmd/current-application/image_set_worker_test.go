package main

import (
	"github.com/stretchr/testify/require"
	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/core/config"
	"testing"
)

func TestFullImageWorkerCannotUseTheHTTPRoleOrAnotherOwner(t *testing.T) {
	expected := currentapplication.DatabaseConfig{Host: "127.0.0.1", Port: 5432, Database: "image"}
	valid := config.DatabaseConfig{Host: expected.Host, Port: expected.Port, Database: expected.Database, User: "image_agent_worker_runtime", MaxConnections: 4}
	cfg := &config.Config{Database: &valid}
	require.NoError(t, validateImageSetWorkerDatabase(cfg, expected))
	for _, mutate := range []func(*config.DatabaseConfig){func(d *config.DatabaseConfig) { d.User = "image_agent_runtime" }, func(d *config.DatabaseConfig) { d.Database = "other" }, func(d *config.DatabaseConfig) { d.MaxConnections = 20 }} {
		changed := valid
		mutate(&changed)
		cfg.Database = &changed
		require.Error(t, validateImageSetWorkerDatabase(cfg, expected))
	}
}
