package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateOpenAIConfig_ClientsMustBeComplete(t *testing.T) {
	cfg := OpenAIConfig{
		APIKey:  "",
		Model:   "gemini-2.5-flash",
		BaseURL: "https://example.com/v1",
		Timeout: 30,
		Clients: map[string]OpenAIClientConfig{
			"vision": {
				Model: "gpt-4o-mini",
			},
		},
	}

	errors := ValidateOpenAIConfig(&cfg)
	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[len(errors)-1].Error(), "openai.clients.vision.apiKey")
}

func TestValidateAmazonConfig_SPAPIRequiresCredentials(t *testing.T) {
	cfg := AmazonConfig{
		Enabled:           true,
		DataFreshnessDays: 7,
		CrawlTimeout:      30,
		RiskControl: AmazonRiskControlConfig{
			CaptchaRecreateThreshold:        1,
			AuthenticationRecreateThreshold: 1,
			BrowserCrashRecreateThreshold:   1,
			TimeoutRecreateThreshold:        3,
			NetworkRecreateThreshold:        2,
			ServerErrorRecreateThreshold:    3,
		},
		RegionGuard: AmazonRegionGuardConfig{
			Enabled:                 true,
			FailureThreshold:        3,
			EvaluationWindowSeconds: 300,
			CooldownSeconds:         180,
		},
		QualityControl: AmazonQualityControlConfig{
			RetryOnValidationFailure:   true,
			ValidationRetryMaxAttempts: 2,
		},
		SPAPI: SPAPIConfig{
			Enabled: true,
			Region:  "us-east-1",
		},
	}

	errors := ValidateAmazonConfig(&cfg)
	assert.NotEmpty(t, errors)
}

func TestValidatePlatformConfig_SchedulerNeedsTask(t *testing.T) {
	cfg := PlatformConfig{
		SchedulerEnabled: true,
	}

	errors := ValidatePlatformConfig("temu", &cfg)
	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Error(), "platforms.temu.schedulerEnabled")
}

func TestValidateRabbitMQConfig_EnabledRequiresCoreFields(t *testing.T) {
	cfg := RabbitMQConfig{
		Enabled: true,
		Node: NodeConfig{
			MaxConcurrency: 0,
		},
	}

	errors := ValidateRabbitMQConfig(&cfg)
	assert.NotEmpty(t, errors)
}

func TestValidateManagementConfigDoesNotRequireRetiredServiceCredentials(t *testing.T) {
	errors := ValidateManagementConfig(&ManagementConfig{})
	assert.Empty(t, errors)
}

func TestValidateConfig_CatchesDependencyErrors(t *testing.T) {
	cfg := &Config{
		Worker: WorkerConfig{
			Concurrency:      1,
			BufferSize:       1,
			TaskInterval:     1,
			MaxFetchPerCycle: 1,
		},
		Management: ManagementConfig{
			BaseURL:      "http://example.com",
			ClientID:     "client",
			ClientSecret: "secret",
			TokenURL:     "http://example.com/token",
			Scopes:       []string{"user.read"},
			TenantID:     "1",
		},
		OpenAI: OpenAIConfig{
			APIKey:  "key",
			Model:   "model",
			BaseURL: "http://example.com/v1",
			Timeout: 10,
			Clients: map[string]OpenAIClientConfig{
				"vision": {},
			},
		},
		Browser: BrowserConfig{
			Enabled:        true,
			PoolSize:       1,
			ViewportWidth:  1,
			ViewportHeight: 1,
		},
		Amazon: AmazonConfig{
			Enabled:           true,
			DataFreshnessDays: 1,
			CrawlTimeout:      1,
			RiskControl: AmazonRiskControlConfig{
				CaptchaRecreateThreshold:        1,
				AuthenticationRecreateThreshold: 1,
				BrowserCrashRecreateThreshold:   1,
				TimeoutRecreateThreshold:        3,
				NetworkRecreateThreshold:        2,
				ServerErrorRecreateThreshold:    3,
			},
			RegionGuard: AmazonRegionGuardConfig{
				Enabled:                 true,
				FailureThreshold:        3,
				EvaluationWindowSeconds: 300,
				CooldownSeconds:         180,
			},
			QualityControl: AmazonQualityControlConfig{
				RetryOnValidationFailure:   true,
				ValidationRetryMaxAttempts: 2,
			},
			SPAPI: SPAPIConfig{
				Enabled: true,
				Region:  "us-east-1",
			},
		},
		RabbitMQ: &RabbitMQConfig{
			Enabled: true,
			URL:     "",
			Consumer: RabbitMQConsumerConfig{
				PrefetchCount: 1,
			},
			Node: NodeConfig{
				MaxConcurrency:  1,
				HealthCheckPort: 1,
				MetricsPort:     1,
			},
		},
		Platforms: PlatformsConfig{
			Temu: PlatformConfig{
				SchedulerEnabled: true,
			},
		},
	}

	errors := cfg.Validate()
	assert.NotEmpty(t, errors)
}
