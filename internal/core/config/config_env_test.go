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
	t.Setenv("TASK_PROCESSOR_MANAGEMENT_STORE_IDS", "101, 202,303")
	t.Setenv("TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_ID", "amzn-client")
	t.Setenv("TASK_PROCESSOR_AMAZON_SPAPI_DEFAULT_MARKETPLACE", "ATVPDKIKX0DER")
	t.Setenv("TASK_PROCESSOR_AMAZON_REMOTE_API_BASE_URL", "http://crawler.internal:8080")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_HEALTH_CHECK_PORT", "18081")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_METRICS_PORT", "19090")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID", "shein-store-a")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_OWNED_STORES", "101, 202")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_OWNED_BUCKETS", "0, 3")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_USE_STORE_QUEUES", "true")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED", "true")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_PLATFORM", "shein")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES", "shein-store-a, shein-store-b")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY", "image-key")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL", "https://image.example.test/v1")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE", "nanobanana")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL", "nano-banana-fast")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_TIMEOUT", "300")

	v := newViper()

	assert.Equal(t, "tenant-123", v.GetString("management.tenantID"))
	assert.Equal(t, []int64{101, 202, 303}, getInt64Slice(v, "management.storeIDs"))
	assert.Equal(t, "amzn-client", v.GetString("amazon.spapi.clientID"))
	assert.Equal(t, "ATVPDKIKX0DER", v.GetString("amazon.spapi.defaultMarketplace"))
	assert.Equal(t, "http://crawler.internal:8080", v.GetString("amazon.remoteAPI.baseURL"))
	assert.Equal(t, "shein-store-a", v.GetString("rabbitmq.node.nodeID"))
	assert.Equal(t, []int64{101, 202}, getInt64Slice(v, "rabbitmq.node.ownedStores"))
	assert.Equal(t, []int{0, 3}, getIntSlice(v, "rabbitmq.node.ownedBuckets"))
	assert.True(t, v.GetBool("rabbitmq.node.useStoreQueues"))
	assert.True(t, v.GetBool("rabbitmq.autoShard.enabled"))
	assert.Equal(t, "shein", v.GetString("rabbitmq.autoShard.platform"))
	assert.Equal(t, []string{"shein-store-a", "shein-store-b"}, getStringSlice(v, "rabbitmq.autoShard.candidateNodes"))
	assert.Equal(t, 18081, v.GetInt("rabbitmq.node.healthCheckPort"))
	assert.Equal(t, 19090, v.GetInt("rabbitmq.node.metricsPort"))
	assert.Equal(t, "image-key", v.GetString("openai.clients.image.apiKey"))
	assert.Equal(t, "https://image.example.test/v1", v.GetString("openai.clients.image.baseURL"))
	assert.Equal(t, "nanobanana", v.GetString("openai.clients.image.apiStyle"))
	assert.Equal(t, "nano-banana-fast", v.GetString("openai.clients.image.model"))
	assert.Equal(t, 300, v.GetInt("openai.clients.image.timeout"))
}

func TestGetStringSlice_SplitsCommaSeparatedSingleEntry(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES", "shein-store-a,shein-store-b,shein-store-c,shein-store-d")

	v := newViper()

	assert.Equal(t,
		[]string{"shein-store-a", "shein-store-b", "shein-store-c", "shein-store-d"},
		getStringSlice(v, "rabbitmq.autoShard.candidateNodes"),
	)
}

