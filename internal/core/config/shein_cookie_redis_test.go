package config

import "testing"

func TestEffectiveSheinCookieRedisFallsBackToGlobalRedis(t *testing.T) {
	cfg := &Config{
		Redis: &RedisConfig{
			Host:     "redis.local",
			Port:     6379,
			Password: "secret",
			DB:       9,
			PoolSize: 12,
		},
	}

	got := cfg.EffectiveSheinCookieRedis()
	if got.Host != "redis.local" || got.Port != 6379 || got.Password != "secret" || got.DB != 9 || got.PoolSize != 12 {
		t.Fatalf("EffectiveSheinCookieRedis() = %+v", got)
	}
}

func TestEffectiveSheinCookieRedisPrefersExplicitSheinConfig(t *testing.T) {
	cfg := &Config{
		Redis: &RedisConfig{Host: "global.redis", Port: 6379, DB: 0},
		Platforms: PlatformsConfig{
			Shein: PlatformConfig{
				CookieRedis: RedisConfig{
					Host: "shein.redis",
					Port: 6380,
					DB:   3,
				},
			},
		},
	}

	got := cfg.EffectiveSheinCookieRedis()
	if got.Host != "shein.redis" || got.Port != 6380 || got.DB != 3 {
		t.Fatalf("EffectiveSheinCookieRedis() = %+v", got)
	}
}
