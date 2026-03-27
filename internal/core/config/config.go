package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/watermark"

	"github.com/spf13/viper"
)

type envBinding struct {
	Primary    string
	Deprecated []string
}

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
		"rabbitmq.url": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_URL",
			Deprecated: []string{"RABBITMQ_URL"},
		},
		"rabbitmq.node.maxConcurrency": {
			Primary:    "TASK_PROCESSOR_RABBITMQ_NODE_MAX_CONCURRENCY",
			Deprecated: []string{"RABBITMQ_NODE_MAX_CONCURRENCY"},
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
	}
}

func deprecatedEnvWarnings() []string {
	var warnings []string

	for key, binding := range knownEnvBindings() {
		primaryValue, primarySet := os.LookupEnv(binding.Primary)
		for _, deprecated := range binding.Deprecated {
			deprecatedValue, deprecatedSet := os.LookupEnv(deprecated)
			if !deprecatedSet {
				continue
			}

			warning := fmt.Sprintf("environment variable %s is deprecated; use %s for %s instead", deprecated, binding.Primary, key)
			if primarySet && strings.TrimSpace(primaryValue) != "" && strings.TrimSpace(deprecatedValue) != "" {
				warning = fmt.Sprintf("%s (both are set; %s takes precedence)", warning, binding.Primary)
			}
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

func LoadConfig() (*Config, error) {
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
