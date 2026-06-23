package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/watermark"

	"github.com/spf13/viper"
)

type envBinding struct {
	Primary    string
	Deprecated []string
}

var (
	loadDotEnvMu      sync.Mutex
	loadedDotEnvPaths = map[string]struct{}{}
)

type Config struct {
	Logging             LoggingConfig             `yaml:"logging"`
	Processor           ProcessorConfig           `yaml:"processor"`
	Worker              WorkerConfig              `yaml:"worker"`
	OpenAI              OpenAIConfig              `yaml:"openai"`
	Management          ManagementConfig          `yaml:"management"`
	Browser             BrowserConfig             `yaml:"browser"`
	Amazon              AmazonConfig              `yaml:"amazon"`
	RabbitMQ            *RabbitMQConfig           `yaml:"rabbitmq"`
	Updater             UpdaterConfig             `yaml:"updater"`
	Platforms           PlatformsConfig           `yaml:"platforms"`
	Watermark           *watermark.Config         `yaml:"watermark"`
	ProductImage        ProductImageConfig        `yaml:"productimage"`
	Database            *DatabaseConfig           `yaml:"database"`
	Redis               *RedisConfig              `yaml:"redis"`
	Prompts             PromptsConfig             `yaml:"prompts"`
	Debug               DebugConfig               `yaml:"debug"`
	ListingKit          ListingKitConfig          `yaml:"listingkit"`
	ListingControlPlane ListingControlPlaneConfig `yaml:"listingControlPlane"`
}

type DebugConfig struct {
	SavePublishJSON      bool `yaml:"save_publish_json"`
	ProductEnrichMockLLM bool `yaml:"productEnrichMockLLM"`
}

type PromptsConfig struct {
	Dir       string `yaml:"dir"`
	HotReload bool   `yaml:"hotReload"`
}

type LoggingConfig struct {
	Level        string                   `yaml:"level"`
	Format       string                   `yaml:"format"`
	File         string                   `yaml:"file"`
	SplitByLevel []logger.LevelFileConfig `yaml:"split_by_level"`
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("TASK_PROCESSOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindKnownEnvs(v)
	setDefaults(v)
	return v
}

func tryLoadDotEnv() {
	loadDotEnvCandidates(dotEnvCandidates())
}

func tryLoadDotEnvForConfig(configFile string) {
	loadDotEnvCandidates(dotEnvCandidatesForConfig(configFile))
}

func dotEnvCandidates() []string {
	candidates := []string{
		".env",
		filepath.Join(".", ".env"),
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, ".env"),
			filepath.Join(exeDir, "..", ".env"),
		)
	}

	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func dotEnvCandidatesForConfig(configFile string) []string {
	baseCandidates := dotEnvCandidates()
	if strings.TrimSpace(configFile) == "" {
		return baseCandidates
	}

	configDir := filepath.Dir(configFile)
	configBase := strings.TrimSuffix(filepath.Base(configFile), filepath.Ext(configFile))
	candidates := []string{
		filepath.Join(configDir, ".env."+configBase),
		filepath.Join(configDir, configBase+".env"),
	}
	candidates = append(candidates, baseCandidates...)

	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func loadDotEnvCandidates(candidates []string) {
	for _, candidate := range candidates {
		if loadDotEnvFileIfNeeded(candidate) {
			logger.GetGlobalLogger("core/config").Infof("loaded .env file: %s", candidate)
		}
	}
}

func loadDotEnvFileIfNeeded(path string) bool {
	cleaned := filepath.Clean(path)

	loadDotEnvMu.Lock()
	if _, ok := loadedDotEnvPaths[cleaned]; ok {
		loadDotEnvMu.Unlock()
		return false
	}
	loadDotEnvMu.Unlock()

	if err := loadDotEnvFile(cleaned); err != nil {
		return false
	}

	loadDotEnvMu.Lock()
	loadedDotEnvPaths[cleaned] = struct{}{}
	loadDotEnvMu.Unlock()
	return true
}

func resetDotEnvLoadStateForTests() {
	loadDotEnvMu.Lock()
	defer loadDotEnvMu.Unlock()
	loadedDotEnvPaths = map[string]struct{}{}
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s from %s: %w", key, path, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan .env file %s: %w", path, err)
	}

	return nil
}

