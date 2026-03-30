package config

import (
	"strings"
	"testing"
)

func TestLoadFromBytesLoadsAmazonProxyPool(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
management:
  clientSecret: test-secret
  scopes: [user.read]
openai:
  apiKey: test-key
  model: test-model
  baseURL: https://example.com/v1
  timeout: 30
amazon:
  enabled: true
  dataFreshnessDays: 7
  crawlTimeout: 120
  proxyPool:
    enabled: true
    strategy: round_robin
    failureCooldownSeconds: 300
    proxies:
      - http://proxy-a:8000
      - http://proxy-b:8000
`))
	if err != nil {
		t.Fatalf("expected config to load, got error: %v", err)
	}

	if !cfg.Amazon.ProxyPool.Enabled {
		t.Fatal("expected proxy pool to be enabled")
	}
	if cfg.Amazon.ProxyPool.Strategy != "round_robin" {
		t.Fatalf("expected strategy round_robin, got %s", cfg.Amazon.ProxyPool.Strategy)
	}
	if len(cfg.Amazon.ProxyPool.Proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(cfg.Amazon.ProxyPool.Proxies))
	}
}

func TestValidateAmazonConfigRejectsEnabledProxyPoolWithoutProxies(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Amazon.Enabled = true
	cfg.Amazon.ProxyPool.Enabled = true
	cfg.Amazon.ProxyPool.Strategy = "round_robin"
	cfg.Amazon.ProxyPool.FailureCooldownSeconds = 300
	cfg.Amazon.ProxyPool.Proxies = nil

	errs := ValidateAmazonConfig(&cfg.Amazon)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for empty proxy list")
	}

	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "amazon.proxyPool.proxies") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected proxy pool proxy-list validation error, got: %v", errs)
	}
}
