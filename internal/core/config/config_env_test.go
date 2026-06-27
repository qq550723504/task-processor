package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConfigSource struct {
	name string
	data []byte
}

func (s stubConfigSource) Read() ([]byte, error) { return s.data, nil }

func (s stubConfigSource) Watch(_ context.Context, _ func([]byte)) error { return nil }

func (s stubConfigSource) Name() string { return s.name }

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
	t.Setenv("TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_TARGET_STORES_PER_NODE", "3")
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
	assert.Equal(t, 3, v.GetInt("rabbitmq.autoShard.targetStoresPerNode"))
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

func TestNewViperBindsProcessingTimeoutWatchdogEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_ENABLED", "true")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_INTERVAL_SECONDS", "300")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_TIMEOUT_MINUTES", "30")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_RECOVERY_LIMIT", "100")

	v := newViper()

	assert.True(t, v.GetBool("rabbitmq.processingTimeoutWatchdog.enabled"))
	assert.Equal(t, 300, v.GetInt("rabbitmq.processingTimeoutWatchdog.intervalSeconds"))
	assert.Equal(t, 30, v.GetInt("rabbitmq.processingTimeoutWatchdog.timeoutMinutes"))
	assert.Equal(t, 100, v.GetInt("rabbitmq.processingTimeoutWatchdog.recoveryLimit"))
}

func TestNewViperBindsStaleQueuedWatchdogEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_ENABLED", "true")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_INTERVAL_SECONDS", "300")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_TIMEOUT_MINUTES", "120")
	t.Setenv("TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_RECOVERY_LIMIT", "500")

	v := newViper()

	assert.True(t, v.GetBool("rabbitmq.staleQueuedWatchdog.enabled"))
	assert.Equal(t, 300, v.GetInt("rabbitmq.staleQueuedWatchdog.intervalSeconds"))
	assert.Equal(t, 120, v.GetInt("rabbitmq.staleQueuedWatchdog.timeoutMinutes"))
	assert.Equal(t, 500, v.GetInt("rabbitmq.staleQueuedWatchdog.recoveryLimit"))
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

func TestNewViper_BindsSheinCookieRedisEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST", "login-redis.example")
	t.Setenv("TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT", "6379")
	t.Setenv("TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PASSWORD", "cookie-pass")
	t.Setenv("TASK_PROCESSOR_SHEIN_COOKIE_REDIS_DB", "9")

	v := newViper()

	assert.Equal(t, "login-redis.example", v.GetString("platforms.shein.cookieRedis.host"))
	assert.Equal(t, 6379, v.GetInt("platforms.shein.cookieRedis.port"))
	assert.Equal(t, "cookie-pass", v.GetString("platforms.shein.cookieRedis.password"))
	assert.Equal(t, 9, v.GetInt("platforms.shein.cookieRedis.db"))
}

func TestNewViper_BindsSDSLoginServiceEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SDS_LOGIN_SERVICE_BASE_URL", "http://login.svc:8000")
	t.Setenv("TASK_PROCESSOR_SDS_LOGIN_SERVICE_SHARED_KEY", "sds-key")
	t.Setenv("TASK_PROCESSOR_SDS_LOGIN_SERVICE_TENANT_ID", "manual")
	t.Setenv("TASK_PROCESSOR_SDS_LOGIN_SERVICE_IDENTIFIER", "default")
	t.Setenv("TASK_PROCESSOR_SDS_MERCHANT_NAME", "merchant")
	t.Setenv("TASK_PROCESSOR_SDS_USERNAME", "tester")
	t.Setenv("TASK_PROCESSOR_SDS_PASSWORD", "secret")
	t.Setenv("TASK_PROCESSOR_SDS_LOGIN_SERVICE_DEFAULT_HEADLESS", "true")
	t.Setenv("TASK_PROCESSOR_SDS_LOGIN_SERVICE_CLOAKBROWSER_ENABLED", "true")
	t.Setenv("TASK_PROCESSOR_SDS_LOGIN_SERVICE_CLOAKBROWSER_PATH", "C:/cloak/chrome.exe")

	v := newViper()

	assert.Equal(t, "http://login.svc:8000", v.GetString("platforms.sds.loginService.baseURL"))
	assert.Equal(t, "sds-key", v.GetString("platforms.sds.loginService.sharedKey"))
	assert.Equal(t, "manual", v.GetString("platforms.sds.loginService.tenantID"))
	assert.Equal(t, "default", v.GetString("platforms.sds.loginService.identifier"))
	assert.Equal(t, "merchant", v.GetString("platforms.sds.loginService.merchantName"))
	assert.Equal(t, "tester", v.GetString("platforms.sds.loginService.username"))
	assert.Equal(t, "secret", v.GetString("platforms.sds.loginService.password"))
	assert.True(t, v.GetBool("platforms.sds.loginService.defaultHeadless"))
	assert.True(t, v.GetBool("platforms.sds.loginService.cloakBrowserEnabled"))
	assert.Equal(t, "C:/cloak/chrome.exe", v.GetString("platforms.sds.loginService.cloakBrowserPath"))
}

func TestNewViper_BindsSDSAuthRedisEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SDS_AUTH_REDIS_HOST", "sds-redis.example")
	t.Setenv("TASK_PROCESSOR_SDS_AUTH_REDIS_PORT", "6381")
	t.Setenv("TASK_PROCESSOR_SDS_AUTH_REDIS_PASSWORD", "sds-pass")
	t.Setenv("TASK_PROCESSOR_SDS_AUTH_REDIS_DB", "9")
	t.Setenv("TASK_PROCESSOR_SDS_AUTH_REDIS_POOL_SIZE", "16")

	v := newViper()

	assert.Equal(t, "sds-redis.example", v.GetString("platforms.sds.authRedis.host"))
	assert.Equal(t, 6381, v.GetInt("platforms.sds.authRedis.port"))
	assert.Equal(t, "sds-pass", v.GetString("platforms.sds.authRedis.password"))
	assert.Equal(t, 9, v.GetInt("platforms.sds.authRedis.db"))
	assert.Equal(t, 16, v.GetInt("platforms.sds.authRedis.pool_size"))
}

func TestNewViper_BindsSDSAuthBootstrapEnvironmentVariables(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SDS_ACCESS_TOKEN", "access-token")
	t.Setenv("TASK_PROCESSOR_SDS_OUT_ACCESS_TOKEN", "out-token")
	t.Setenv("TASK_PROCESSOR_SDS_MERCHANT_ID", "12345")
	t.Setenv("TASK_PROCESSOR_SDS_COOKIE", "cookie=value")
	t.Setenv("TASK_PROCESSOR_SDS_DOMAIN_NAME", "www.sdsdiy.com")
	t.Setenv("TASK_PROCESSOR_SDS_VERIFY_CAPTCHA_PARAM", "captcha-param")
	t.Setenv("TASK_PROCESSOR_SDS_EXTRA_INFO", "{\"risk\":1}")

	v := newViper()

	assert.Equal(t, "access-token", v.GetString("platforms.sds.authBootstrap.staticAccessToken"))
	assert.Equal(t, "out-token", v.GetString("platforms.sds.authBootstrap.staticOutToken"))
	assert.Equal(t, int64(12345), v.GetInt64("platforms.sds.authBootstrap.staticMerchantID"))
	assert.Equal(t, "cookie=value", v.GetString("platforms.sds.authBootstrap.staticCookie"))
	assert.Equal(t, "www.sdsdiy.com", v.GetString("platforms.sds.authBootstrap.loginDomainName"))
	assert.Equal(t, "captcha-param", v.GetString("platforms.sds.authBootstrap.loginVerifyCaptchaParam"))
	assert.Equal(t, "{\"risk\":1}", v.GetString("platforms.sds.authBootstrap.loginExtraInfo"))
}

func TestNewViper_BindsProductEnrichMockLLMEnvironmentVariable(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_PRODUCTENRICH_MOCK_LLM", "true")

	v := newViper()

	assert.True(t, v.GetBool("debug.productEnrichMockLLM"))
}

