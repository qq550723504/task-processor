package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewViper_BindsPrimaryEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_MANAGEMENT_TENANT_ID", "tenant-123")
	t.Setenv("TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_ID", "amzn-client")
	t.Setenv("TASK_PROCESSOR_AMAZON_SPAPI_DEFAULT_MARKETPLACE", "ATVPDKIKX0DER")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_HEALTH_CHECK_PORT", "18081")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_METRICS_PORT", "19090")

	v := newViper()

	assert.Equal(t, "tenant-123", v.GetString("management.tenantID"))
	assert.Equal(t, "amzn-client", v.GetString("amazon.spapi.clientID"))
	assert.Equal(t, "ATVPDKIKX0DER", v.GetString("amazon.spapi.defaultMarketplace"))
	assert.Equal(t, 18081, v.GetInt("rabbitmq.node.healthCheckPort"))
	assert.Equal(t, 19090, v.GetInt("rabbitmq.node.metricsPort"))
}

func TestNewViper_BindsLegacyEnvironmentAliases(t *testing.T) {
	t.Setenv("AMAZON_SPAPI_CLIENT_SECRET", "legacy-secret")
	t.Setenv("AMAZON_SPAPI_MARKETPLACE_ID", "LEGACY-MARKET")
	t.Setenv("HEALTH_CHECK_PORT", "28081")
	t.Setenv("METRICS_PORT", "29090")

	v := newViper()

	assert.Equal(t, "legacy-secret", v.GetString("amazon.spapi.clientSecret"))
	assert.Equal(t, "LEGACY-MARKET", v.GetString("amazon.spapi.defaultMarketplace"))
	assert.Equal(t, 28081, v.GetInt("rabbitmq.node.healthCheckPort"))
	assert.Equal(t, 29090, v.GetInt("rabbitmq.node.metricsPort"))
}

func TestDeprecatedEnvWarnings_ReportsLegacyAliases(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://legacy")
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "primary")
	t.Setenv("OPENAI_API_KEY", "legacy")

	warnings := deprecatedEnvWarnings()

	assert.NotEmpty(t, warnings)
	assert.Contains(t, strings.Join(warnings, "\n"), "RABBITMQ_URL is deprecated")
	assert.Contains(t, strings.Join(warnings, "\n"), "OPENAI_API_KEY is deprecated")
	assert.Contains(t, strings.Join(warnings, "\n"), "TASK_PROCESSOR_OPENAI_API_KEY takes precedence")
}
