package shein

import (
	"context"
	"testing"

	"task-processor/internal/app/consumer"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/listingruntime"
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

func TestShouldEnableDynamicStoreAssignmentForWorkerRole(t *testing.T) {
	tests := []struct {
		name      string
		autoShard config.AutoShardConfig
		want      bool
	}{
		{
			name:      "legacy auto shard disabled can use dynamic assignment",
			autoShard: config.AutoShardConfig{Enabled: false},
			want:      true,
		},
		{
			name:      "worker role uses dynamic assignment",
			autoShard: config.AutoShardConfig{Enabled: true, Role: config.AutoShardRoleWorker},
			want:      true,
		},
		{
			name:      "empty enabled role defaults coordinator and skips dynamic assignment",
			autoShard: config.AutoShardConfig{Enabled: true},
			want:      false,
		},
		{
			name:      "coordinator role skips dynamic assignment",
			autoShard: config.AutoShardConfig{Enabled: true, Role: config.AutoShardRoleCoordinator},
			want:      false,
		},
		{
			name:      "disabled role skips dynamic assignment",
			autoShard: config.AutoShardConfig{Enabled: true, Role: config.AutoShardRoleDisabled},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				RabbitMQ: &config.RabbitMQConfig{
					Node: config.NodeConfig{
						UseStoreQueues: true,
					},
					AutoShard: tt.autoShard,
				},
				Redis: &config.RedisConfig{},
			}

			require.Equal(t, tt.want, shouldEnableDynamicStoreAssignment(cfg))
		})
	}
}

func TestShouldConfigureAutoShardOnlyForCoordinatorRole(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{
			name: "coordinator",
			role: config.AutoShardRoleCoordinator,
			want: true,
		},
		{
			name: "worker",
			role: config.AutoShardRoleWorker,
			want: false,
		},
		{
			name: "disabled",
			role: config.AutoShardRoleDisabled,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				RabbitMQ: &config.RabbitMQConfig{
					AutoShard: config.AutoShardConfig{
						Enabled: true,
						Role:    tt.role,
					},
				},
			}

			require.Equal(t, tt.want, shouldConfigureAutoShard(cfg))
		})
	}
}

func TestHasEnabledSheinScheduledTaskConfigsUsesRuntimeConfigs(t *testing.T) {
	runtime := &stubSheinSchedulerRuntime{
		configs: map[appscheduler.TaskType][]listingruntime.ScheduledTaskConfig{
			appscheduler.TaskTypeProductSync: {
				{StoreID: 870, Platform: "shein", TaskType: string(appscheduler.TaskTypeProductSync), Enabled: true},
			},
		},
	}

	require.True(t, hasEnabledSheinScheduledTaskConfigs(context.Background(), runtime, logrus.New()))
	require.Equal(t, []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
	}, runtime.calls)
}

func TestHasEnabledSheinScheduledTaskConfigsReturnsFalseWithoutRuntimeConfigs(t *testing.T) {
	runtime := &stubSheinSchedulerRuntime{}

	require.False(t, hasEnabledSheinScheduledTaskConfigs(context.Background(), runtime, logrus.New()))
	require.Equal(t, []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	}, runtime.calls)
}

func TestCoordinatorRoleDoesNotUseDynamicStoreAssignment(t *testing.T) {
	cfg := &config.Config{
		RabbitMQ: &config.RabbitMQConfig{
			Node: config.NodeConfig{
				UseStoreQueues: true,
			},
			AutoShard: config.AutoShardConfig{
				Enabled: true,
				Role:    config.AutoShardRoleCoordinator,
			},
		},
		Redis: &config.RedisConfig{},
	}

	require.False(t, shouldEnableDynamicStoreAssignment(cfg))
	require.True(t, shouldConfigureAutoShard(cfg))
}

func TestDedicatedStaticStoreDoesNotUseDynamicAssignment(t *testing.T) {
	cfg := &config.Config{
		RabbitMQ: &config.RabbitMQConfig{
			Node: config.NodeConfig{
				UseStoreQueues: true,
				OwnedStores:    []int64{976},
			},
			AutoShard: config.AutoShardConfig{
				Enabled: false,
				Role:    config.AutoShardRoleDisabled,
			},
		},
		Redis: &config.RedisConfig{},
	}

	require.False(t, shouldEnableDynamicStoreAssignment(cfg))
	require.False(t, shouldConfigureAutoShard(cfg))
}

func consumerTestRuntimeContext(cfg *config.Config) consumer.PlatformRuntimeContext {
	return consumer.BuildPlatformRuntimeContext(consumer.PlatformRuntimeContextInput{
		Config: cfg,
		Logger: logrus.New(),
	})
}

type stubSheinSchedulerRuntime struct {
	configs map[appscheduler.TaskType][]listingruntime.ScheduledTaskConfig
	calls   []appscheduler.TaskType
}

func (s *stubSheinSchedulerRuntime) GetRuntimeStoreService() listingruntime.StoreService {
	return nil
}

func (s *stubSheinSchedulerRuntime) ListRuntimeAutoPricingStoreIDs(context.Context, string) ([]int64, error) {
	return nil, nil
}

func (s *stubSheinSchedulerRuntime) ListRuntimeScheduledTaskConfigs(
	_ context.Context,
	_ string,
	taskType appscheduler.TaskType,
) ([]listingruntime.ScheduledTaskConfig, error) {
	s.calls = append(s.calls, taskType)
	return s.configs[taskType], nil
}

func (s *stubSheinSchedulerRuntime) ListRuntimeScheduledTaskConfigStates(
	ctx context.Context,
	platform string,
	taskType appscheduler.TaskType,
) ([]listingruntime.ScheduledTaskConfig, error) {
	return s.ListRuntimeScheduledTaskConfigs(ctx, platform, taskType)
}
