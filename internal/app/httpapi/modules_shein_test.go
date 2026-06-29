package httpapi

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestBuildSheinLoginModuleSkipsModuleWithoutLocalStoreRepository(t *testing.T) {
	t.Parallel()

	result, closer, err := buildSheinLoginModuleResult(&runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg: &config.Config{
				Platforms: config.PlatformsConfig{
					Shein: config.PlatformConfig{
						CookieRedis: config.RedisConfig{Host: "127.0.0.1"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildSheinLoginModuleResult() error = %v", err)
	}
	if result != nil {
		t.Fatalf("buildSheinLoginModuleResult() result = %#v, want nil", result)
	}
	if closer != nil {
		t.Fatal("buildSheinLoginModuleResult() closer should be nil when module is skipped")
	}
}