func TestNewViper_BindsListingKitEnvironmentVariables(t *testing.T) {
	t.Setenv("LISTINGKIT_DEBUG_SUBMIT_DUMP_DIR", "D:/tmp/shein-submit-dumps")
	t.Setenv("LISTINGKIT_PLATFORM_ADMIN_USERS", "user-a,user-b")
	t.Setenv("LISTINGKIT_PLATFORM_ADMIN_ROLES", "role-a,role-b")
	t.Setenv("ZITADEL_ISSUER_URL", "https://issuer.example")
	t.Setenv("ZITADEL_CLIENT_ID", "listingkit-client")
	t.Setenv("ZITADEL_CLIENT_SECRET", "listingkit-secret")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS", "tenant-a,tenant-b")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS", "user-a,user-b")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES", "alice,bob")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES", "listingkit_admin,platform_admin")

	v := newViper()

	assert.Equal(t, "D:/tmp/shein-submit-dumps", v.GetString("listingkit.sheinSubmitDebugDumpDir"))
	assert.Equal(t, []string{"user-a", "user-b"}, getStringSlice(v, "listingkit.platformAdminUsers"))
	assert.Equal(t, []string{"role-a", "role-b"}, getStringSlice(v, "listingkit.platformAdminRoles"))
	assert.False(t, v.IsSet("listingkit.ownerScopeRequired"))
	assert.Equal(t, "https://issuer.example", v.GetString("listingkit.zitadel.issuerURL"))
	assert.Equal(t, "listingkit-client", v.GetString("listingkit.zitadel.clientID"))
	assert.Equal(t, "listingkit-secret", v.GetString("listingkit.zitadel.clientSecret"))
	assert.False(t, v.IsSet("listingkit.zitadel.authRequired"))
	assert.False(t, v.IsSet("listingkit.zitadel.authorizationRequired"))
	assert.Equal(t, []string{"tenant-a", "tenant-b"}, getStringSlice(v, "listingkit.zitadel.allowedTenantIDs"))
	assert.Equal(t, []string{"user-a", "user-b"}, getStringSlice(v, "listingkit.zitadel.allowedUserIDs"))
	assert.Equal(t, []string{"alice", "bob"}, getStringSlice(v, "listingkit.zitadel.allowedUsernames"))
	assert.Equal(t, []string{"listingkit_admin", "platform_admin"}, getStringSlice(v, "listingkit.zitadel.allowedRoles"))
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

func TestLoadFromBytes_AppliesViperManagedEnvironmentOverrides(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_BROWSER_HEADLESS", "false")
	t.Setenv("TASK_PROCESSOR_BROWSER_POOL_SIZE", "7")
	t.Setenv("TASK_PROCESSOR_WORKER_CONCURRENCY", "22")
	t.Setenv("TASK_PROCESSOR_PLATFORM_TEMU_ENABLED", "true")
	t.Setenv("TASK_PROCESSOR_REDIS_HOST", "redis.env.local")
	t.Setenv("TASK_PROCESSOR_REDIS_PORT", "6381")

	cfg, err := LoadFromBytes([]byte(strings.Join([]string{
		"management:",
		"  clientSecret: \"test-secret\"",
		"  scopes: [\"user.read\"]",
		"openai:",
		"  apiKey: \"test-openai-key\"",
		"  model: \"gemini-2.5-flash\"",
		"  baseURL: \"https://api.example.test/v1\"",
		"  timeout: 30",
		"browser:",
		"  headless: true",
		"  poolSize: 1",
		"worker:",
		"  concurrency: 1",
		"  bufferSize: 10",
		"  taskInterval: 60",
		"platforms:",
		"  temu:",
		"    enabled: false",
	}, "\n")))
	require.NoError(t, err)

	assert.False(t, cfg.Browser.Headless)
	assert.Equal(t, 7, cfg.Browser.PoolSize)
	assert.Equal(t, 22, cfg.Worker.Concurrency)
	assert.True(t, cfg.Platforms.Temu.Enabled)
	require.NotNil(t, cfg.Redis)
	assert.Equal(t, "redis.env.local", cfg.Redis.Host)
	assert.Equal(t, 6381, cfg.Redis.Port)
}

