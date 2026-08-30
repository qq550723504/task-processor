package local

import "testing"

func TestRedisSheinCookieProviderCloseHandlesNilProvider(t *testing.T) {
	var provider *redisSheinCookieProvider
	if err := provider.Close(); err != nil {
		t.Fatalf("close nil cookie provider: %v", err)
	}
}
