package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/watermark"
	"time"

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
	Logging      LoggingConfig      `yaml:"logging"`
	Processor    ProcessorConfig    `yaml:"processor"`
	Worker       WorkerConfig       `yaml:"worker"`
	OpenAI       OpenAIConfig       `yaml:"openai"`
	Management   ManagementConfig   `yaml:"management"`
	Browser      BrowserConfig      `yaml:"browser"`
	Amazon       AmazonConfig       `yaml:"amazon"`
	RabbitMQ     *RabbitMQConfig    `yaml:"rabbitmq"`
	Updater      UpdaterConfig      `yaml:"updater"`
	Platforms    PlatformsConfig    `yaml:"platforms"`
	Watermark    *watermark.Config  `yaml:"watermark"`
	ProductImage ProductImageConfig `yaml:"productimage"`
	Database     *DatabaseConfig    `yaml:"database"`
	Redis        *RedisConfig       `yaml:"redis"`
	Prompts      PromptsConfig      `yaml:"prompts"`
	Debug        DebugConfig        `yaml:"debug"`
}

type DebugConfig struct {
	SavePublishJSON bool `yaml:"save_publish_json"`
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

func lookupKnownEnvValue(key string) (string, bool) {
	binding, ok := knownEnvBindings()[key]
	if !ok {
		return "", false
	}

	candidates := append([]string{binding.Primary}, binding.Deprecated...)
	for _, envKey := range candidates {
		if value, exists := os.LookupEnv(envKey); exists && strings.TrimSpace(value) != "" {
			return value, true
		}
	}

	return "", false
}

func lookupKnownEnvInt(key string) (int, bool) {
	binding, ok := knownEnvBindings()[key]
	if !ok {
		return 0, false
	}

	candidates := append([]string{binding.Primary}, binding.Deprecated...)
	for _, envKey := range candidates {
		if value, exists := os.LookupEnv(envKey); exists && strings.TrimSpace(value) != "" {
			i, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return 0, false
			}
			return i, true
		}
	}

	return 0, false
}

func lookupKnownEnvInt64Slice(key string) ([]int64, bool) {
	binding, ok := knownEnvBindings()[key]
	if !ok {
		return nil, false
	}

	candidates := append([]string{binding.Primary}, binding.Deprecated...)
	for _, envKey := range candidates {
		value, exists := os.LookupEnv(envKey)
		if !exists || strings.TrimSpace(value) == "" {
			continue
		}

		parts := strings.Split(value, ",")
		result := make([]int64, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			parsed, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, false
			}
			result = append(result, parsed)
		}

		return result, true
	}

	return nil, false
}