func TestConfigManagerLoad_UsesSameEnvironmentOverrideBehavior(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_BROWSER_HEADLESS", "false")
	t.Setenv("TASK_PROCESSOR_WORKER_CONCURRENCY", "19")
	t.Setenv("TASK_PROCESSOR_PLATFORM_TEMU_ENABLED", "true")

	mgr := NewConfigManager(logrus.New())
	source := stubConfigSource{
		name: "memory",
		data: []byte(strings.Join([]string{
			"management:",
			"  clientSecret: \"test-secret\"",
			"  scopes: [\"user.read\"]",
			"openai:",
			"  apiKey: \"test-openai-key\"",
			"  model: \"gemini-2.5-flash\"",
			"  baseURL: \"https://api.example.test/v1\"",
			"  timeout: 30",
			"browser:",
			"  headless: true",
			"worker:",
			"  concurrency: 1",
			"  bufferSize: 10",
			"  taskInterval: 60",
			"platforms:",
			"  temu:",
			"    enabled: false",
		}, "\n")),
	}

	cfg, err := mgr.Load(source)
	require.NoError(t, err)

	assert.False(t, cfg.Browser.Headless)
	assert.Equal(t, 19, cfg.Worker.Concurrency)
	assert.True(t, cfg.Platforms.Temu.Enabled)
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

func TestLoadConfigFromFile_AssemblesListingKitAndZitadelConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config-test.yaml")
	configBody := strings.Join([]string{
		"management:",
		"  baseURL: \"http://example.com\"",
		"  clientID: \"test-client\"",
		"  clientSecret: \"test-secret\"",
		"  tokenURL: \"http://example.com/token\"",
		"  scopes: [\"user.read\"]",
		"openai:",
		"  apiKey: \"test-openai-key\"",
		"  model: \"gemini-2.5-flash\"",
		"  baseURL: \"https://api.example.test/v1\"",
		"  timeout: 30",
		"listingkit:",
		"  sheinSubmitDebugDumpDir: \"./.local/tmp/shein-dumps\"",
		"  platformAdminUsers: [\"user-a\"]",
		"  platformAdminRoles: [\"platform_admin\"]",
		"  ownerScopeRequired: false",
		"  zitadel:",
		"    issuerURL: \"https://issuer.file.example\"",
		"    clientID: \"file-client\"",
		"    clientSecret: \"file-secret\"",
		"    authRequired: false",
		"    authorizationRequired: false",
		"    allowedUsernames: [\"file-admin\"]",
	}, "\n")
	require.NoError(t, os.WriteFile(configPath, []byte(configBody), 0o600))

	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES", "listingkit_admin,platform_admin")
	t.Setenv("LISTINGKIT_PLATFORM_ADMIN_USERS", "user-b,user-c")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_OWNER_SCOPE_REQUIRED", "1")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED", "1")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED", "1")

	cfg, err := LoadConfigFromFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, "./.local/tmp/shein-dumps", cfg.ListingKit.SheinSubmitDebugDumpDir)
	assert.Equal(t, []string{"user-b", "user-c"}, cfg.ListingKit.PlatformAdminUsers)
	assert.Equal(t, []string{"platform_admin"}, cfg.ListingKit.PlatformAdminRoles)
	assert.True(t, cfg.ListingKit.OwnerScopeRequired)
	assert.False(t, cfg.ListingKit.Zitadel.AuthRequired)
	assert.True(t, cfg.ListingKit.Zitadel.AuthorizationRequired)
	assert.Equal(t, "https://issuer.file.example", cfg.ListingKit.Zitadel.IssuerURL)
	assert.Equal(t, "file-client", cfg.ListingKit.Zitadel.ClientID)
	assert.Equal(t, []string{"file-admin"}, cfg.ListingKit.Zitadel.AllowedUsernames)
	assert.Equal(t, []string{"listingkit_admin", "platform_admin"}, cfg.ListingKit.Zitadel.AllowedRoles)
}