func TestNewViper_BindsLegacyEnvironmentAliases(t *testing.T) {
	t.Setenv("AMAZON_SPAPI_CLIENT_SECRET", "legacy-secret")
	t.Setenv("AMAZON_SPAPI_MARKETPLACE_ID", "LEGACY-MARKET")
	t.Setenv("HEALTH_CHECK_PORT", "28081")
	t.Setenv("METRICS_PORT", "29090")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_APIKEY", "legacy-image-key")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASEURL", "https://legacy-image.example.test/v1")
	t.Setenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_APISTYLE", "legacy-style")

	v := newViper()

	assert.Equal(t, "legacy-secret", v.GetString("amazon.spapi.clientSecret"))
	assert.Equal(t, "LEGACY-MARKET", v.GetString("amazon.spapi.defaultMarketplace"))
	assert.Equal(t, 28081, v.GetInt("rabbitmq.node.healthCheckPort"))
	assert.Equal(t, 29090, v.GetInt("rabbitmq.node.metricsPort"))
	assert.Equal(t, "legacy-image-key", v.GetString("openai.clients.image.apiKey"))
	assert.Equal(t, "https://legacy-image.example.test/v1", v.GetString("openai.clients.image.baseURL"))
	assert.Equal(t, "legacy-style", v.GetString("openai.clients.image.apiStyle"))
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
	t.Setenv("TASK_PROCESSOR_MANAGEMENT_STORE_IDS", "11,22")
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "env-openai-key")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID", "store-shard-a")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_OWNED_STORES", "3001,3002")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_NODE_USE_STORE_QUEUES", "true")

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
		"rabbitmq:",
		"  enabled: true",
		"  node:",
		"    maxConcurrency: 1",
		"    healthCheckPort: 8081",
		"    metricsPort: 8082",
	}, "\n")))
	require.NoError(t, err)

	assert.Equal(t, "env-management-secret", cfg.Management.ClientSecret)
	assert.Equal(t, []int64{11, 22}, cfg.Management.StoreIDs)
	assert.Equal(t, "env-openai-key", cfg.OpenAI.APIKey)
	assert.Equal(t, "env-openai-key", cfg.OpenAI.ToClientConfigs()["vision"].APIKey)
	assert.Equal(t, "store-shard-a", cfg.RabbitMQ.Node.NodeID)
	assert.Equal(t, []int64{3001, 3002}, cfg.RabbitMQ.Node.OwnedStores)
	assert.True(t, cfg.RabbitMQ.Node.UseStoreQueues)
}

func TestDotEnvCandidatesForConfig_PrioritizesScopedEnvFile(t *testing.T) {
	candidates := dotEnvCandidatesForConfig(filepath.Join("config", "config-amazon-crawler-api.yaml"))

	require.NotEmpty(t, candidates)
	assert.Equal(t, filepath.Clean(filepath.Join("config", ".env.config-amazon-crawler-api")), candidates[0])
	assert.Equal(t, filepath.Clean(filepath.Join("config", "config-amazon-crawler-api.env")), candidates[1])
}

func TestLoadDotEnvCandidates_LoadsScopedEnvBeforeSharedDotEnv(t *testing.T) {
	resetDotEnvLoadStateForTests()
	t.Cleanup(func() {
		resetDotEnvLoadStateForTests()
		_ = os.Unsetenv("TASK_PROCESSOR_PLATFORM_1688_ENABLED")
		_ = os.Unsetenv("TASK_PROCESSOR_OPENAI_API_KEY")
	})

	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	sharedEnvPath := filepath.Join(tempDir, ".env")
	scopedEnvPath := filepath.Join(configDir, ".env.config-amazon-crawler-api")

	require.NoError(t, os.WriteFile(sharedEnvPath, []byte(strings.Join([]string{
		"TASK_PROCESSOR_PLATFORM_1688_ENABLED=true",
		"TASK_PROCESSOR_OPENAI_API_KEY=shared-key",
	}, "\n")), 0o600))
	require.NoError(t, os.WriteFile(scopedEnvPath, []byte(strings.Join([]string{
		"TASK_PROCESSOR_PLATFORM_1688_ENABLED=false",
	}, "\n")), 0o600))

	_ = os.Unsetenv("TASK_PROCESSOR_PLATFORM_1688_ENABLED")
	_ = os.Unsetenv("TASK_PROCESSOR_OPENAI_API_KEY")

	loadDotEnvCandidates([]string{scopedEnvPath, sharedEnvPath})

	assert.Equal(t, "false", os.Getenv("TASK_PROCESSOR_PLATFORM_1688_ENABLED"))
	assert.Equal(t, "shared-key", os.Getenv("TASK_PROCESSOR_OPENAI_API_KEY"))
}
