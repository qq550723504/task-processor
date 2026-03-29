package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewViper_BindsDatabaseEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_DATABASE_HOST", "db.example")
	t.Setenv("TASK_PROCESSOR_DATABASE_PORT", "30432")
	t.Setenv("DB_NAME", "legacy-db")

	v := newViper()

	assert.Equal(t, "db.example", v.GetString("database.host"))
	assert.Equal(t, 30432, v.GetInt("database.port"))
	assert.Equal(t, "legacy-db", v.GetString("database.database"))
}

func TestNewViper_BindsRedisEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_REDIS_HOST", "redis.example")
	t.Setenv("TASK_PROCESSOR_REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "legacy-pass")

	v := newViper()

	assert.Equal(t, "redis.example", v.GetString("redis.host"))
	assert.Equal(t, 6380, v.GetInt("redis.port"))
	assert.Equal(t, "legacy-pass", v.GetString("redis.password"))
}

func TestDeprecatedEnvWarnings_ReportsLegacyAliases(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://legacy")
	t.Setenv("OPENAI_API_KEY", "legacy")

	warnings := deprecatedEnvWarnings()

	assert.NotEmpty(t, warnings)
	assert.Contains(t, strings.Join(warnings, "\n"), "RABBITMQ_URL is deprecated")
	assert.Contains(t, strings.Join(warnings, "\n"), "OPENAI_API_KEY is deprecated")
}

func TestDeprecatedEnvWarnings_SkipsWhenPrimaryEnvSet(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "primary")
	t.Setenv("OPENAI_API_KEY", "legacy")

	warnings := deprecatedEnvWarnings()

	for _, w := range warnings {
		assert.NotContains(t, w, "OPENAI_API_KEY")
	}
}

func TestLoadDotEnvFile_PopulatesUnsetEnvironmentVariables(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	err := os.WriteFile(envPath, []byte(strings.Join([]string{
		"# local development overrides",
		"TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET=from-dotenv",
		"TASK_PROCESSOR_OPENAI_API_KEY=dotenv-openai-key",
		"export TASK_PROCESSOR_OPENAI_BASE_URL=https://example.test/v1",
		"",
	}, "\n")), 0o600)
	require.NoError(t, err)

	t.Setenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET", "")
	require.NoError(t, os.Unsetenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET"))
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "")
	require.NoError(t, os.Unsetenv("TASK_PROCESSOR_OPENAI_API_KEY"))
	t.Setenv("TASK_PROCESSOR_OPENAI_BASE_URL", "")
	require.NoError(t, os.Unsetenv("TASK_PROCESSOR_OPENAI_BASE_URL"))

	require.NoError(t, loadDotEnvFile(envPath))

	assert.Equal(t, "from-dotenv", os.Getenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET"))
	assert.Equal(t, "dotenv-openai-key", os.Getenv("TASK_PROCESSOR_OPENAI_API_KEY"))
	assert.Equal(t, "https://example.test/v1", os.Getenv("TASK_PROCESSOR_OPENAI_BASE_URL"))
}

func TestLoadDotEnvFile_DoesNotOverrideExistingEnvironmentVariables(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	err := os.WriteFile(envPath, []byte("TASK_PROCESSOR_OPENAI_API_KEY=from-dotenv\n"), 0o600)
	require.NoError(t, err)

	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "from-process")

	require.NoError(t, loadDotEnvFile(envPath))

	assert.Equal(t, "from-process", os.Getenv("TASK_PROCESSOR_OPENAI_API_KEY"))
}

func TestLoadFromBytes_AppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET", "env-management-secret")
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "env-openai-key")

	cfg, err := LoadFromBytes([]byte(strings.Join([]string{
		"management:",
		"  clientSecret: \"\"",
		"  scopes: [\"user.read\"]",
		"openai:",
		"  apiKey: \"\"",
		"  model: \"gemini-2.5-flash\"",
		"  baseURL: \"https://api.example.test/v1\"",
		"  timeout: 30",
		"  clients:",
		"    vision:",
		"      model: \"gemini-2.5-flash\"",
		"      timeout: 30",
	}, "\n")))
	require.NoError(t, err)

	assert.Equal(t, "env-management-secret", cfg.Management.ClientSecret)
	assert.Equal(t, "env-openai-key", cfg.OpenAI.APIKey)
	assert.Equal(t, "env-openai-key", cfg.OpenAI.ToClientConfigs()["vision"].APIKey)
}
