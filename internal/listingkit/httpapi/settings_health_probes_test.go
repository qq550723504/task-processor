package httpapi

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSettingsHealthProbesFromConfigReportsReadyConfiguredRuntime(t *testing.T) {
	t.Parallel()

	probes := buildSettingsHealthProbesFromConfig(&config.Config{
		Platforms: config.PlatformsConfig{
			Shein: config.PlatformConfig{
				LoginService: config.LoginServiceConfig{
					BaseURL:    "http://login:8000",
					TenantID:   "tenant-1",
					Identifier: "store-1",
				},
				CookieRedis: config.RedisConfig{Host: "redis", Port: 6379},
			},
			SDS: config.SDSPlatformConfig{
				LoginService: config.SDSLoginServiceConfig{
					BaseURL:    "http://login:8000",
					TenantID:   "tenant-1",
					Identifier: "sds-store-1",
				},
			},
		},
		ProductImage: config.ProductImageConfig{
			Publisher: config.ProductImagePublisherConfig{
				Provider:   "s3",
				PublicBase: "https://cdn.example.test/assets",
				S3: config.ProductImagePublisherS3Config{
					Bucket:          "listingkit",
					Endpoint:        "https://s3.example.test",
					AccessKeyID:     "access",
					SecretAccessKey: "secret",
				},
			},
		},
	})

	if !probes.SheinIntegration.Configured || len(probes.SheinIntegration.Missing) > 0 {
		t.Fatalf("shein integration probe = %+v, want ready", probes.SheinIntegration)
	}
	if !probes.SDSLogin.Configured || len(probes.SDSLogin.Missing) > 0 {
		t.Fatalf("sds login probe = %+v, want ready", probes.SDSLogin)
	}
	if !probes.ObjectStorage.Configured || len(probes.ObjectStorage.Missing) > 0 {
		t.Fatalf("object storage probe = %+v, want ready", probes.ObjectStorage)
	}
}

func TestBuildSettingsHealthProbesFromConfigReportsMissingRuntimeFields(t *testing.T) {
	t.Parallel()

	probes := buildSettingsHealthProbesFromConfig(&config.Config{
		ProductImage: config.ProductImageConfig{
			Publisher: config.ProductImagePublisherConfig{
				Provider: "s3",
			},
		},
	})

	assertProbeMissing(t, probes.SheinIntegration, "shein.loginService.baseURL 缺失")
	assertProbeMissing(t, probes.SheinIntegration, "shein.loginService.identifier 缺失")
	assertProbeMissing(t, probes.SDSLogin, "sds.loginService.baseURL 缺失")
	assertProbeMissing(t, probes.SDSLogin, "sds.loginService.identifier 缺失")
	assertProbeMissing(t, probes.ObjectStorage, "productimage.publisher.s3.bucket 缺失")
	assertProbeMissing(t, probes.ObjectStorage, "productimage.publisher.s3.endpoint 缺失")
}

func TestCompleteSettingsHealthProbesWithSubmitRuntimeReportsMissingSheinCapabilities(t *testing.T) {
	t.Parallel()

	probes := buildSettingsHealthProbesFromConfig(&config.Config{
		Platforms: config.PlatformsConfig{
			Shein: config.PlatformConfig{
				LoginService: config.LoginServiceConfig{
					BaseURL:    "http://login:8000",
					TenantID:   "tenant-1",
					Identifier: "store-1",
				},
				CookieRedis: config.RedisConfig{Host: "redis", Port: 6379},
			},
		},
	})

	completed := completeSettingsHealthProbesWithSubmitRuntime(probes, submitModule{})

	assertProbeMissing(t, completed.SheinIntegration, "shein.productAPIBuilder 未接入")
	assertProbeMissing(t, completed.SheinIntegration, "shein.imageAPIBuilder 未接入")
	assertProbeMissing(t, completed.SheinIntegration, "shein.categoryResolver 未接入")
}

func TestCompleteSettingsHealthProbesWithSubmitRuntimeKeepsReadySheinCapabilities(t *testing.T) {
	t.Parallel()

	probes := completeSettingsHealthProbesWithSubmitRuntime(listingkit.SettingsHealthProbes{
		SheinIntegration: listingkit.SettingsHealthProbe{Configured: true},
	}, submitModule{
		shein: submitSheinDependencies{
			productAPIBuilder: settingsHealthProductAPIBuilderStub{},
			imageAPIBuilder:   settingsHealthImageAPIBuilderStub{},
			categoryResolver:  sheinpub.NewCategoryResolver(nil),
		},
	})

	if !probes.SheinIntegration.Configured || len(probes.SheinIntegration.Missing) > 0 {
		t.Fatalf("shein integration probe = %+v, want ready", probes.SheinIntegration)
	}
}

func TestBuildListingKitServiceConfigIncludesSubmitRuntimeHealthProbes(t *testing.T) {
	t.Parallel()

	cfg := buildListingKitServiceConfig(buildListingKitServiceConfigInput{
		input: BuildServiceInput{
			Config: &config.Config{
				Platforms: config.PlatformsConfig{
					Shein: config.PlatformConfig{
						LoginService: config.LoginServiceConfig{
							BaseURL:    "http://login:8000",
							TenantID:   "tenant-1",
							Identifier: "store-1",
						},
						CookieRedis: config.RedisConfig{Host: "redis", Port: 6379},
					},
				},
			},
		},
		repositories: &builtRepositories{},
		submit:       submitModule{},
	})

	assertProbeMissing(t, cfg.Health.SheinIntegration, "shein.productAPIBuilder 未接入")
	assertProbeMissing(t, cfg.Health.SheinIntegration, "shein.imageAPIBuilder 未接入")
	assertProbeMissing(t, cfg.Health.SheinIntegration, "shein.categoryResolver 未接入")
}

type settingsHealthProductAPIBuilderStub struct{}

func (settingsHealthProductAPIBuilderStub) BuildProductAPI(context.Context, int64) (sheinproduct.ProductAPI, string) {
	return nil, ""
}

type settingsHealthImageAPIBuilderStub struct{}

func (settingsHealthImageAPIBuilderStub) BuildImageAPI(context.Context, int64) (sheinimage.ImageAPI, string) {
	return nil, ""
}

func assertProbeMissing(t *testing.T, probe listingkit.SettingsHealthProbe, want string) {
	t.Helper()
	for _, got := range probe.Missing {
		if got == want {
			return
		}
	}
	t.Fatalf("probe missing = %#v, want %q", probe.Missing, want)
}