func bindKnownEnvs(v *viper.Viper) {
	for key, binding := range knownEnvBindings() {
		args := []string{key, binding.Primary}
		args = append(args, binding.Deprecated...)
		_ = v.BindEnv(args...)
	}
}

func knownEnvBindings() map[string]envBinding {
	return map[string]envBinding{
		"management.baseURL": {
			Primary:    "TASK_PROCESSOR_MANAGEMENT_BASE_URL",
			Deprecated: []string{"MANAGEMENT_BASE_URL"},
		},
		"management.clientID": {
			Primary:    "TASK_PROCESSOR_MANAGEMENT_CLIENT_ID",
			Deprecated: []string{"MANAGEMENT_CLIENT_ID"},
		},
		"management.clientSecret": {
			Primary:    "TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET",
			Deprecated: []string{"MANAGEMENT_CLIENT_SECRET"},
		},
		"management.tokenURL": {
			Primary:    "TASK_PROCESSOR_MANAGEMENT_TOKEN_URL",
			Deprecated: []string{"MANAGEMENT_TOKEN_URL"},
		},
		"management.tenantID": {
			Primary:    "TASK_PROCESSOR_MANAGEMENT_TENANT_ID",
			Deprecated: []string{"MANAGEMENT_TENANT_ID"},
		},
		"management.storeIDs": {
			Primary:    "TASK_PROCESSOR_MANAGEMENT_STORE_IDS",
			Deprecated: []string{"MANAGEMENT_STORE_IDS"},
		},
		"openai.apiKey": {
			Primary:    "TASK_PROCESSOR_OPENAI_API_KEY",
			Deprecated: []string{"OPENAI_API_KEY"},
		},
		"openai.model": {
			Primary:    "TASK_PROCESSOR_OPENAI_MODEL",
			Deprecated: []string{"OPENAI_MODEL"},
		},
		"openai.baseURL": {
			Primary:    "TASK_PROCESSOR_OPENAI_BASE_URL",
			Deprecated: []string{"OPENAI_BASE_URL"},
		},
		"openai.clients.image.apiKey": {
			Primary:    "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY",
			Deprecated: []string{"TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_APIKEY"},
		},
		"openai.clients.image.baseURL": {
			Primary:    "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL",
			Deprecated: []string{"TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASEURL"},
		},
		"openai.clients.image.apiStyle": {
			Primary:    "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE",
			Deprecated: []string{"TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_APISTYLE"},
		},
		"openai.clients.image.model": {
			Primary:    "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL",
			Deprecated: nil,
		},
		"openai.clients.image.timeout": {
			Primary:    "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_TIMEOUT",
			Deprecated: nil,
		},
		"amazon.spapi.clientID": {
			Primary:    "TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_ID",
			Deprecated: []string{"AMAZON_SPAPI_CLIENT_ID"},
		},
		"amazon.spapi.clientSecret": {
			Primary:    "TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_SECRET",
			Deprecated: []string{"AMAZON_SPAPI_CLIENT_SECRET"},
		},
		"amazon.spapi.refreshToken": {
			Primary:    "TASK_PROCESSOR_AMAZON_SPAPI_REFRESH_TOKEN",
			Deprecated: []string{"AMAZON_SPAPI_REFRESH_TOKEN"},
		},
		"amazon.spapi.region": {
			Primary:    "TASK_PROCESSOR_AMAZON_SPAPI_REGION",
			Deprecated: []string{"AMAZON_SPAPI_REGION"},
		},
		"amazon.spapi.defaultMarketplace": {
			Primary:    "TASK_PROCESSOR_AMAZON_SPAPI_DEFAULT_MARKETPLACE",
			Deprecated: []string{"TASK_PROCESSOR_AMAZON_SPAPI_MARKETPLACE_ID", "AMAZON_SPAPI_DEFAULT_MARKETPLACE", "AMAZON_SPAPI_MARKETPLACE_ID"},
		},
		"amazon.spapi.defaultFulfillmentType": {
			Primary:    "TASK_PROCESSOR_AMAZON_SPAPI_DEFAULT_FULFILLMENT_TYPE",
			Deprecated: []string{"AMAZON_SPAPI_DEFAULT_FULFILLMENT_TYPE"},
		},
		"amazon.spapi.defaultCondition": {
			Primary:    "TASK_PROCESSOR_AMAZON_SPAPI_DEFAULT_CONDITION",
			Deprecated: []string{"AMAZON_SPAPI_DEFAULT_CONDITION"},
		},
		"amazon.remoteAPI.enabled": {
			Primary:    "TASK_PROCESSOR_AMAZON_REMOTE_API_ENABLED",
			Deprecated: []string{"AMAZON_REMOTE_API_ENABLED"},
		},
		"amazon.remoteAPI.baseURL": {
			Primary:    "TASK_PROCESSOR_AMAZON_REMOTE_API_BASE_URL",
			Deprecated: []string{"AMAZON_REMOTE_API_BASE_URL"},
		},
		"amazon.remoteAPI.timeout": {
			Primary:    "TASK_PROCESSOR_AMAZON_REMOTE_API_TIMEOUT",
			Deprecated: []string{"AMAZON_REMOTE_API_TIMEOUT"},
		},
		"rabbitmq.enabled": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_ENABLED",
			Deprecated: []string{"RABBITMQ_ENABLED"},
		},
		"rabbitmq.url": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_URL",
			Deprecated: []string{"RABBITMQ_URL"},
		},
		"rabbitmq.node.maxConcurrency": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_MAX_CONCURRENCY",
			Deprecated: []string{"RABBITMQ_NODE_MAX_CONCURRENCY"},
		},
		"rabbitmq.node.role": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_ROLE",
			Deprecated: []string{"RABBITMQ_NODE_ROLE"},
		},
		"rabbitmq.node.nodeID": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_NODE_ID",
			Deprecated: []string{"TASK_PROCESSOR_RABBITMQ_NODEID", "RABBITMQ_NODE_NODE_ID", "RABBITMQ_NODEID"},
		},
		"rabbitmq.node.ownedStores": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_OWNED_STORES",
			Deprecated: []string{"RABBITMQ_NODE_OWNED_STORES"},
		},
		"rabbitmq.node.ownedBuckets": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_OWNED_BUCKETS",
			Deprecated: []string{"RABBITMQ_NODE_OWNED_BUCKETS"},
		},
		"rabbitmq.node.useStoreQueues": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_USE_STORE_QUEUES",
			Deprecated: []string{"RABBITMQ_NODE_USE_STORE_QUEUES"},
		},
		"rabbitmq.autoShard.enabled": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_ENABLED"},
		},
		"rabbitmq.autoShard.role": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_ROLE"},
		},
		"rabbitmq.autoShard.platform": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_PLATFORM",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_PLATFORM"},
		},
		"rabbitmq.autoShard.interval": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_INTERVAL",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_INTERVAL"},
		},
		"rabbitmq.autoShard.pageSize": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_PAGE_SIZE",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_PAGE_SIZE"},
		},
		"rabbitmq.autoShard.lockKey": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_LOCK_KEY",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_LOCK_KEY"},
		},
		"rabbitmq.autoShard.lockTTL": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_LOCK_TTL",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_LOCK_TTL"},
		},
		"rabbitmq.autoShard.candidateNodes": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_CANDIDATE_NODES",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_CANDIDATE_NODES"},
		},
		"rabbitmq.autoShard.targetStoresPerNode": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_TARGET_STORES_PER_NODE",
			Deprecated: []string{"RABBITMQ_AUTO_SHARD_TARGET_STORES_PER_NODE"},
		},
		"rabbitmq.deadLetter.enabled": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_DEAD_LETTER_ENABLED",
			Deprecated: []string{"RABBITMQ_DEAD_LETTER_ENABLED"},
		},
		"rabbitmq.deadLetter.queueName": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_DEAD_LETTER_QUEUE_NAME",
			Deprecated: []string{"RABBITMQ_DEAD_LETTER_QUEUE_NAME"},
		},
		"rabbitmq.processingTimeoutWatchdog.enabled": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_ENABLED",
			Deprecated: []string{"RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_ENABLED"},
		},
		"rabbitmq.processingTimeoutWatchdog.intervalSeconds": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_INTERVAL_SECONDS",
			Deprecated: []string{"RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_INTERVAL_SECONDS"},
		},
		"rabbitmq.processingTimeoutWatchdog.timeoutMinutes": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_TIMEOUT_MINUTES",
			Deprecated: []string{"RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_TIMEOUT_MINUTES"},
		},
		"rabbitmq.processingTimeoutWatchdog.recoveryLimit": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_RECOVERY_LIMIT",
			Deprecated: []string{"RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_RECOVERY_LIMIT"},
		},
		"listingControlPlane.enabled": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_ENABLED",
		},
		"listingControlPlane.platform": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_PLATFORM",
		},
		"listingControlPlane.scanInterval": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_SCAN_INTERVAL",
		},
		"listingControlPlane.batchSize": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_BATCH_SIZE",
		},
		"listingControlPlane.perStoreBurst": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_PER_STORE_BURST",
		},
		"listingControlPlane.maxQueuedPerStore": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_MAX_QUEUED_PER_STORE",
		},
		"listingControlPlane.dryRun": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_DRY_RUN",
		},
		"listingControlPlane.enableLegacyQuotaKeys": {
			Primary: "TASK_PROCESSOR_LISTING_CONTROL_PLANE_ENABLE_LEGACY_QUOTA_KEYS",
		},
		"rabbitmq.staleQueuedWatchdog.enabled": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_ENABLED",
			Deprecated: []string{"RABBITMQ_STALE_QUEUED_WATCHDOG_ENABLED"},
		},
		"rabbitmq.staleQueuedWatchdog.intervalSeconds": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_INTERVAL_SECONDS",
			Deprecated: []string{"RABBITMQ_STALE_QUEUED_WATCHDOG_INTERVAL_SECONDS"},
		},
		"rabbitmq.staleQueuedWatchdog.timeoutMinutes": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_TIMEOUT_MINUTES",
			Deprecated: []string{"RABBITMQ_STALE_QUEUED_WATCHDOG_TIMEOUT_MINUTES"},
		},
		"rabbitmq.staleQueuedWatchdog.recoveryLimit": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_RECOVERY_LIMIT",
			Deprecated: []string{"RABBITMQ_STALE_QUEUED_WATCHDOG_RECOVERY_LIMIT"},
		},
		"rabbitmq.node.healthCheckPort": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_HEALTH_CHECK_PORT",
			Deprecated: []string{"RABBITMQ_NODE_HEALTH_CHECK_PORT", "HEALTH_CHECK_PORT"},
		},
		"rabbitmq.node.metricsPort": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_METRICS_PORT",
			Deprecated: []string{"RABBITMQ_NODE_METRICS_PORT", "METRICS_PORT"},
		},
		"browser.browserPath": {
			Primary:    "TASK_PROCESSOR_BROWSER_PATH",
			Deprecated: []string{"BROWSER_PATH"},
		},
		"browser.userDataDir": {
			Primary:    "TASK_PROCESSOR_BROWSER_USER_DATA_DIR",
			Deprecated: []string{"BROWSER_USER_DATA_DIR"},
		},
		"browser.headless": {
			Primary:    "TASK_PROCESSOR_BROWSER_HEADLESS",
			Deprecated: []string{"BROWSER_HEADLESS"},
		},
		"browser.poolSize": {
			Primary:    "TASK_PROCESSOR_BROWSER_POOL_SIZE",
			Deprecated: []string{"BROWSER_POOL_SIZE"},
		},
		"worker.concurrency": {
			Primary:    "TASK_PROCESSOR_WORKER_CONCURRENCY",
			Deprecated: []string{"WORKER_CONCURRENCY"},
		},
		"worker.bufferSize": {
			Primary:    "TASK_PROCESSOR_WORKER_BUFFER_SIZE",
			Deprecated: []string{"WORKER_BUFFER_SIZE"},
		},
		"worker.taskInterval": {
			Primary:    "TASK_PROCESSOR_WORKER_TASK_INTERVAL",
			Deprecated: []string{"WORKER_TASK_INTERVAL"},
		},
		"platforms.temu.enabled": {
			Primary:    "TASK_PROCESSOR_PLATFORM_TEMU_ENABLED",
			Deprecated: []string{"PLATFORM_TEMU_ENABLED"},
		},
		"platforms.shein.enabled": {
			Primary:    "TASK_PROCESSOR_PLATFORM_SHEIN_ENABLED",
			Deprecated: []string{"PLATFORM_SHEIN_ENABLED"},
		},
		"platforms.alibaba1688.enabled": {
			Primary:    "TASK_PROCESSOR_PLATFORM_1688_ENABLED",
			Deprecated: []string{"PLATFORM_1688_ENABLED"},
		},
		"database.host": {
			Primary:    "TASK_PROCESSOR_DATABASE_HOST",
			Deprecated: []string{"DB_HOST"},
		},
		"database.port": {
			Primary:    "TASK_PROCESSOR_DATABASE_PORT",
			Deprecated: []string{"DB_PORT"},
		},
		"database.user": {
			Primary:    "TASK_PROCESSOR_DATABASE_USER",
			Deprecated: []string{"DB_USER"},
		},
		"database.password": {
			Primary:    "TASK_PROCESSOR_DATABASE_PASSWORD",
			Deprecated: []string{"DB_PASSWORD"},
		},
		"database.database": {
			Primary:    "TASK_PROCESSOR_DATABASE_NAME",
			Deprecated: []string{"DB_NAME"},
		},
		"database.max_connections": {
			Primary:    "TASK_PROCESSOR_DATABASE_MAX_CONNECTIONS",
			Deprecated: []string{"DB_MAX_CONNECTIONS"},
		},
		"database.max_idle_connections": {
			Primary:    "TASK_PROCESSOR_DATABASE_MAX_IDLE_CONNECTIONS",
			Deprecated: []string{"DB_MAX_IDLE_CONNECTIONS"},
		},
		"redis.host": {
			Primary:    "TASK_PROCESSOR_REDIS_HOST",
			Deprecated: []string{"REDIS_HOST"},
		},
		"redis.port": {
			Primary:    "TASK_PROCESSOR_REDIS_PORT",
			Deprecated: []string{"REDIS_PORT"},
		},
		"redis.password": {
			Primary:    "TASK_PROCESSOR_REDIS_PASSWORD",
			Deprecated: []string{"REDIS_PASSWORD"},
		},
		"redis.db": {
			Primary:    "TASK_PROCESSOR_REDIS_DB",
			Deprecated: []string{"REDIS_DB"},
		},
		"redis.pool_size": {
			Primary:    "TASK_PROCESSOR_REDIS_POOL_SIZE",
			Deprecated: []string{"REDIS_POOL_SIZE"},
		},
		"platforms.shein.cookieRedis.host": {
			Primary: "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST",
		},
		"platforms.shein.cookieRedis.port": {
			Primary: "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT",
		},
		"platforms.shein.cookieRedis.password": {
			Primary: "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PASSWORD",
		},
		"platforms.shein.cookieRedis.db": {
			Primary: "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_DB",
		},
		"platforms.shein.cookieRedis.pool_size": {
			Primary: "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_POOL_SIZE",
		},
		"platforms.shein.loginService.tenantID": {
			Primary: "TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_TENANT_ID",
		},
		"platforms.shein.loginService.identifier": {
			Primary: "TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_IDENTIFIER",
		},
		"platforms.sds.loginService.baseURL": {
			Primary: "TASK_PROCESSOR_SDS_LOGIN_SERVICE_BASE_URL",
		},
		"platforms.sds.loginService.sharedKey": {
			Primary: "TASK_PROCESSOR_SDS_LOGIN_SERVICE_SHARED_KEY",
		},
		"platforms.sds.loginService.tenantID": {
			Primary: "TASK_PROCESSOR_SDS_LOGIN_SERVICE_TENANT_ID",
		},
		"platforms.sds.loginService.identifier": {
			Primary: "TASK_PROCESSOR_SDS_LOGIN_SERVICE_IDENTIFIER",
		},
		"platforms.sds.loginService.merchantName": {
			Primary: "TASK_PROCESSOR_SDS_MERCHANT_NAME",
		},
		"platforms.sds.loginService.username": {
			Primary: "TASK_PROCESSOR_SDS_USERNAME",
		},
		"platforms.sds.loginService.password": {
			Primary: "TASK_PROCESSOR_SDS_PASSWORD",
		},
		"platforms.sds.loginService.defaultHeadless": {
			Primary: "TASK_PROCESSOR_SDS_LOGIN_SERVICE_DEFAULT_HEADLESS",
		},
		"platforms.sds.loginService.cloakBrowserEnabled": {
			Primary: "TASK_PROCESSOR_SDS_LOGIN_SERVICE_CLOAKBROWSER_ENABLED",
		},
		"platforms.sds.loginService.cloakBrowserPath": {
			Primary: "TASK_PROCESSOR_SDS_LOGIN_SERVICE_CLOAKBROWSER_PATH",
		},
		"platforms.sds.authRedis.host": {
			Primary: "TASK_PROCESSOR_SDS_AUTH_REDIS_HOST",
		},
		"platforms.sds.authRedis.port": {
			Primary: "TASK_PROCESSOR_SDS_AUTH_REDIS_PORT",
		},
		"platforms.sds.authRedis.password": {
			Primary: "TASK_PROCESSOR_SDS_AUTH_REDIS_PASSWORD",
		},
		"platforms.sds.authRedis.db": {
			Primary: "TASK_PROCESSOR_SDS_AUTH_REDIS_DB",
		},
		"platforms.sds.authRedis.pool_size": {
			Primary: "TASK_PROCESSOR_SDS_AUTH_REDIS_POOL_SIZE",
		},
		"platforms.sds.authBootstrap.staticAccessToken": {
			Primary: "TASK_PROCESSOR_SDS_ACCESS_TOKEN",
		},
		"platforms.sds.authBootstrap.staticOutToken": {
			Primary: "TASK_PROCESSOR_SDS_OUT_ACCESS_TOKEN",
		},
		"platforms.sds.authBootstrap.staticMerchantID": {
			Primary: "TASK_PROCESSOR_SDS_MERCHANT_ID",
		},
		"platforms.sds.authBootstrap.staticCookie": {
			Primary: "TASK_PROCESSOR_SDS_COOKIE",
		},
		"platforms.sds.authBootstrap.loginDomainName": {
			Primary: "TASK_PROCESSOR_SDS_DOMAIN_NAME",
		},
		"platforms.sds.authBootstrap.loginVerifyCaptchaParam": {
			Primary: "TASK_PROCESSOR_SDS_VERIFY_CAPTCHA_PARAM",
		},
		"platforms.sds.authBootstrap.loginExtraInfo": {
			Primary: "TASK_PROCESSOR_SDS_EXTRA_INFO",
		},
		"platforms.sds.authBootstrap.managementStoreID": {
			Primary: "TASK_PROCESSOR_SDS_MANAGEMENT_STORE_ID",
		},
		"debug.productEnrichMockLLM": {
			Primary: "TASK_PROCESSOR_PRODUCTENRICH_MOCK_LLM",
		},
		"listingkit.sheinSubmitDebugDumpDir": {
			Primary: "LISTINGKIT_DEBUG_SUBMIT_DUMP_DIR",
		},
		"listingkit.platformAdminUsers": {
			Primary: "LISTINGKIT_PLATFORM_ADMIN_USERS",
		},
		"listingkit.platformAdminRoles": {
			Primary: "LISTINGKIT_PLATFORM_ADMIN_ROLES",
		},
		"listingkit.zitadel.issuerURL": {
			Primary: "ZITADEL_ISSUER_URL",
		},
		"listingkit.zitadel.clientID": {
			Primary: "ZITADEL_CLIENT_ID",
		},
		"listingkit.zitadel.clientSecret": {
			Primary: "ZITADEL_CLIENT_SECRET",
		},
		"listingkit.zitadel.allowedTenantIDs": {
			Primary:    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS",
			Deprecated: []string{"LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS"},
		},
		"listingkit.zitadel.allowedUserIDs": {
			Primary:    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS",
			Deprecated: []string{"LISTINGKIT_ZITADEL_ALLOWED_USER_IDS"},
		},
		"listingkit.zitadel.allowedUsernames": {
			Primary:    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
			Deprecated: []string{"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES"},
		},
		"listingkit.zitadel.allowedRoles": {
			Primary:    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES",
			Deprecated: []string{"LISTINGKIT_ZITADEL_ALLOWED_ROLES"},
		},
	}
}