func lookupKnownEnvIntSlice(key string) ([]int, bool) {
	binding, ok := knownEnvBindings()[key]
	if !ok {
		return nil, false
	}

	candidates := append([]string{binding.Primary}, binding.Deprecated...)
	for _, envKey := range candidates {
		value, exists := os.LookupEnv(envKey)
		if !exists || strings.TrimSpace(value) == "" {
			continue
		}

		parts := strings.Split(value, ",")
		result := make([]int, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			parsed, err := strconv.Atoi(part)
			if err != nil {
				return nil, false
			}
			result = append(result, parsed)
		}

		return result, true
	}

	return nil, false
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	tryLoadDotEnv()

	if value, ok := lookupKnownEnvValue("management.baseURL"); ok {
		cfg.Management.BaseURL = value
	}
	if value, ok := lookupKnownEnvValue("management.clientID"); ok {
		cfg.Management.ClientID = value
	}
	if value, ok := lookupKnownEnvValue("management.clientSecret"); ok {
		cfg.Management.ClientSecret = value
	}
	if value, ok := lookupKnownEnvValue("management.tokenURL"); ok {
		cfg.Management.TokenURL = value
	}
	if value, ok := lookupKnownEnvValue("management.tenantID"); ok {
		cfg.Management.TenantID = value
	}
	if value, ok := lookupKnownEnvInt64Slice("management.storeIDs"); ok {
		cfg.Management.StoreIDs = value
	}

	if value, ok := lookupKnownEnvValue("openai.apiKey"); ok {
		cfg.OpenAI.APIKey = value
	}
	if value, ok := lookupKnownEnvValue("openai.model"); ok {
		cfg.OpenAI.Model = value
	}
	if value, ok := lookupKnownEnvValue("openai.baseURL"); ok {
		cfg.OpenAI.BaseURL = value
	}

	if value, ok := lookupKnownEnvValue("amazon.spapi.clientID"); ok {
		cfg.Amazon.SPAPI.ClientID = value
	}
	if value, ok := lookupKnownEnvValue("amazon.spapi.clientSecret"); ok {
		cfg.Amazon.SPAPI.ClientSecret = value
	}
	if value, ok := lookupKnownEnvValue("amazon.spapi.refreshToken"); ok {
		cfg.Amazon.SPAPI.RefreshToken = value
	}
	if value, ok := lookupKnownEnvValue("amazon.spapi.region"); ok {
		cfg.Amazon.SPAPI.Region = value
	}
	if value, ok := lookupKnownEnvValue("amazon.spapi.defaultMarketplace"); ok {
		cfg.Amazon.SPAPI.DefaultMarketplace = value
	}
	if value, ok := lookupKnownEnvValue("amazon.spapi.defaultFulfillmentType"); ok {
		cfg.Amazon.SPAPI.DefaultFulfillmentType = value
	}
	if value, ok := lookupKnownEnvValue("amazon.spapi.defaultCondition"); ok {
		cfg.Amazon.SPAPI.DefaultCondition = value
	}
	if value, ok := lookupKnownEnvValue("amazon.remoteAPI.enabled"); ok {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			cfg.Amazon.RemoteAPI.Enabled = parsed
		}
	}
	if value, ok := lookupKnownEnvValue("amazon.remoteAPI.baseURL"); ok {
		cfg.Amazon.RemoteAPI.BaseURL = value
	}
	if value, ok := lookupKnownEnvInt("amazon.remoteAPI.timeout"); ok {
		cfg.Amazon.RemoteAPI.Timeout = value
	}

	if value, ok := lookupKnownEnvValue("rabbitmq.enabled"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			cfg.RabbitMQ.Enabled = parsed
		}
	}
	if value, ok := lookupKnownEnvValue("rabbitmq.url"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.URL = value
	}
	if value, ok := lookupKnownEnvInt("rabbitmq.node.maxConcurrency"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.Node.MaxConcurrency = value
	}
	if value, ok := lookupKnownEnvValue("rabbitmq.node.nodeID"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.Node.NodeID = value
	}
	if value, ok := lookupKnownEnvInt64Slice("rabbitmq.node.ownedStores"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.Node.OwnedStores = value
	}
	if value, ok := lookupKnownEnvIntSlice("rabbitmq.node.ownedBuckets"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.Node.OwnedBuckets = value
	}
	if value, ok := lookupKnownEnvValue("rabbitmq.node.useStoreQueues"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			cfg.RabbitMQ.Node.UseStoreQueues = parsed
		}
	}
	if value, ok := lookupKnownEnvValue("rabbitmq.autoShard.enabled"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			cfg.RabbitMQ.AutoShard.Enabled = parsed
		}
	}
	if value, ok := lookupKnownEnvValue("rabbitmq.autoShard.platform"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.AutoShard.Platform = value
	}
	if value, ok := lookupKnownEnvInt("rabbitmq.autoShard.interval"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.AutoShard.Interval = time.Duration(value) * time.Second
	}
	if value, ok := lookupKnownEnvInt("rabbitmq.autoShard.pageSize"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.AutoShard.PageSize = value
	}
	if value, ok := lookupKnownEnvValue("rabbitmq.autoShard.lockKey"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.AutoShard.LockKey = value
	}
	if value, ok := lookupKnownEnvInt("rabbitmq.autoShard.lockTTL"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.AutoShard.LockTTL = time.Duration(value) * time.Second
	}
	if value, ok := lookupKnownEnvValue("rabbitmq.autoShard.candidateNodes"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.AutoShard.CandidateNodes = splitCommaSeparatedStrings(value)
	}
	if value, ok := lookupKnownEnvInt("rabbitmq.node.healthCheckPort"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.Node.HealthCheckPort = value
	}
	if value, ok := lookupKnownEnvInt("rabbitmq.node.metricsPort"); ok {
		if cfg.RabbitMQ == nil {
			cfg.RabbitMQ = &RabbitMQConfig{}
		}
		cfg.RabbitMQ.Node.MetricsPort = value
	}
	if value, ok := lookupKnownEnvValue("browser.browserPath"); ok {
		cfg.Browser.BrowserPath = value
	}

	ensureDatabase := func() {
		if cfg.Database == nil {
			cfg.Database = &DatabaseConfig{}
		}
	}
	if value, ok := lookupKnownEnvValue("database.host"); ok {
		ensureDatabase()
		cfg.Database.Host = value
	}
	if value, ok := lookupKnownEnvInt("database.port"); ok {
		ensureDatabase()
		cfg.Database.Port = value
	}
	if value, ok := lookupKnownEnvValue("database.user"); ok {
		ensureDatabase()
		cfg.Database.User = value
	}
	if value, ok := lookupKnownEnvValue("database.password"); ok {
		ensureDatabase()
		cfg.Database.Password = value
	}
	if value, ok := lookupKnownEnvValue("database.database"); ok {
		ensureDatabase()
		cfg.Database.Database = value
	}
	if value, ok := lookupKnownEnvInt("database.max_connections"); ok {
		ensureDatabase()
		cfg.Database.MaxConnections = value
	}
	if value, ok := lookupKnownEnvInt("database.max_idle_connections"); ok {
		ensureDatabase()
		cfg.Database.MaxIdleConnections = value
	}
}

func splitCommaSeparatedStrings(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
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
