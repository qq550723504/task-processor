package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRabbitMQAutoShardRoleEnvironmentVariable(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE", "worker")

	v := newViper()
	cfg := BuildRabbitMQConfig(v)

	assert.Equal(t, AutoShardRoleWorker, cfg.AutoShard.Role)
}

func TestAutoShardEffectiveRoleDefaults(t *testing.T) {
	tests := []struct {
		name        string
		config      AutoShardConfig
		effective   string
		valid       bool
		coordinator bool
		worker      bool
	}{
		{
			name:      "disabled ignores configured role",
			config:    AutoShardConfig{Enabled: false, Role: "worker"},
			effective: AutoShardRoleDisabled,
			valid:     true,
		},
		{
			name:        "enabled empty role defaults coordinator",
			config:      AutoShardConfig{Enabled: true},
			effective:   AutoShardRoleCoordinator,
			valid:       true,
			coordinator: true,
		},
		{
			name:      "normalizes configured role",
			config:    AutoShardConfig{Enabled: true, Role: " Worker "},
			effective: AutoShardRoleWorker,
			valid:     true,
			worker:    true,
		},
		{
			name:      "invalid role returns normalized value",
			config:    AutoShardConfig{Enabled: true, Role: " Controller "},
			effective: "controller",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.effective, tt.config.EffectiveRole())
			assert.Equal(t, tt.valid, tt.config.HasValidRole())
			assert.Equal(t, tt.coordinator, tt.config.IsCoordinator())
			assert.Equal(t, tt.worker, tt.config.IsWorker())
		})
	}
}

func TestValidateRabbitMQRejectsInvalidAutoShardRole(t *testing.T) {
	cfg := validRabbitMQConfigWithAutoShard()
	cfg.AutoShard.Role = "leader"

	errors := ValidateRabbitMQConfig(&cfg)

	assert.Contains(t, validationErrorFields(errors), "rabbitmq.autoShard.role")
}

func TestValidateRabbitMQAllowsWorkerWithoutCandidateNodes(t *testing.T) {
	cfg := validRabbitMQConfigWithAutoShard()
	cfg.AutoShard.Role = AutoShardRoleWorker
	cfg.AutoShard.CandidateNodes = nil

	errors := ValidateRabbitMQConfig(&cfg)

	assert.NotContains(t, validationErrorFields(errors), "rabbitmq.autoShard.candidateNodes")
}

func TestValidateRabbitMQAllowsDisabledRoleWithoutCandidateNodes(t *testing.T) {
	cfg := validRabbitMQConfigWithAutoShard()
	cfg.AutoShard.Role = AutoShardRoleDisabled
	cfg.AutoShard.CandidateNodes = nil

	errors := ValidateRabbitMQConfig(&cfg)

	assert.NotContains(t, validationErrorFields(errors), "rabbitmq.autoShard.candidateNodes")
}

func TestValidateRabbitMQAutoShardTimingStillRequiredForNonCoordinatorRoles(t *testing.T) {
	for _, role := range []string{AutoShardRoleWorker, AutoShardRoleDisabled} {
		t.Run(role, func(t *testing.T) {
			cfg := validRabbitMQConfigWithAutoShard()
			cfg.AutoShard.Role = role
			cfg.AutoShard.CandidateNodes = nil
			cfg.AutoShard.Interval = 0
			cfg.AutoShard.LockTTL = 0

			errors := ValidateRabbitMQConfig(&cfg)

			fields := validationErrorFields(errors)
			assert.Contains(t, fields, "rabbitmq.autoShard.interval")
			assert.Contains(t, fields, "rabbitmq.autoShard.lockTTL")
		})
	}
}

func validRabbitMQConfigWithAutoShard() RabbitMQConfig {
	return RabbitMQConfig{
		Enabled: true,
		URL:     "amqp://guest:guest@localhost:5672/",
		Consumer: RabbitMQConsumerConfig{
			PrefetchCount: 1,
			MaxRetries:    0,
		},
		Node: NodeConfig{
			MaxConcurrency:  1,
			HealthCheckPort: 1,
			MetricsPort:     1,
		},
		AutoShard: AutoShardConfig{
			Enabled:        true,
			Interval:       time.Second,
			LockTTL:        time.Second,
			CandidateNodes: []string{"shein-listing-store-a"},
		},
	}
}

func validationErrorFields(errors []error) []string {
	fields := make([]string, 0, len(errors))
	for _, err := range errors {
		if validationErr, ok := err.(*ValidationError); ok {
			fields = append(fields, validationErr.Field)
		}
	}
	return fields
}