func deprecatedEnvWarnings() []string {
	var warnings []string

	for key, binding := range knownEnvBindings() {
		_, primarySet := os.LookupEnv(binding.Primary)
		for _, deprecated := range binding.Deprecated {
			_, deprecatedSet := os.LookupEnv(deprecated)
			if !deprecatedSet {
				continue
			}
			// 已设置新名称时不再提示旧别名（避免系统环境仍带 REDIS_* 等与项目 TASK_PROCESSOR_* 并存时刷屏）
			if primarySet {
				continue
			}

			warning := fmt.Sprintf("environment variable %s is deprecated; use %s for %s instead", deprecated, binding.Primary, key)
			warnings = append(warnings, warning)
		}
	}

	return warnings
}

func logDeprecatedEnvUsage() {
	logger := logger.GetGlobalLogger("core/config")
	for _, warning := range deprecatedEnvWarnings() {
		logger.Warn(warning)
	}
}

func loadWithViper(v *viper.Viper) (*Config, error) {
	cfg := BuildConfig(v)
	if err := cfg.ValidateWithError(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadWithViperWithoutValidation(v *viper.Viper) *Config {
	return BuildConfig(v)
}

func LoadConfig() (*Config, error) {
	tryLoadDotEnv()

	env := os.Getenv("TASK_PROCESSOR_ENV")
	if env == "" {
		env = "dev"
	}

	configName := fmt.Sprintf("config-%s", env)
	logger.GetGlobalLogger("core/config").Infof("loading config file: %s.yaml", configName)
	logDeprecatedEnvUsage()

	v := newViper()
	v.SetConfigName(configName)

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		v.AddConfigPath(filepath.Join(exeDir, "config"))
		v.AddConfigPath(exeDir)
	}

	v.AddConfigPath("./config")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/task-processor/")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s.yaml: %w", configName, err)
	}

	logger.GetGlobalLogger("core/config").Infof("loaded config file: %s", v.ConfigFileUsed())
	return loadWithViper(v)
}

func LoadConfigFromFile(configFile string) (*Config, error) {
	tryLoadDotEnvForConfig(configFile)

	logger.GetGlobalLogger("core/config").Infof("loading config file: %s", configFile)
	logDeprecatedEnvUsage()

	v := newViper()
	v.SetConfigFile(configFile)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", configFile, err)
	}

	logger.GetGlobalLogger("core/config").Infof("loaded config file: %s", v.ConfigFileUsed())
	return loadWithViper(v)
}

func LoadConfigFromFileWithoutValidation(configFile string) (*Config, error) {
	tryLoadDotEnvForConfig(configFile)

	logger.GetGlobalLogger("core/config").Infof("loading config file: %s", configFile)
	logDeprecatedEnvUsage()

	v := newViper()
	v.SetConfigFile(configFile)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", configFile, err)
	}

	logger.GetGlobalLogger("core/config").Infof("loaded config file: %s", v.ConfigFileUsed())
	return loadWithViperWithoutValidation(v), nil
}
