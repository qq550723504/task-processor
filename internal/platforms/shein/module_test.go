package shein

import (
	"context"
	"testing"

	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/prompt"
	"task-processor/internal/shared/tenantctx"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type stubTenantPromptStore struct {
	content string
}

func (s stubTenantPromptStore) GetEnabled(_ context.Context, _ string, _ string) (*prompt.TenantPromptTemplate, error) {
	return &prompt.TenantPromptTemplate{
		TenantID: "123",
		Key:      "tmpl.key",
		Content:  s.content,
		Enabled:  true,
	}, nil
}

func (s stubTenantPromptStore) ListTenant(context.Context, string) ([]prompt.TenantPromptTemplate, error) {
	return nil, nil
}

func (s stubTenantPromptStore) SetEnabled(context.Context, string, string, bool) error {
	return nil
}

func (s stubTenantPromptStore) Upsert(context.Context, prompt.TenantPromptTemplate) error {
	return nil
}

func TestConfigureTenantPromptStoreAttachesStoreToGlobalRegistry(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = prompt.NewRegistry(logrus.New().WithField("component", "test"))
	defer func() { prompt.GlobalRegistry = previous }()

	rt := consumerTestRuntimeContext(&config.Config{
		Database: &config.DatabaseConfig{},
	})

	err := configureTenantPromptStore(rt, func(_ *config.DatabaseConfig, _ *logrus.Logger) (prompt.TenantPromptStore, func() error, error) {
		return stubTenantPromptStore{content: "tenant prompt"}, nil, nil
	})
	require.NoError(t, err)

	got, err := prompt.GetTenantFromContext(tenantctx.WithTenantID(context.Background(), "123"), "tmpl.key")
	require.NoError(t, err)
	require.Equal(t, "tenant prompt", got)
}

func consumerTestRuntimeContext(cfg *config.Config) consumer.PlatformRuntimeContext {
	return consumer.PlatformRuntimeContext{
		Config: cfg,
		Logger: logrus.New(),
	}
}
