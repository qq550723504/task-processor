package amazon

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestEffectiveBrowserPoolSizeUsesConfiguredValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Browser.PoolSize = 5

	if got := effectiveBrowserPoolSize(cfg); got != 5 {
		t.Fatalf("expected configured pool size 5, got %d", got)
	}
}

func TestEffectiveBrowserPoolSizeFallsBackToAmazonDefault(t *testing.T) {
	if got := effectiveBrowserPoolSize(nil); got != defaultAmazonBrowserPoolSize {
		t.Fatalf("expected default pool size %d, got %d", defaultAmazonBrowserPoolSize, got)
	}

	cfg := &config.Config{}
	if got := effectiveBrowserPoolSize(cfg); got != defaultAmazonBrowserPoolSize {
		t.Fatalf("expected default pool size %d for zero config, got %d", defaultAmazonBrowserPoolSize, got)
	}
}
