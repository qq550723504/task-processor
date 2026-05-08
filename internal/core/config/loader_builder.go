package config

import (
	"strconv"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/spf13/viper"
)

func BuildConfig(v *viper.Viper) *Config {
	cfg := &Config{
		Processor: ProcessorConfig{
			MaxRetries: v.GetInt("processor.maxRetries"),
			Timeout:    v.GetInt("processor.timeout"),
		},
		Worker: WorkerConfig{
			Concurrency:        v.GetInt("worker.concurrency"),
			BufferSize:         v.GetInt("worker.bufferSize"),
			TaskInterval:       v.GetInt("worker.taskInterval"),
			MaxFetchPerCycle:   v.GetInt("worker.maxFetchPerCycle"),
			QueueThreshold:     v.GetInt("worker.queueThreshold"),
			CleanupInterval:    v.GetInt("worker.cleanupInterval"),
			TaskTimeout:        v.GetInt("worker.taskTimeout"),
			StuckTaskThreshold: v.GetInt("worker.stuckTaskThreshold"),
			ForceCleanupAfter:  v.GetInt("worker.forceCleanupAfter"),
		},
		OpenAI: OpenAIConfig{
			APIKey:  v.GetString("openai.apiKey"),
			Model:   v.GetString("openai.model"),
			BaseURL: v.GetString("openai.baseURL"),
			Timeout: v.GetInt("openai.timeout"),
			Clients: buildOpenAIClients(v),
		},
		Management: ManagementConfig{
			BaseURL:      v.GetString("management.baseURL"),
			ClientID:     v.GetString("management.clientID"),
			ClientSecret: v.GetString("management.clientSecret"),
			TokenURL:     v.GetString("management.tokenURL"),
			Scopes:       v.GetStringSlice("management.scopes"),
			TenantID:     v.GetString("management.tenantID"),
			UserID:       v.GetInt64("management.userID"),
			StoreIDs:     getInt64Slice(v, "management.storeIDs"),
		},
		Platforms: PlatformsConfig{
			Temu:  BuildPlatformConfig(v, "platforms.temu"),
			Shein: BuildPlatformConfig(v, "platforms.shein"),
			SDS: SDSPlatformConfig{
				LoginService: BuildLoginServiceConfig(v, "platforms.sds.loginService"),
			},
			Alibaba1688: Alibaba1688Config{
				Enabled:  v.GetBool("platforms.alibaba1688.enabled"),
				Timeout:  v.GetInt("platforms.alibaba1688.timeout"),
				PoolSize: v.GetInt("platforms.alibaba1688.poolSize"),
			},
		},
		Browser: BrowserConfig{
			Enabled:        v.GetBool("browser.enabled"),
			Headless:       v.GetBool("browser.headless"),
			BrowserPath:    v.GetString("browser.browserPath"),
			PoolSize:       v.GetInt("browser.poolSize"),
			ViewportWidth:  v.GetInt("browser.viewportWidth"),
			ViewportHeight: v.GetInt("browser.viewportHeight"),
			ProxyServer:    v.GetString("browser.proxyServer"),
			RandomConfig: BrowserRandomConfig{
				Enabled:             v.GetBool("browser.randomConfig.enabled"),
				Strategy:            v.GetString("browser.randomConfig.strategy"),
				PresetName:          v.GetString("browser.randomConfig.presetName"),
				FingerprintStrategy: v.GetString("browser.randomConfig.fingerprintStrategy"),
				HealthCheckEnabled:  v.GetBool("browser.randomConfig.healthCheckEnabled"),
				MaxRetries:          v.GetInt("browser.randomConfig.maxRetries"),
				MaxUsesPerInstance:  v.GetInt("browser.randomConfig.maxUsesPerInstance"),
			},
		},
		Amazon: AmazonConfig{
			Enabled:           v.GetBool("amazon.enabled"),
			Zipcodes:          v.GetStringMapString("amazon.zipcodes"),
			DataFreshnessDays: v.GetInt("amazon.dataFreshnessDays"),
			CrawlTimeout:      v.GetInt("amazon.crawlTimeout"),
			ProductDedupe: ProductDedupeConfig{
				LockTTLSeconds:     v.GetInt("amazon.productDedupe.lockTTLSeconds"),
				ResultTTLSeconds:   v.GetInt("amazon.productDedupe.resultTTLSeconds"),
				WaitTimeoutSeconds: v.GetInt("amazon.productDedupe.waitTimeoutSeconds"),
				PollIntervalMillis: v.GetInt("amazon.productDedupe.pollIntervalMillis"),
			},
			FailureArtifacts: FailureArtifactsConfig{
				Enabled:      v.GetBool("amazon.failureArtifacts.enabled"),
				Directory:    v.GetString("amazon.failureArtifacts.directory"),
				CaptureHTML:  v.GetBool("amazon.failureArtifacts.captureHTML"),
				MaxHTMLBytes: v.GetInt("amazon.failureArtifacts.maxHTMLBytes"),
			},
			RiskControl: AmazonRiskControlConfig{
				CaptchaRecreateThreshold:        v.GetInt("amazon.riskControl.captchaRecreateThreshold"),
				AuthenticationRecreateThreshold: v.GetInt("amazon.riskControl.authenticationRecreateThreshold"),
				BrowserCrashRecreateThreshold:   v.GetInt("amazon.riskControl.browserCrashRecreateThreshold"),
				TimeoutRecreateThreshold:        v.GetInt("amazon.riskControl.timeoutRecreateThreshold"),
				NetworkRecreateThreshold:        v.GetInt("amazon.riskControl.networkRecreateThreshold"),
				ServerErrorRecreateThreshold:    v.GetInt("amazon.riskControl.serverErrorRecreateThreshold"),
			},
			RegionGuard: AmazonRegionGuardConfig{
				Enabled:                 v.GetBool("amazon.regionGuard.enabled"),
				FailureThreshold:        v.GetInt("amazon.regionGuard.failureThreshold"),
				EvaluationWindowSeconds: v.GetInt("amazon.regionGuard.evaluationWindowSeconds"),
				CooldownSeconds:         v.GetInt("amazon.regionGuard.cooldownSeconds"),
			},
			QualityControl: AmazonQualityControlConfig{
				RetryOnValidationFailure:   v.GetBool("amazon.qualityControl.retryOnValidationFailure"),
				ValidationRetryMaxAttempts: v.GetInt("amazon.qualityControl.validationRetryMaxAttempts"),
			},
			ProxyPool: AmazonProxyPoolConfig{
				Enabled:                v.GetBool("amazon.proxyPool.enabled"),
				Strategy:               v.GetString("amazon.proxyPool.strategy"),
				FailureCooldownSeconds: v.GetInt("amazon.proxyPool.failureCooldownSeconds"),
				Proxies:                v.GetStringSlice("amazon.proxyPool.proxies"),
			},
			ConcurrencyControl: AmazonConcurrencyControlConfig{
				Enabled:               v.GetBool("amazon.concurrencyControl.enabled"),
				MaxInFlight:           v.GetInt("amazon.concurrencyControl.maxInFlight"),
				MaxWaiting:            v.GetInt("amazon.concurrencyControl.maxWaiting"),
				AcquireTimeoutSeconds: v.GetInt("amazon.concurrencyControl.acquireTimeoutSeconds"),
				PerRegion:             getStringIntMap(v, "amazon.concurrencyControl.perRegion"),
			},
			RemoteAPI: RemoteAPIConfig{
				Enabled: v.GetBool("amazon.remoteAPI.enabled"),
				BaseURL: v.GetString("amazon.remoteAPI.baseURL"),
				Timeout: v.GetInt("amazon.remoteAPI.timeout"),
			},
		},
		ProductImage: ProductImageConfig{
			WorkDir: v.GetString("productimage.workDir"),
			Segmenter: ProductImageModelConfig{
				Enabled:  v.GetBool("productimage.segmenter.enabled"),
				Endpoint: v.GetString("productimage.segmenter.endpoint"),
				APIKey:   v.GetString("productimage.segmenter.apiKey"),
				Timeout:  v.GetInt("productimage.segmenter.timeout"),
			},
			WhiteBackground: ProductImageModelConfig{
				Enabled:  v.GetBool("productimage.whiteBackground.enabled"),
				Endpoint: v.GetString("productimage.whiteBackground.endpoint"),
				APIKey:   v.GetString("productimage.whiteBackground.apiKey"),
				Timeout:  v.GetInt("productimage.whiteBackground.timeout"),
			},
			Scene: ProductImageModelConfig{
				Enabled:  v.GetBool("productimage.scene.enabled"),
				Endpoint: v.GetString("productimage.scene.endpoint"),
				APIKey:   v.GetString("productimage.scene.apiKey"),
				Timeout:  v.GetInt("productimage.scene.timeout"),
			},
			Publisher: ProductImagePublisherConfig{
				Enabled:    v.GetBool("productimage.publisher.enabled"),
				Provider:   v.GetString("productimage.publisher.provider"),
				OutputDir:  v.GetString("productimage.publisher.outputDir"),
				PublicBase: v.GetString("productimage.publisher.publicBase"),
				S3: ProductImagePublisherS3Config{
					Bucket:          v.GetString("productimage.publisher.s3.bucket"),
					Region:          v.GetString("productimage.publisher.s3.region"),
					Endpoint:        v.GetString("productimage.publisher.s3.endpoint"),
					AccessKeyID:     v.GetString("productimage.publisher.s3.accessKeyID"),
					SecretAccessKey: v.GetString("productimage.publisher.s3.secretAccessKey"),
					UsePathStyle:    v.GetBool("productimage.publisher.s3.usePathStyle"),
				},
			},
			Lifecycle: ProductImageLifecycleConfig{
				CleanupTemporaryFiles: v.GetBool("productimage.lifecycle.cleanupTemporaryFiles"),
				ReuseExistingAssets:   v.GetBool("productimage.lifecycle.reuseExistingAssets"),
			},
		},
		Updater: UpdaterConfig{
			Enabled:            v.GetBool("updater.enabled"),
			UpdateURL:          v.GetString("updater.updateURL"),
			CheckInterval:      v.GetInt("updater.checkInterval"),
			InsecureSkipVerify: v.GetBool("updater.insecureSkipVerify"),
		},
	}

	if v.GetBool("rabbitmq.enabled") {
		cfg.RabbitMQ = BuildRabbitMQConfig(v)
	}

	if v.GetString("database.host") != "" {
		cfg.Database = &DatabaseConfig{
			Host:                  v.GetString("database.host"),
			Port:                  v.GetInt("database.port"),
			User:                  v.GetString("database.user"),
			Password:              v.GetString("database.password"),
			Database:              v.GetString("database.database"),
			MaxConnections:        v.GetInt("database.max_connections"),
			MaxIdleConnections:    v.GetInt("database.max_idle_connections"),
			ConnectionMaxLifetime: time.Duration(v.GetInt64("database.connection_max_lifetime")),
		}
	}

	if v.GetString("redis.host") != "" {
		port := v.GetInt("redis.port")
		if port == 0 {
			port = 6379
		}
		poolSize := v.GetInt("redis.pool_size")
		if poolSize == 0 {
			poolSize = 10
		}
		cfg.Redis = &RedisConfig{
			Host:     v.GetString("redis.host"),
			Port:     port,
			Password: v.GetString("redis.password"),
			DB:       v.GetInt("redis.db"),
			PoolSize: poolSize,
		}
	}

	cfg.Logging = LoggingConfig{
		Level:        v.GetString("logging.level"),
		Format:       v.GetString("logging.format"),
		File:         v.GetString("logging.file"),
		SplitByLevel: buildSplitByLevelConfig(v),
	}

	return cfg
}

