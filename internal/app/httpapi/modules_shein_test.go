package httpapi

import (
	"testing"

	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
)

func TestBuildSheinLoginAccountProviderReturnsNilWithoutLocalStoreRepository(t *testing.T) {
	t.Parallel()

	provider, closer, err := buildSheinLoginAccountProvider(&runtimeDeps{
		cfg: &config.Config{},
	})
	if err != nil {
		t.Fatalf("buildSheinLoginAccountProvider() error = %v", err)
	}
	if provider != nil {
		t.Fatalf("buildSheinLoginAccountProvider() provider = %T, want nil", provider)
	}
	if closer != nil {
		t.Fatal("buildSheinLoginAccountProvider() closer should be nil when local repository is unavailable")
	}
}

func TestBuildSheinLoginModuleSkipsModuleWithoutLocalStoreRepository(t *testing.T) {
	t.Parallel()

	handler, closer, err := buildSheinLoginModule(&runtimeDeps{
		cfg: &config.Config{
			Platforms: config.PlatformsConfig{
				Shein: config.PlatformConfig{
					CookieRedis: config.RedisConfig{Host: "127.0.0.1"},
				},
			},
		},
		shared: &appbootstrap.SharedResources{
			ManagementClient: management.NewClientManager(&config.ManagementConfig{}),
		},
	})
	if err != nil {
		t.Fatalf("buildSheinLoginModule() error = %v", err)
	}
	if handler != nil {
		t.Fatalf("buildSheinLoginModule() handler = %T, want nil", handler)
	}
	if closer != nil {
		t.Fatal("buildSheinLoginModule() closer should be nil when module is skipped")
	}
}