func buildOpenAIClients(v *viper.Viper) map[string]OpenAIClientConfig {
	raw := v.GetStringMap("openai.clients")
	if len(raw) == 0 {
		return nil
	}

	defaultKey := v.GetString("openai.apiKey")
	defaultBase := v.GetString("openai.baseURL")
	defaultTimeout := v.GetInt("openai.timeout")

	clients := make(map[string]OpenAIClientConfig, len(raw))
	for name := range raw {
		prefix := "openai.clients." + name

		apiKey := v.GetString(prefix + ".apiKey")
		if apiKey == "" {
			apiKey = defaultKey
		}
		baseURL := v.GetString(prefix + ".baseURL")
		if baseURL == "" {
			baseURL = defaultBase
		}
		timeout := v.GetInt(prefix + ".timeout")
		if timeout == 0 {
			timeout = defaultTimeout
		}

		clients[name] = OpenAIClientConfig{
			APIKey:   apiKey,
			Model:    v.GetString(prefix + ".model"),
			BaseURL:  baseURL,
			Timeout:  timeout,
			APIStyle: v.GetString(prefix + ".apiStyle"),
		}
	}
	return clients
}

func getStringIntMap(v *viper.Viper, key string) map[string]int {
	raw := v.GetStringMap(key)
	if len(raw) == 0 {
		return nil
	}

	result := make(map[string]int, len(raw))
	for mapKey, value := range raw {
		mapKey = strings.TrimSpace(mapKey)
		if mapKey == "" {
			continue
		}
		switch typed := value.(type) {
		case int:
			result[mapKey] = typed
		case int64:
			result[mapKey] = int(typed)
		case float64:
			result[mapKey] = int(typed)
		case float32:
			result[mapKey] = int(typed)
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(typed))
			if err == nil {
				result[mapKey] = parsed
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func buildSplitByLevelConfig(v *viper.Viper) []logger.LevelFileConfig {
	raw := v.Get("logging.split_by_level")
	if raw == nil {
		return nil
	}

	list, ok := raw.([]any)
	if !ok {
		return nil
	}

	configs := make([]logger.LevelFileConfig, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		cfg := logger.LevelFileConfig{
			File: getStringFromMap(m, "file"),
		}

		if levelsRaw, ok := m["levels"]; ok {
			switch values := levelsRaw.(type) {
			case []any:
				for _, level := range values {
					if s, ok := level.(string); ok {
						cfg.Levels = append(cfg.Levels, s)
					}
				}
			case []string:
				cfg.Levels = values
			}
		}

		if cfg.File != "" && len(cfg.Levels) > 0 {
			configs = append(configs, cfg)
		}
	}

	return configs
}
